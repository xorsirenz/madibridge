package bridge

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	_ "github.com/lib/pq"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/xorsirenz/madibridge/internal/config"
	"github.com/xorsirenz/madibridge/internal/discord"
	"github.com/xorsirenz/madibridge/internal/matrix"
)

type Bridge struct {
	cfg         *config.Config
	matrix      *matrix.Client
	discord     *discord.Client
	avatarCache map[id.UserID]string
	cacheLock   sync.RWMutex
	db          *sql.DB
}

// creates a new bridge
func New(cfg *config.Config) (*Bridge, error) {
	m, err := matrix.New(cfg.Matrix.Homeserver, cfg.Matrix.UserID, cfg.Matrix.AccessToken)
	if err != nil {
		return nil, err
	}

	d, err := discord.New(cfg.Discord.Token, cfg.Discord.ChannelID)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("postgres", cfg.DB.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping db: %w", err)
	}

	b := &Bridge{
		cfg:         cfg,
		matrix:      m,
		discord:     d,
		avatarCache: make(map[id.UserID]string),
		db:          db,
	}

	if err := b.ensureTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	b.setupHandlers()
	return b, nil
}

// sets up bi-directional message bridging
func (b *Bridge) setupHandlers() {
	// discord -> matrix
	b.discord.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.Bot || m.ChannelID != b.cfg.Discord.ChannelID {
			return
		}

		contentBody := m.Content

		if len(m.Attachments) > 0 {
			var sb strings.Builder

			sb.WriteString(contentBody)

			for _, a := range m.Attachments {
				sb.WriteString("\n")
				sb.WriteString(a.URL)
			}

			contentBody = sb.String()
		}

		msgContent := &event.MessageEventContent{
			MsgType: event.MsgText,
			Body:    contentBody,
		}

		if m.MessageReference != nil {
			matrixReplyID, err := b.getMatrixID(m.MessageReference.MessageID)
			if err == nil {
				msgContent.RelatesTo = &event.RelatesTo{
					InReplyTo: &event.InReplyTo{
						EventID: id.EventID(matrixReplyID),
					},
				}
			}
		}

		resp, err := b.matrix.SendMessageEvent(
			context.Background(),
			id.RoomID(b.cfg.Matrix.RoomID),
			event.EventMessage,
			msgContent,
		)

		if err != nil {
			log.Println("matrix send error:", err)
			return
		}

		if err := b.storeMessageMap(
			m.ID,
			string(resp.EventID),
			"",
		); err != nil {
			log.Println("failed to store map:", err)
		}
	})

	b.discord.AddHandler(func(s *discordgo.Session, m *discordgo.MessageUpdate) {
		if m.Author == nil || m.Author.Bot || m.ChannelID != b.cfg.Discord.ChannelID {
			return
		}

		matrixID, err := b.getMatrixID(m.ID)
		if err != nil {
			return
		}

		_, err = b.matrix.SendMessageEvent(
			context.Background(),
			id.RoomID(b.cfg.Matrix.RoomID),
			event.EventMessage,
			&event.MessageEventContent{
				MsgType: event.MsgText,
				Body: "* " + m.Content,

				NewContent: &event.MessageEventContent{
					MsgType: event.MsgText,
					Body: m.Content,
				},

				RelatesTo: &event.RelatesTo{
					Type: event.RelReplace,
					EventID: id.EventID(matrixID),
				},
			},
		)

		if err != nil {
			log.Println("matrix edit message error:", err)
		}
	})

	// matrix -> discord
	syncer := mautrix.NewDefaultSyncer()
	b.matrix.Syncer = syncer
	syncer.OnEventType(event.EventMessage, func(ctx context.Context, evt *event.Event) {
		if evt.RoomID.String() != b.cfg.Matrix.RoomID || evt.Sender.String() == b.cfg.Matrix.UserID {
			return
		}

		content := evt.Content.AsMessage()
		displayName := evt.Sender.Localpart()
		avatarURL := b.getAvatarURL(ctx, evt.Sender)

		if profile, err := b.matrix.Client.GetProfile(ctx, evt.Sender); err == nil && profile.DisplayName != "" {
			displayName = profile.DisplayName
		}

		if content.RelatesTo != nil && content.RelatesTo.Type == event.RelReplace {
			webhookMsgID, err := b.getDiscordWebhookID(
				string(content.RelatesTo.EventID),
			)

			if err == nil && content.NewContent != nil {
				err = b.discord.EditMessage(
					webhookMsgID,
					content.NewContent.Body,
				)

				if err != nil {
					log.Println("discord message edit error:", err)
				}
			}

			return
		}

		replyTo := ""

		if content.RelatesTo != nil && content.RelatesTo.InReplyTo != nil {
			replyDiscordID, err := b.getDiscordID(
				string(content.RelatesTo.InReplyTo.EventID),
			)

			if err == nil {
				replyTo = replyDiscordID
			}
		}

		body := content.Body

		switch content.MsgType {
		case event.MsgImage, event.MsgFile, event.MsgVideo:
			if content.URL != "" {
				body += "\n" + b.mxcToHTTP(string(content.URL))
			}
		}

		msg, err := b.discord.SendMessage(
			displayName,
			avatarURL,
			body,
			replyTo,
		)

		if err != nil {
			log.Println("discord send message error:", err)
			return
		}

		err = b.storeMessageMap(msg.ID, string(evt.ID), msg.ID)

		if err != nil {
			log.Println("map store error:", err)
		}
	})
}

