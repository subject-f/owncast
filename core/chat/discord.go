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
	DISCORD_BIND_COMMAND   = "!bind"
	DISCORD_UNBIND_COMMAND = "!unbind"
)

var (
	channelId       string
	_discordClient  *discordgo.Session
	_discordChannel = make(chan events.UserMessageEvent, CHANNEL_BUFFER_SIZE)
	re              = regexp.MustCompile(`(?:<img.*?alt=")(.*?)(?:".*?>)`)
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

	if m.Content == DISCORD_BIND_COMMAND || m.Content == DISCORD_UNBIND_COMMAND {
		perms, err := s.UserChannelPermissions(m.Author.ID, m.ChannelID)

		if err != nil {
			log.Warnln("Failed to check permissions for the user.", err)
			return
		}

		if perms&discordgo.PermissionManageMessages != discordgo.PermissionManageMessages {
			s.ChannelMessageSend(m.ChannelID, "You don't have permissions to do that.")
			return
		}

		if m.Content == DISCORD_BIND_COMMAND {
			s.ChannelMessageSend(m.ChannelID, "Current channel has been bound to Owncast chat.")
			channelId = m.ChannelID
			return
		}

		if m.Content == DISCORD_UNBIND_COMMAND {
			s.ChannelMessageSend(m.ChannelID, "Owncast unbound.")
			channelId = "0"
			return
		}
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

func parseString(input string) string {
	return re.ReplaceAllString(input, `$1`)
}
