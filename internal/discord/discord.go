package discord

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

const webhookName = "madibridge"

type Client struct {
	Session   *discordgo.Session
	ChannelID string
	Webhook   *discordgo.Webhook
}

func New(token string, channelID string) (*Client, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	dg.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsMessageContent |
		discordgo.IntentsGuildMessageReactions

	return &Client{
		Session:   dg,
		ChannelID: channelID,
	}, nil
}

func (c *Client) Open() error {
	if err := c.Session.Open(); err != nil {
		return err
	}

	webhook, err := c.ensureWebhook()
	if err != nil {
		return err
	}

	c.Webhook = webhook
	log.Println("discord connected and webhook ready")
	return nil
}

func (c *Client) ensureWebhook() (*discordgo.Webhook, error) {
	webhooks, err := c.Session.ChannelWebhooks(c.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("failed to list webhooks: %w", err)
	}

	for _, wh := range webhooks {
		if wh.Name == webhookName {
			return wh, nil
		}
	}

	webhook, err := c.Session.WebhookCreate(c.ChannelID, webhookName, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook: %w", err)
	}

	return webhook, nil
}

func (c *Client) SendMessage(displayName, avatarURL, content string, replyTo string) (*discordgo.Message, error) {
	if c.Webhook == nil {
		return nil, fmt.Errorf("webhook not initialized")
	}

	params := &discordgo.WebhookParams{
		Content: content,
		Username: displayName,
		AvatarURL: avatarURL,
	}

	if replyTo != "" {
		content = fmt.Sprintf("replying to `%s`\n%s", replyTo, content)
		params.Content = content
	}


	message, err := c.Session.WebhookExecute(
		c.Webhook.ID,
		c.Webhook.Token,
		true, 
		params,
	)
	return message, err
}

func (c *Client) EditMessage(messageID, content string) error {
	_, err := c.Session.WebhookMessageEdit(
		c.Webhook.ID,
		c.Webhook.Token,
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
