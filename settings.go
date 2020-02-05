package mytg

import (
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

const (
	tokenKey    = "BOTKEY"
	listenPort  = "LISTENPORT"
	tgAPIDomain = "https://api.scnace.me/tg"
)

var (
	token string
)

func init() {
	token = os.Getenv(tokenKey)
}

// ConnectTG returns the bot instance
func ConnectTG() (bot *tgbotapi.BotAPI, err error) {
	return tgbotapi.NewBotAPI(token)
}
