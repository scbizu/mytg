package mytg

import (
	"errors"
	"fmt"
	"github.com/scbizu/mytg/plugin"
	"net/http"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
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

func (b *Bot) ServeBotUpdateMessage(plugins ...plugin.MessagePlugin) error {
	go listenWebhook()
	if _, err := b.bot.RemoveWebhook(); err != nil {
		logrus.Warnf("mytg: serve request: %q", err)
	}
	cert := NewDomainCert(tgAPIDomain)
	domainWithToken := fmt.Sprintf("%s/%s", cert.GetDomain(), token)
	if _, err := b.bot.SetWebhook(tgbotapi.NewWebhook(domainWithToken)); err != nil {
		logrus.Errorf("notify webhook failed:%s", err.Error())
		return err
	}
	if b.isDebugMode {
		logrus.SetLevel(logrus.DebugLevel)
		info, err := b.bot.GetWebhookInfo()
		if err != nil {
			return err
		}
		logrus.Debug(info.LastErrorMessage, info.LastErrorDate)
	}
	pattern := fmt.Sprintf("/tg/%s", token)
	updatesMsgChannel := b.bot.ListenForWebhook(pattern)

	logrus.Debugf("msg in channel:%d", len(updatesMsgChannel))
	for update := range updatesMsgChannel {
		logrus.Debugf("[raw msg]:%#v\n", update)

		if update.Message == nil {
			continue
		}

		var config tgbotapi.MessageConfig

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

		config.ReplyToMessageID = update.Message.MessageID
		if _, err := b.bot.Send(config); err != nil {
			logrus.Errorf("mytg: send message: %q", err)
			continue
		}
	}
	return nil
}
