/*
The Owncast Latency Compensator.

It will try to slowly adjust the playback rate to enable the player to get
further into the future, with the goal of being as close to the live edge as
possible, without causing any buffering events.

How does latency occur?
Two pieces are at play. The first being the server. The larger each segment is
that is being generated by Owncast, the larger gap you are going to be from
live when you begin playback.

Second is your media player.
The player tries to play every segment as it comes in.
However, your computer is not always 100% in playing things in real time, and
there are natural stutters in playback. So if one frame is delayed in playback
you may not see it visually, but now you're one frame behind. Eventually this
can compound and you can be many seconds behind.

How to help with this? The Owncast Latency Compensator will:
  - Determine the start (max) and end (min) latency values.
  - Keep an eye on download speed and stop compensating if it drops too low.
  - Limit the playback speedup rate so it doesn't sound weird by jumping speeds.
  - Force a large jump to into the future once compensation begins.
  - Dynamically calculate the speedup rate based on network speed.
  - Pause the compensation if buffering events occur.
  - Completely give up on all compensation if too many buffering events occur.
*/

const REBUFFER_EVENT_LIMIT = 5; // Max number of buffering events before we stop compensating for latency.
const MIN_BUFFER_DURATION = 200; // Min duration a buffer event must last to be counted.
const MAX_SPEEDUP_RATE = 1.08; // The playback rate when compensating for latency.
const MAX_SPEEDUP_RAMP = 0.02; // The max amount we will increase the playback rate at once.
const TIMEOUT_DURATION = 30 * 1000; // The amount of time we stop handling latency after certain events.
const CHECK_TIMER_INTERVAL = 3 * 1000; // How often we check if we should be compensating for latency.
const BUFFERING_AMNESTY_DURATION = 3 * 1000 * 60; // How often until a buffering event expires.
const REQUIRED_BANDWIDTH_RATIO = 2.0; // The player:bitrate ratio required to enable compensating for latency.
const HIGHEST_LATENCY_SEGMENT_LENGTH_MULTIPLIER = 2.6; // Segment length * this value is when we start compensating.
const LOWEST_LATENCY_SEGMENT_LENGTH_MULTIPLIER = 1.8; // Segment length * this value is when we stop compensating.
const MIN_LATENCY = 4 * 1000; // The absolute lowest we'll continue compensation to be running at.
const MAX_LATENCY = 15 * 1000; // The absolute highest we'll allow a target latency to be before we start compensating.
const MAX_JUMP_LATENCY = 7 * 1000; // How much behind the max latency we need to be behind before we allow a jump.
const MAX_JUMP_FREQUENCY = 20 * 1000; // How often we'll allow a time jump.
const STARTUP_WAIT_TIME = 10 * 1000; // The amount of time after we start up that we'll allow monitoring to occur.

class LatencyCompensator {
  constructor(player) {
    this.player = player;
    this.enabled = false;
    this.running = false;
    this.inTimeout = false;
    this.jumpingToLiveIgnoreBuffer = false;
    this.performedInitialLiveJump = false;
    this.timeoutTimer = 0;
    this.checkTimer = 0;
    this.bufferingCounter = 0;
    this.bufferingTimer = 0;
    this.playbackRate = 1.0;
    this.lastJumpOccurred = null;
    this.startupTime = new Date();
    this.player.on('playing', this.handlePlaying.bind(this));
    this.player.on('error', this.handleError.bind(this));
    this.player.on('waiting', this.handleBuffering.bind(this));
    this.player.on('ended', this.handleEnded.bind(this));
    this.player.on('canplaythrough', this.handlePlaying.bind(this));
    this.player.on('canplay', this.handlePlaying.bind(this));
  }

