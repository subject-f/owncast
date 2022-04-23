package chat

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/owncast/owncast/core/chat/events"
	"github.com/owncast/owncast/core/data"
	"github.com/owncast/owncast/core/user"
	"github.com/owncast/owncast/core/webhooks"
	log "github.com/sirupsen/logrus"
)

func (s *Server) userNameChanged(eventData chatClientEvent) {
	var receivedEvent events.NameChangeEvent
	if err := json.Unmarshal(eventData.data, &receivedEvent); err != nil {
		log.Errorln("error unmarshalling to NameChangeEvent", err)
		return
	}

	proposedUsername := receivedEvent.NewName

	// Check if name is on the blocklist
	blocklist := data.GetForbiddenUsernameRegexList()

	for _, blockedRegex := range blocklist {
		if blockedRegex.MatchString(proposedUsername) {
			// Denied.
			log.Warnln(eventData.client.User.DisplayName, "blocked from changing name to", proposedUsername, "due to blocked name", blockedRegex.String())
			message := fmt.Sprintf("You cannot change your name to **%s**.", proposedUsername)
			s.sendActionToClient(eventData.client, message)

			// Resend the client's user so their username is in sync.
			eventData.client.sendConnectedClientInfo()

			return
		}
	}

	// Check if the name is not already assigned to a registered user.
	if available, err := user.IsDisplayNameAvailable(proposedUsername); err != nil {
		log.Errorln("error checking if name is available", err)
		return
	} else if !available {
		message := fmt.Sprintf("You cannot change your name to **%s**, it is already in use.", proposedUsername)
		s.sendActionToClient(eventData.client, message)

		// Resend the client's user so their username is in sync.
		eventData.client.sendConnectedClientInfo()

		return
	}

	savedUser := user.GetUserByToken(eventData.client.accessToken)
	oldName := savedUser.DisplayName

	// Save the new name
	if err := user.ChangeUsername(eventData.client.User.ID, receivedEvent.NewName); err != nil {
		log.Errorln("error changing username", err)
	}

	// Update the connected clients associated user with the new name
	now := time.Now()
	eventData.client.User = savedUser
	eventData.client.User.NameChangedAt = &now

	// Send chat event letting everyone about about the name change
	savedUser.DisplayName = receivedEvent.NewName

	broadcastEvent := events.NameChangeBroadcast{
		Oldname: oldName,
	}
	broadcastEvent.User = savedUser
	broadcastEvent.SetDefaults()
	payload := broadcastEvent.GetBroadcastPayload()
	if err := s.Broadcast(payload); err != nil {
		log.Errorln("error broadcasting NameChangeEvent", err)
		return
	}

	// Send chat user name changed webhook
	receivedEvent.User = savedUser
	receivedEvent.ClientID = eventData.client.id
	webhooks.SendChatEventUsernameChanged(receivedEvent)
}

func (s *Server) userMessageSent(eventData chatClientEvent) {
	var event events.UserMessageEvent
	if err := json.Unmarshal(eventData.data, &event); err != nil {
		log.Errorln("error unmarshalling to UserMessageEvent", err)
		return
	}

	event.SetDefaults()
	event.ClientID = eventData.client.id

	// Ignore empty messages
	if event.Empty() {
		return
	}

	event.User = user.GetUserByToken(eventData.client.accessToken)

	// Guard against nil users
	if event.User == nil {
		return
	}

	payload := event.GetBroadcastPayload()

	// Check against the word filter. We're currently using the forbidden users list as a stand-in.
	blocklist := data.GetForbiddenUsernameRegexList()

	for _, blockedRegex := range blocklist {
		if blockedRegex.MatchString(event.Body) && eventData.client != nil {
			log.Warnf("blocked a message that caught a word filter (filter: %v, contents: %v, displayname: %v, userid: %v)", blockedRegex.String(), event.Body, event.User.DisplayName, event.User.ID)
			// Silently drop the message and don't save it, but send it back to the user.
			s.Send(payload, eventData.client)
			return
		}
	}

	if err := s.Broadcast(payload); err != nil {
		log.Errorln("error broadcasting UserMessageEvent payload", err)
		return
	}

	// Send chat message back to Discord
	_discordChannel <- event

	// Send chat message sent webhook
	webhooks.SendChatEvent(&event)
	chatMessagesSentCounter.Inc()

	SaveUserMessage(event)
	eventData.client.MessageCount++
	_lastSeenCache[event.User.ID] = time.Now()
}

func (s *Server) discordMessageSent(eventData chatClientEvent) {
	var event events.UserMessageEvent
	if err := json.Unmarshal(eventData.data, &event); err != nil {
		log.Errorln("error unmarshalling to UserMessageEvent", err)
		return
	}

	// Second layer of the message event
	var discordMessage DiscordMessage
	if err := json.Unmarshal(eventData.data, &discordMessage); err != nil {
		log.Errorln("error unmarshalling to DiscordMessage", err)
		return
	}

	event.SetDefaults()
	event.ClientID = 0

	// Ignore empty messages
	if event.Empty() {
		return
	}

	event.User = &user.User{
		DisplayName:  discordMessage.Username + "#" + discordMessage.Discriminator,
		DisplayColor: discordMessage.Color,
	}

	payload := event.GetBroadcastPayload()
	if err := s.Broadcast(payload); err != nil {
		log.Errorln("error broadcasting UserMessageEvent payload", err)
		return
	}

	chatMessagesSentCounter.Inc()
}
