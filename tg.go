package mytg

import (
	"errors"
	"fmt"
	"net/http"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/scbizu/mytg/plugin"
	"github.com/sirupsen/logrus"
)

type Bot struct {
	bot         *tgbotapi.BotAPI
	isDebugMode bool
}

func NewBot(isDebugMode bool) (*Bot, error) {
	bot := new(Bot)
	tgConn, err := ConnectTG()
	if err != nil {
		return nil, err
	}
	bot.bot = tgConn
	if isDebugMode {
		logrus.SetLevel(logrus.DebugLevel)
		bot.isDebugMode = true
		tgConn.Debug = true
		logrus.Infof("bot auth passed as %s", tgConn.Self.UserName)
	}
	return bot, nil
}

func listenWebhook() {
	port := fmt.Sprintf(":%s", listenPort)
	if err := http.ListenAndServe(port, nil); err != nil {
		panic(err)
	}
}

func (b *Bot) RegisterWebhook() {
	go listenWebhook()
}

func (b *Bot) ServeInlineMode(
	resHandler func(updateMsg tgbotapi.Update) ([]interface{}, error),
	OnChosenHandler func(*tgbotapi.ChosenInlineResult) error) error {
	msgs, err := b.getUpdateMessage()
	if err != nil {
		return err
	}
	for msg := range msgs {
		if msg.InlineQuery == nil {
			continue
		}
		if msg.ChosenInlineResult != nil &&
			OnChosenHandler != nil {
			if err := OnChosenHandler(msg.ChosenInlineResult); err != nil {
				logrus.Errorf("inline mode: %w", err)
				continue
			}
			return nil
		}
		r, err := resHandler(msg)
		if err != nil {
			continue
		}
		config := tgbotapi.InlineConfig{
			InlineQueryID: msg.InlineQuery.ID,
			Results:       r,
			IsPersonal:    true,
		}
		if _, err := b.bot.AnswerInlineQuery(config); err != nil {
			return err
		}
	}
	return nil
}

func (b *Bot) getUpdateMessage() (tgbotapi.UpdatesChannel, error) {
	if _, err := b.bot.RemoveWebhook(); err != nil {
		logrus.Warnf("mytg: serve request: %q", err)
	}
	cert := NewDomainCert(tgAPIDomain)
	domainWithToken := fmt.Sprintf("%s/%s", cert.GetDomain(), token)
	if _, err := b.bot.SetWebhook(tgbotapi.NewWebhook(domainWithToken)); err != nil {
		logrus.Errorf("notify webhook failed:%s", err.Error())
		return nil, err
	}
	if b.isDebugMode {
		logrus.SetLevel(logrus.DebugLevel)
		info, err := b.bot.GetWebhookInfo()
		if err != nil {
			return nil, err
		}
		logrus.Debug(info.LastErrorMessage, info.LastErrorDate)
	}

	pattern := fmt.Sprintf("/tg/%s", token)
	updatesMsgChannel := b.bot.ListenForWebhook(pattern)
	return updatesMsgChannel, nil
}

func (b *Bot) ServeBotUpdateMessage(plugins ...plugin.MessagePlugin) error {
	updatesMsgChannel, err := b.getUpdateMessage()
	if err != nil {
		return err
	}

	logrus.Debugf("msg in channel:%d", len(updatesMsgChannel))
	for update := range updatesMsgChannel {
		logrus.Debugf("[raw msg]:%#v\n", update)

		if update.Message == nil {
			continue
		}

		var config tgbotapi.Chattable

		for _, p := range plugins {
			var err error
			config, err = p.HandleMessage(
				update.Message,
			)

			if errors.Is(err, plugin.ErrMessageNotMatched) {
				continue
			}

			if err != nil {
				logrus.Errorf("plugin: %q", err)
				continue
			}
		}

		if _, err := b.bot.Send(config); err != nil {
			logrus.Errorf("mytg: send message: %q", err)
			continue
		}
	}
	return nil
}