  // This is run on a timer to check if we should be compensating for latency.
  check() {
    if (new Date().getTime() - this.startupTime.getTime() < STARTUP_WAIT_TIME) {
      return;
    }

    // If we're paused then do nothing.
    if (this.player.paused()) {
      return;
    }

    if (this.player.seeking()) {
      return;
    }

    if (this.inTimeout) {
      console.log('in timeout...');
      return;
    }

    if (!this.enabled) {
      console.log('not enabled...');
      return;
    }

    const tech = this.player.tech({ IWillNotUseThisInPlugins: true });

    if (!tech || !tech.vhs) {
      return;
    }

    try {
      // Check the player buffers to make sure there's enough playable content
      // that we can safely play.
      if (tech.vhs.stats.buffered.length === 0) {
        console.log('timeout due to zero buffers');
        this.timeout();
      }

      let totalBuffered = 0;

      tech.vhs.stats.buffered.forEach((buffer) => {
        totalBuffered += buffer.end - buffer.start;
      });
      console.log('buffered', totalBuffered);

      if (totalBuffered < 18) {
        this.timeout();
      }
    } catch (e) {}

    // Determine how much of the current playlist's bandwidth requirements
    // we're utilizing. If it's too high then we can't afford to push
    // further into the future because we're downloading too slowly.
    const currentPlaylist = tech.vhs.playlists.media();
    const currentPlaylistBandwidth = currentPlaylist.attributes.BANDWIDTH;
    const playerBandwidth = tech.vhs.systemBandwidth;
    const bandwidthRatio = playerBandwidth / currentPlaylistBandwidth;

    // If we don't think we have the bandwidth to play faster, then don't do it.
    if (bandwidthRatio < REQUIRED_BANDWIDTH_RATIO) {
      this.timeout();
      return;
    }

    try {
      const segment = getCurrentlyPlayingSegment(tech);
      if (!segment) {
        return;
      }

      // How far away from live edge do we start the compensator.
      const maxLatencyThreshold = Math.min(
        MAX_LATENCY,
        segment.duration * 1000 * HIGHEST_LATENCY_SEGMENT_LENGTH_MULTIPLIER
      );

      // How far away from live edge do we stop the compensator.
      const minLatencyThreshold = Math.max(
        MIN_LATENCY,
        segment.duration * 1000 * LOWEST_LATENCY_SEGMENT_LENGTH_MULTIPLIER
      );

      const segmentTime = segment.dateTimeObject.getTime();
      const now = new Date().getTime();
      const latency = now - segmentTime;

      // Using our bandwidth ratio determine a wide guess at how fast we can play.
      var proposedPlaybackRate = bandwidthRatio * 0.33;

      // But limit the playback rate to a max value.
      proposedPlaybackRate = Math.max(
        Math.min(proposedPlaybackRate, MAX_SPEEDUP_RATE),
        1.0
      );

      if (proposedPlaybackRate > this.playbackRate + MAX_SPEEDUP_RAMP) {
        // If this proposed speed is substantially faster than the current rate,
        // then allow us to ramp up by using a slower value for now.
        proposedPlaybackRate = this.playbackRate + MAX_SPEEDUP_RAMP;
      }

      // Limit to 3 decimal places of precision.
      proposedPlaybackRate =
        Math.round(proposedPlaybackRate * Math.pow(10, 3)) / Math.pow(10, 3);

      if (latency > maxLatencyThreshold) {
        // If the current latency exceeds the max jump amount then
        // force jump into the future, skipping all the video in between.
        if (
          this.shouldJumpToLive() &&
          latency > maxLatencyThreshold + MAX_JUMP_LATENCY
        ) {
          const jumpAmount = latency / 1000 - segment.duration * 3;
          console.log('jump amount', jumpAmount);
          const seekPosition = this.player.currentTime() + jumpAmount;
          console.log(
            'latency',
            latency / 1000,
            'jumping to live from ',
            this.player.currentTime(),
            ' to ',
            seekPosition
          );
          this.jump(seekPosition);
        }

        // Otherwise start the playback rate adjustment.
        this.start(proposedPlaybackRate);
      } else if (latency <= minLatencyThreshold) {
        this.stop();
      }

      console.log(
        'latency',
        latency / 1000,
        'min',
        minLatencyThreshold / 1000,
        'max',
        maxLatencyThreshold / 1000,
        'playback rate',
        this.playbackRate,
        'enabled:',
        this.enabled,
        'running: ',
        this.running,
        'timeout: ',
        this.inTimeout,
        'buffers: ',
        this.bufferingCounter
      );
    } catch (err) {
      console.error(err);
    }
  }

