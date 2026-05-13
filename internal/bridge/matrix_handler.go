package bridge

import (
	"context"
	"fmt"
	"log"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

func (b *Bridge) registerMatrixHandlers() {
	syncer := mautrix.NewDefaultSyncer()
	b.matrix.Syncer = syncer
	syncer.OnEventType(event.EventMessage, b.onMatrixMessage)
}

func (b *Bridge) onMatrixMessage(ctx context.Context, evt *event.Event) {
	if evt.RoomID.String() != b.cfg.Matrix.RoomID {
		return
	}

	if evt.Sender.String() == b.cfg.Matrix.UserID {
		return
	}

	content := evt.Content.AsMessage()

	if b.handleMatrixEdit(content) {
		return
	}

	body := b.buildMatrixMessageBody(content)
	displayName := evt.Sender.Localpart()

	if profile, err := b.matrix.Client.GetProfile(
		ctx,
		evt.Sender,
	); err == nil {
		if profile.DisplayName != "" {
			displayName = profile.DisplayName
		}
	}

	avatarURL := b.getAvatarURL(ctx, evt.Sender)
	msg, err := b.discord.SendMessage(
		displayName,
		avatarURL,
		body,
		"",
	)

	if err != nil {
		log.Println("discord send error:", err)
		return
	}

	if err := b.storeMessageMap(MessageMap{
		DiscordID:           msg.ID,
		MatrixID:            string(evt.ID),
		DiscordWebhookMsgID: msg.ID,
		Username:            displayName,
	}); err != nil {
		log.Println("store map error:", err)
	}
}

func (b *Bridge) handleMatrixEdit(content *event.MessageEventContent) bool {
	if content.RelatesTo == nil {
		return false
	}

	if content.RelatesTo.Type != event.RelReplace {
		return false
	}

	webhookMsgID, err := b.getDiscordWebhookID(string(content.RelatesTo.EventID))

	if err != nil {
		return true
	}

	if content.NewContent == nil {
		return true
	}

	if err := b.discord.EditMessage(
		webhookMsgID,
		content.NewContent.Body,
	); err != nil {
		log.Println("discord edit error:", err)
	}

	return true
}

func (b *Bridge) buildMatrixMessageBody(content *event.MessageEventContent) string {
	body := content.Body
	body = b.applyMatrixReply(body, content)

	switch content.MsgType {
	case event.MsgImage, event.MsgFile, event.MsgVideo:
		if content.URL != "" {
			body += "\n" + b.mxcToHTTP(string(content.URL))
		}
	}

	return body
}

func (b *Bridge) applyMatrixReply(body string, content *event.MessageEventContent) string {
	if content.RelatesTo == nil {
		return body
	}

	if content.RelatesTo.InReplyTo == nil {
		return body
	}

	matrixReplyID := string(content.RelatesTo.InReplyTo.EventID)
	discordID, err := b.getDiscordID(matrixReplyID)

	if err != nil || discordID == "" {
		return body
	}

	username, err := b.getDiscordUsername(discordID)

	if err != nil || username == "" {
		username = discordID
	}

	return fmt.Sprintf("> %s\n%s", username, body)
}