// fetch avatar from cache or server
func (b *Bridge) getAvatarURL(ctx context.Context, user id.UserID) string {
	b.cacheLock.RLock()
	if url, ok := b.avatarCache[user]; ok {
		b.cacheLock.RUnlock()
		return url
	}
	b.cacheLock.RUnlock()

	profile, err := b.matrix.Client.GetProfile(ctx, user)
	if err != nil || profile.AvatarURL.String() == "" {
		return ""
	}

	mxc := profile.AvatarURL.String()
	withoutPrefix := strings.TrimPrefix(mxc, "mxc://")
	parts := strings.SplitN(withoutPrefix, "/", 2)
	if len(parts) != 2 {
		return ""
	}

	avatarURL := fmt.Sprintf("%s/_matrix/media/v3/download/%s/%s", b.cfg.Matrix.Homeserver, parts[0], parts[1])

	b.cacheLock.Lock()
	b.avatarCache[user] = avatarURL
	b.cacheLock.Unlock()
	return avatarURL
}

func (b *Bridge) mxcToHTTP(mxc string) string {
	mxcPrefix := strings.TrimPrefix(mxc, "mxc://")

	parts := strings.SplitN(mxcPrefix, "/", 2)
	if len(parts) != 2 {
		return ""
	}

	return fmt.Sprintf("%s/_matrix/media/v3/download/%s/%s", b.cfg.Matrix.Homeserver, parts[0], parts[1])
}

// verify table exists
func (b *Bridge) ensureTables() error {
	_, err := b.db.Exec(`
		CREATE TABLE IF NOT EXISTS message_map (
			discord_id TEXT UNIQUE,
			matrix_id TEXT UNIQUE
		);
	`)
	return err
}

func (b *Bridge) storeMessageMap(discordID, matrixID, discordWebhookID string) error {
	_, err := b.db.Exec(`
		INSERT INTO message_map(
			discord_id, 
			matrix_id,
			discord_webhook_id
		)
		VALUES($1, $2, $3)
		ON CONFLICT DO NOTHING
	`, discordID, matrixID, discordWebhookID)

	return err
}

func (b *Bridge) getMatrixID(discordID string) (string, error) {
	var matrixID string

	err := b.db.QueryRow(`
		SELECT matrix_id
		FROM message_map
		WHERE discord_id=$1
	`, discordID).Scan(&matrixID)

	return matrixID, err
}

func (b *Bridge) getDiscordID(matrixID string) (string, error) {
	var discordID string

	err := b.db.QueryRow(`
		SELECT discord_id
		FROM message_map
		WHERE matrix_id=$1
	`, matrixID).Scan(&discordID)

	return discordID, err
}

func (b *Bridge) getDiscordWebhookID(matrixID string) (string, error) {
	var webhookID string

	err := b.db.QueryRow(`
		SELECT discord_webhook_id
		FROM message_map
		WHERE matrix_id$1
	`, matrixID).Scan(&webhookID)

	return webhookID, err
}

// starts the bridge and handles shutdown
func (b *Bridge) Run() error {
	if err := b.discord.Open(); err != nil {
		return err
	}
	defer func() {
		if err := b.discord.Close(); err != nil {
			log.Println("error closing Discord:", err)
		} else {
			log.Println("discord closed")
		}
	}()

	log.Println("discord connected")

	stop := make(chan os.Signal, 1)
	syncDone := make(chan struct{})
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// start matrix sync
	go func() {
		defer close(syncDone)
		if err := b.matrix.Sync(); err != nil {
			log.Println("matrix sync error:", err)
		} else {
			log.Println("matrix sync ended gracefully")
		}
	}()
	log.Println("matrix sync started")

	// wait for shutdown signal or sync completion
	select {
	case <-stop:
		log.Println("shutdown signal received")
	case <-syncDone:
		log.Println("matrix sync finished")
	}

	time.Sleep(500 * time.Millisecond)
	log.Println("bridge shutting down")
	return nil
}
