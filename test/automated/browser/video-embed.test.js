const listenForErrors = require('./lib/errors.js').listenForErrors;
const videoTest = require('./tests/video.js').videoTest;

describe('Video embed page', () => {
  beforeAll(async () => {
    await page.setViewport({ width: 1080, height: 720 });
    listenForErrors(browser, page);
    await page.goto('http://localhost:5309/embed/video');
  });

  afterAll(async () => {
    // await page.waitForTimeout(5000);
    await page.screenshot({
      path: 'screenshots/screenshot_video_embed.png',
      fullPage: true,
    });
  });

  it('should have the video container element', async () => {
    await page.waitForSelector('#video-container');
  });

  it('should have the stream info status bar', async () => {
    await page.waitForSelector('#stream-info');
  });
  // videoTest(browser, page);
});