  shouldJumpToLive() {
    const now = new Date().getTime();
    const delta = now - this.lastJumpOccurred;
    return delta > MAX_JUMP_FREQUENCY;
  }

  jump(seekPosition) {
    this.jumpingToLiveIgnoreBuffer = true;
    this.performedInitialLiveJump = true;

    this.lastJumpOccurred = new Date();

    console.log(
      'current time',
      this.player.currentTime(),
      'seeking to',
      seekPosition
    );
    this.player.currentTime(seekPosition);

    setTimeout(() => {
      this.jumpingToLiveIgnoreBuffer = false;
    }, 5000);
  }

  setPlaybackRate(rate) {
    this.playbackRate = rate;
    this.player.playbackRate(rate);
  }

  start(rate = 1.0) {
    if (this.inTimeout || !this.enabled || rate === this.playbackRate) {
      return;
    }

    this.running = true;
    this.setPlaybackRate(rate);
  }

  stop() {
    if (this.running) {
      console.log('stopping latency compensator...');
    }
    this.running = false;
    this.setPlaybackRate(1.0);
  }

  enable() {
    this.enabled = true;
    clearInterval(this.checkTimer);
    clearTimeout(this.bufferingTimer);

    this.checkTimer = setInterval(() => {
      this.check();
    }, CHECK_TIMER_INTERVAL);
  }

  // Disable means we're done for good and should no longer compensate for latency.
  disable() {
    clearInterval(this.checkTimer);
    clearTimeout(this.timeoutTimer);
    this.stop();
    this.enabled = false;
  }

  timeout() {
    if (this.inTimeout) {
      return;
    }

    if (!this.performedInitialLiveJump) {
      return;
    }

    if (this.jumpingToLiveIgnoreBuffer) {
      return;
    }

    this.inTimeout = true;
    this.stop();

    clearTimeout(this.timeoutTimer);
    this.timeoutTimer = setTimeout(() => {
      this.endTimeout();
    }, TIMEOUT_DURATION);
  }

  endTimeout() {
    clearTimeout(this.timeoutTimer);
    this.inTimeout = false;
  }

  handlePlaying() {
    clearTimeout(this.bufferingTimer);

    if (!this.enabled) {
      return;
    }
  }

  handleEnded() {
    if (!this.enabled) {
      return;
    }

    this.disable();
  }

  handleError(e) {
    if (!this.enabled) {
      return;
    }

    console.log('handle error', e);
    this.timeout();
  }

  countBufferingEvent() {
    this.bufferingCounter++;
    if (this.bufferingCounter > REBUFFER_EVENT_LIMIT) {
      this.disable();
      return;
    }
    console.log('timeout due to buffering');
    this.timeout();

    // Allow us to forget about old buffering events if enough time goes by.
    setTimeout(() => {
      if (this.bufferingCounter > 0) {
        this.bufferingCounter--;
      }
    }, BUFFERING_AMNESTY_DURATION);
  }

  handleBuffering() {
    if (!this.enabled) {
      return;
    }

    if (!this.performedInitialLiveJump) {
      return;
    }

    if (this.jumpingToLiveIgnoreBuffer) {
      this.jumpingToLiveIgnoreBuffer = false;
      return;
    }

    this.bufferingTimer = setTimeout(() => {
      this.countBufferingEvent();
    }, MIN_BUFFER_DURATION);
  }
}

function getCurrentlyPlayingSegment(tech) {
  var target_media = tech.vhs.playlists.media();
  var snapshot_time = tech.currentTime();

  var segment;

  // Itinerate trough available segments and get first within which snapshot_time is
  for (var i = 0, l = target_media.segments.length; i < l; i++) {
    // Note: segment.end may be undefined or is not properly set
    if (snapshot_time < target_media.segments[i].end) {
      segment = target_media.segments[i];
      break;
    }
  }

  if (!segment) {
    segment = target_media.segments[0];
  }

  return segment;
}

export default LatencyCompensator;
