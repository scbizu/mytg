package mytg

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"

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

func (b *Bot) RegisterWebhook(
	resHandler func(updateMsg tgbotapi.Update) ([]interface{}, error),
	OnChosenHandler func(*tgbotapi.ChosenInlineResult, *tgbotapi.BotAPI) error,
	plugins ...plugin.MessagePlugin,
) {

	b.setWebhookOnce()
	router := fmt.Sprintf("/tg/%s", token)
	http.HandleFunc(router, func(w http.ResponseWriter, r *http.Request) {

		bytes, _ := ioutil.ReadAll(r.Body)
		logrus.Debugf("mytg: raw handler message: %s", string(bytes))

		var update tgbotapi.Update
		json.Unmarshal(bytes, &update)

		if err := b.serveBotUpdateMessage(update, plugins...); err != nil {
			logrus.Errorf("mytg: %q", err)
		}
		if err := b.serveInlineMode(update, resHandler, OnChosenHandler); err != nil {
			logrus.Errorf("mytg: %q", err)
		}

	})

	listenWebhook()
}

var webhookOnce sync.Once

func (b *Bot) setWebhookOnce() {
	webhookOnce.Do(func() {
		if _, err := b.bot.RemoveWebhook(); err != nil {
			logrus.Warnf("mytg: serve request: %q", err)
		}
		cert := NewDomainCert(tgAPIDomain)
		domainWithToken := fmt.Sprintf("%s/%s", cert.GetDomain(), token)
		if _, err := b.bot.SetWebhook(tgbotapi.NewWebhook(domainWithToken)); err != nil {
			logrus.Errorf("mytg: notify webhook failed:%q", err)
			return
		}
		if b.isDebugMode {
			logrus.SetLevel(logrus.DebugLevel)
			info, err := b.bot.GetWebhookInfo()
			if err != nil {
				logrus.Errorf("mytg: debug: get webhook info:%q", err)
				return
			}
			logrus.Debug(info.LastErrorMessage, info.LastErrorDate)
		}
	})
}

func (b *Bot) serveInlineMode(msg tgbotapi.Update,
	resHandler func(updateMsg tgbotapi.Update) ([]interface{}, error),
	OnChosenHandler func(*tgbotapi.ChosenInlineResult, *tgbotapi.BotAPI) error) error {

	if msg.ChosenInlineResult != nil &&
		OnChosenHandler != nil {
		logrus.Debugf("mytg: chosen result: %s", msg.ChosenInlineResult.ResultID)
		if err := OnChosenHandler(msg.ChosenInlineResult, b.bot); err != nil {
			logrus.Errorf("inline mode: %q", err)
			return err
		}
		msg.ChosenInlineResult = nil
		return nil
	}

	if msg.InlineQuery == nil {
		return nil
	}

	r, err := resHandler(msg)
	if err != nil {
		return err
	}
	config := tgbotapi.InlineConfig{
		InlineQueryID: msg.InlineQuery.ID,
		Results:       r,
		IsPersonal:    true,
	}
	if _, err := b.bot.AnswerInlineQuery(config); err != nil {
		return err
	}
	return nil
}

func (b *Bot) serveBotUpdateMessage(update tgbotapi.Update, plugins ...plugin.MessagePlugin) error {
	logrus.Debugf("[raw msg]:%#v\n", update)

	if update.Message == nil {
		return nil
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
		return err
	}
	return nil
}
