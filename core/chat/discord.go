package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/owncast/owncast/core/chat/events"
	log "github.com/sirupsen/logrus"
)

const (
	DISCORD_TOKEN_ENV_VAR = "DISCORD_TOKEN"
	INTERVAL_TIMER_MS     = 100
	FLUSH_INTERVAL_MS     = 250 // Just under 5 messages/s
	DEBOUNCE_BUFFER_SIZE  = 30
	CHANNEL_BUFFER_SIZE   = 500
	MAX_CHAR_COUNT        = 2000

	DEFAULT_COLOR          = 0
	DEFAULT_MESSAGE_LENGTH = 64
)

var (
	_discordClient  *discordgo.Session
	_discordChannel = make(chan events.UserMessageEvent, CHANNEL_BUFFER_SIZE)

	channelId        string
	maxMessageLength int = DEFAULT_MESSAGE_LENGTH

	emojiRegex      = regexp.MustCompile(`(?:<img.*?alt=")(.*?)(?:".*?>)`)
	discordCommands = map[string]func(*discordgo.Session, *discordgo.MessageCreate, ...string){
		"!bind":    bindChannelCommand,
		"!unbind":  unbindChannelCommand,
		"!mlength": mlengthCommand,
	}
)

type DiscordMessage struct {
	T             string `json:"type"`
	Body          string `json:"body"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Color         int    `json:"color"`
}

func init() {
	_discordClient, _ = discordgo.New("Bot " + os.Getenv(DISCORD_TOKEN_ENV_VAR))

	_discordClient.Identify.Intents = discordgo.IntentsAll

	log.Infoln("Attempting to start Discord listener.")

	// Start the listener to clear the buffer even if the client fails
	go messageSend()

	if err := _discordClient.Open(); err != nil {
		log.Warnln("Failed to connect to Discord, perhaps the token is incorrect.")
		return
	}

	log.Infoln("Successfully connected to Discord gateway.")

	_discordClient.AddHandler(messageReceive)
}

func messageReceive(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Parse all the commands.
	fields := strings.Fields(m.Content)

	// Guard against empty inputs, though it shouldn't happen.
	if len(fields) == 0 {
		fields = make([]string, 1)
	}

	if command, ok := discordCommands[fields[0]]; ok {
		perms, err := s.UserChannelPermissions(m.Author.ID, m.ChannelID)

		if err != nil {
			log.Warnln("Failed to check permissions for the user.", err)
			return
		}

		log.Debugf("User %v (ID: %v) had perms integer of %v, need %v mask.", m.Author.Username, m.Author.ID, perms, discordgo.PermissionManageMessages)

		if perms&discordgo.PermissionManageMessages != discordgo.PermissionManageMessages {
			s.ChannelMessageSend(m.ChannelID, "You don't have permissions to do that.")
			return
		}

		command(s, m, fields[1:]...)
		return
	}

	if m.ChannelID == channelId {
		color, err := strconv.Atoi(m.Author.Discriminator)
		if err != nil {
			color = DEFAULT_COLOR
		}
		byteData, _ := json.Marshal(&DiscordMessage{
			T:             events.DiscordMessageSent,
			Body:          m.Content,
			Username:      m.Author.Username,
			Discriminator: m.Author.Discriminator,
			Color:         color,
		})

		if _server != nil {
			_server.inbound <- chatClientEvent{data: byteData, client: nil}
		}
	}
}

func bindChannelCommand(s *discordgo.Session, m *discordgo.MessageCreate, args ...string) {
	channelId = m.ChannelID
	s.ChannelMessageSend(m.ChannelID, "Current channel has been bound to Owncast chat.")
}

func unbindChannelCommand(s *discordgo.Session, m *discordgo.MessageCreate, args ...string) {
	channelId = "0"
	s.ChannelMessageSend(m.ChannelID, "Owncast unbound.")
}

func mlengthCommand(s *discordgo.Session, m *discordgo.MessageCreate, args ...string) {
	if len(args) == 0 {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Current max message length is: `%v`", maxMessageLength))
		return
	}

	length, err := strconv.Atoi(args[0])
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "Unable to parse message length. It must be an integer.")
		return
	}

	maxMessageLength = length
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Successfully set message length to `%v`.", maxMessageLength))
}

func messageSend() {
	ticker := time.NewTicker(INTERVAL_TIMER_MS * time.Millisecond)
	lastMessageTime := time.Now()
	// Book keeping to make sure we don't send payloads that are too large
	charCount := 0

	var buffer []string

	// Generally, we should hope that this blocks but for a short time only (ie. no rate limits)
	flushBuffer := func() {
		_, err := _discordClient.State.Channel(channelId)

		if err == nil && len(buffer) > 0 {
			_, err := _discordClient.ChannelMessageSend(channelId, strings.Join(buffer, "\n"))
			if err != nil {
				log.Warnln("Failed to relay messages to discord", err)
			}
		}

		buffer = nil
		charCount = 0
	}

	for {
		select {
		case message := <-_discordChannel:
			lastMessageTime = time.Now()
			messageString := parseString(fmt.Sprintf("**%v:** %v", message.User.DisplayName, message.Body))

			if charCount+len(messageString) > MAX_CHAR_COUNT {
				flushBuffer()
			}
			buffer = append(buffer, messageString)
			charCount += len(messageString)

			if len(buffer) > DEBOUNCE_BUFFER_SIZE {
				flushBuffer()
			}
		case <-ticker.C:
			if time.Since(lastMessageTime) > FLUSH_INTERVAL_MS*time.Millisecond {
				flushBuffer()
			}
		}

	}
}

// IsValidMessageLength returns the valid message length after parsing for emojis.
func IsValidMessageLength(input string) bool {
	parsedMessage := parseString(input)
	return len(parsedMessage) <= maxMessageLength
}

func parseString(input string) string {
	return emojiRegex.ReplaceAllString(input, `$1`)
}
