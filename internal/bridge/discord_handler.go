package bridge

import (
	"context"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

func (b *Bridge) registerDiscordHandlers() {
	b.discord.AddHandler(b.onDiscordMessageCreate)
	b.discord.AddHandler(b.onDiscordMessageUpdate)
}

func (b *Bridge) onDiscordMessageCreate(
	s *discordgo.Session, 
	m *discordgo.MessageCreate,
) {
	if m.Author.Bot {
		return
	}

	matrixRoomID, ok := b.discordToMatrix[m.ChannelID]
	if !ok {
		return
	}

	var parts []string

	if m.Content != "" {
		parts = append(parts, m.Content)
	}

	for _, attachment := range m.Attachments {
		parts = append(parts, attachment.URL)
	}

	body := strings.Join(parts, "\n")
	content := &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    body,
	}

	if m.MessageReference != nil {
		b.applyDiscordReply(content, m.MessageReference.MessageID)
	}

	resp, err := b.matrix.SendMessageEvent(
		context.Background(),
		id.RoomID(matrixRoomID),
		event.EventMessage,
		content,
	)

	if err != nil {
		log.Println("matrix send error:", err)
		return
	}

	if err := b.storeMessageMap(MessageMap{
		DiscordID: m.ID,
		MatrixID:  string(resp.EventID),
		Username:  m.Author.Username,
	}); err != nil {
		log.Println("store map error:", err)
	}
}

func (b *Bridge) onDiscordMessageUpdate(
	s *discordgo.Session,
	m *discordgo.MessageUpdate,
) {
	if m.Author == nil {
		return
	}

	if m.Author.Bot {
		return
	}

	if m.WebhookID != "" {
		return
	}

	if m.Content == "" {
		return
	}

	matrixRoomID, ok := b.discordToMatrix[m.ChannelID]
	if !ok {
		return
	}

	matrixID, err := b.getMatrixID(m.ID)
	if err != nil {
		return
	}

	_, err = b.matrix.SendMessageEvent(
		context.Background(),
		id.RoomID(matrixRoomID),
		event.EventMessage,
		&event.MessageEventContent{
			MsgType: event.MsgText,
			Body:    "* " + m.Content,

			NewContent: &event.MessageEventContent{
				MsgType: event.MsgText,
				Body:    m.Content,
			},

			RelatesTo: &event.RelatesTo{
				Type:    event.RelReplace,
				EventID: id.EventID(matrixID),
			},
		},
	)

	if err != nil {
		log.Println("matrix edit error:", err)
	}
}

func (b *Bridge) applyDiscordReply(
	content *event.MessageEventContent,
	discordMessageID string,
) {
	matrixReplyID, err := b.getMatrixID(discordMessageID)

	if err != nil || matrixReplyID == "" {
		return
	}

	content.RelatesTo = &event.RelatesTo{
		InReplyTo: &event.InReplyTo{
			EventID: id.EventID(matrixReplyID),
		},
	}
}
