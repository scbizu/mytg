package plugin

import (
	"errors"
	api "github.com/go-telegram-bot-api/telegram-bot-api"
)

var (
	ErrMessageNotMatched = errors.New("plugin: message not matched")
)

type MessagePlugin interface {
	HandleMessage(msg *api.Message) (api.Chattable, error)
}
