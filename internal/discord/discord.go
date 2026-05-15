package discord

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

const webhookName = "madibridge"

type Client struct {
	Session  *discordgo.Session
	Webhooks map[string]*discordgo.Webhook
}

func New(token string) (*Client, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	dg.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsMessageContent |
		discordgo.IntentsGuildMessageReactions

	return &Client{
		Session:  dg,
		Webhooks: make(map[string]*discordgo.Webhook),
	}, nil
}

func (c *Client) Open(channelIDs []string) error {
	if err := c.Session.Open(); err != nil {
		return err
	}

	for _, channelID := range channelIDs {
		webhook, err := c.ensureWebhook(channelID)
		if err != nil {
			return err
		}

		c.Webhooks[channelID] = webhook
	}
	log.Println("discord connected and webhook ready")
	return nil
}

func (c *Client) ensureWebhook(channelID string) (*discordgo.Webhook, error) {
	webhooks, err := c.Session.ChannelWebhooks(channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to list webhooks: %w", err)
	}

	for _, wh := range webhooks {
		if wh.Name == webhookName {
			return wh, nil
		}
	}

	webhook, err := c.Session.WebhookCreate(channelID, webhookName, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook: %w", err)
	}

	return webhook, nil
}

func (c *Client) SendMessage(channelID, displayName, avatarURL, content, replyTo string) (*discordgo.Message, error) {
	webhook, ok := c.Webhooks[channelID]
	if !ok {
		return nil, fmt.Errorf("webhook not found %s", channelID)
	}

	params := &discordgo.WebhookParams{
		Content:   content,
		Username:  displayName,
		AvatarURL: avatarURL,
	}

	if replyTo != "" {
		content = fmt.Sprintf("replying to `%s`\n%s", replyTo, content)
		params.Content = content
	}

	message, err := c.Session.WebhookExecute(
		webhook.ID,
		webhook.Token,
		true,
		params,
	)
	return message, err
}

func (c *Client) EditMessage(channelID, messageID, content string) error {
	webhook, ok := c.Webhooks[channelID]
	if !ok {
		return fmt.Errorf("webhookd not found %s", channelID)
	}

	_, err := c.Session.WebhookMessageEdit(
		webhook.ID,
		webhook.Token,
		messageID,
		&discordgo.WebhookEdit{
			Content: &content,
		},
	)

	return err
}

func (c *Client) AddHandler(fn interface{}) {
	c.Session.AddHandler(fn)
}

func (c *Client) Close() error {
	return c.Session.Close()
}
