package chat

import (
	"encoding/json"
	"os"
	"strconv"

	"github.com/bwmarrin/discordgo"
	"github.com/owncast/owncast/core/chat/events"
	log "github.com/sirupsen/logrus"
)

var (
	channelId string
)

const (
	DISCORD_TOKEN_ENV_VAR = "DISCORD_TOKEN"
)

type DiscordMessage struct {
	T             string `json:"type"`
	Body          string `json:"body"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Color         int    `json:"color"`
}

func init() {
	discordClient, _ := discordgo.New("Bot " + os.Getenv(DISCORD_TOKEN_ENV_VAR))

	discordClient.Identify.Intents = discordgo.IntentsGuildMessages

	log.Infoln("Attempting to start Discord listener.")

	if err := discordClient.Open(); err != nil {
		log.Warnln("Failed to connect to Discord, perhaps the token is incorrect.")
		return
	}

	log.Infoln("Successfully connected to Discord gateway.")

	discordClient.AddHandler(messageCreate)
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if m.Content == "!bind" {
		s.ChannelMessageSend(m.ChannelID, "Current channel has been bound to Owncast chat.")
		channelId = m.ChannelID
		return
	}

	if m.ChannelID == channelId {
		color, err := strconv.Atoi(m.Author.Discriminator)
		if err != nil {
			color = 0
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
