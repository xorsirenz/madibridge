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

	if isIgnoredMessage(m.Author) {
		return
	}

	roomID, ok := b.matrixRoomID(m.ChannelID)
	if !ok {
		return
	}

	content := b.buildMessageContent(m.Message)

	resp, err := b.matrix.SendMessageEvent(
		context.Background(),
		roomID,
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
	if shouldIgnoreUpdate(m) {
		return
	}

	roomID, ok := b.matrixRoomID(m.ChannelID)
	if !ok {
		return
	}

	matrixID, err := b.getMatrixID(m.ID)
	if err != nil {
		return
	}

	_, err = b.matrix.SendMessageEvent(
		context.Background(),
		roomID,
		event.EventMessage,
		buildEditContent(m.Content, matrixID),
	)

	if err != nil {
		log.Println("matrix edit error:", err)
	}
}

func (b *Bridge) buildMessageContent(m *discordgo.Message) *event.MessageEventContent {
	var parts []string

	if m.Content != "" {
		parts = append(parts, m.Content)
	}

	for _, attachment := range m.Attachments {
		parts = append(parts, attachment.URL)
	}

	content := &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    strings.Join(parts, "\n"),
	}

	if m.MessageReference != nil {
		b.applyDiscordReply(content, m.MessageReference.MessageID)
	}

	return content
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

func buildEditContent(
	body string,
	matrixID string,
) *event.MessageEventContent {
	return &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    "* " + body,

		NewContent: &event.MessageEventContent{
			MsgType: event.MsgText,
			Body:    body,
		},

		RelatesTo: &event.RelatesTo{
			Type:    event.RelReplace,
			EventID: id.EventID(matrixID),
		},
	}
}

func isIgnoredMessage(author *discordgo.User) bool {
	return author == nil || author.Bot
}

func shouldIgnoreUpdate(m *discordgo.MessageUpdate) bool {
	return m.Author == nil ||
		m.Author.Bot ||
		m.WebhookID != "" ||
		m.Content == ""
}

func (b *Bridge) matrixRoomID(channelID string) (id.RoomID, bool) {
	roomID, ok := b.discordToMatrix[channelID]
	return id.RoomID(roomID), ok
}
