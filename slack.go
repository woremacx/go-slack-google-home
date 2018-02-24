package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/ikasamah/homecast"
	"github.com/nlopes/slack"
	"golang.org/x/sync/errgroup"
)

type SlackBot struct {
	client        *slack.Client
	lang          string
	connectedUser *slack.UserDetails
	devices       []*homecast.CastDevice
}

func NewSlackBot(client *slack.Client, lang string) *SlackBot {
	return &SlackBot{
		client: client,
		lang:   lang,
	}
}

func (s *SlackBot) Run(ctx context.Context) {
	rtm := s.client.NewRTM()

	go rtm.ManageConnection()

	// Handle slack events
	for msg := range rtm.IncomingEvents {
		switch ev := msg.Data.(type) {
		case *slack.ConnectedEvent:
			s.connectedUser = ev.Info.User
			s.devices = homecast.LookupAndConnect(ctx)
			log.Printf("[INFO] Connected: user_id=%s", s.connectedUser.ID)
		case *slack.MessageEvent:
			if err := s.handleMessageEvent(ctx, ev); err != nil {
				if err := s.addReaction(ev, "no_entry_sign"); err != nil {
					log.Printf("[ERROR] Failed to add error reaction: %s", err)
				}
				log.Printf("[ERROR] Failed to handle message: %s", err)
			}
		case *slack.InvalidAuthEvent:
			log.Print("[ERROR] Failed to auth")
			return
		}
	}
}

func (s *SlackBot) handleMessageEvent(ctx context.Context, ev *slack.MessageEvent) error {
	log.Printf("[INFO] message %s", ev.Msg.Text)

	var mentioned bool
	var body string
	mentionFull := fmt.Sprintf("<@%s> ", s.connectedUser.ID)
	if strings.HasPrefix(ev.Msg.Text, mentionFull) {
		body = strings.TrimPrefix(ev.Msg.Text, mentionFull)
		mentioned = true
	}
	if !mentioned {
		mentionNotice := fmt.Sprintf("<@%s|home> ", s.connectedUser.ID)
		idx := strings.Index(ev.Msg.Text, mentionNotice)
		if idx > -1 {
			start := idx + len(mentionNotice)
			body = ev.Msg.Text[start:]
			mentioned = true
		}
	}
	if !mentioned {
		return nil
	}
	log.Printf("[INFO] speak: %s", body)

	if err := s.speak(ctx, body); err != nil {
		// Reload device, because address may have changed according to DHCP.
		log.Printf("[WARN] An error occurred in speak. Attempt to reload devices just now. err: %s", err)
		if err := s.addReaction(ev, "warning"); err != nil {
			return err
		}
		s.devices = homecast.LookupAndConnect(ctx)
		if err := s.speak(ctx, body); err != nil {
			return err
		}
	}
	return s.addReaction(ev, "sound")
}

func (s *SlackBot) speak(ctx context.Context, body string) error {
	var eg errgroup.Group
	for i := range s.devices {
		device := s.devices[i]
		eg.Go(func() error {
			log.Printf("[INFO] Attempting to make device speak: [%s]%s", device.AddrV4, device.Name)
			return device.Speak(ctx, body, s.lang)
		})
	}
	return eg.Wait()
}

func (s *SlackBot) addReaction(ev *slack.MessageEvent, emojiName string) error {
	msgRef := slack.NewRefToMessage(ev.Channel, ev.Timestamp)
	return s.client.AddReaction(emojiName, msgRef)
}
