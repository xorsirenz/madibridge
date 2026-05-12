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

		eventID := "discord:" + m.ID
		if b.isEventSent(eventID) {
			return
		}

		displayName := m.Author.Username
		message := fmt.Sprintf("%s: %s", displayName, m.Content)

		_, err := b.matrix.SendMessageEvent(
			context.Background(),
			id.RoomID(b.cfg.Matrix.RoomID),
			event.EventMessage,
			&event.MessageEventContent{
				MsgType: event.MsgText,
				Body:    message,
			},
		)
		if err != nil {
			log.Println("matrix send error:", err)
			return
		}

		if err := b.markEventSent(eventID); err != nil {
			log.Println("failed to mark discord event as sent:", err)
		}
	})

	// matrix -> discord
	syncer := mautrix.NewDefaultSyncer()
	b.matrix.Syncer = syncer
	syncer.OnEventType(event.EventMessage, func(ctx context.Context, evt *event.Event) {
		if evt.RoomID.String() != b.cfg.Matrix.RoomID || evt.Sender.String() == b.cfg.Matrix.UserID {
			return
		}

		eventID := "matrix:" + string(evt.ID)
		if b.isEventSent(eventID) {
			return
		}

		content := evt.Content.AsMessage()
		if content.MsgType != event.MsgText {
			return
		}

		displayName := evt.Sender.Localpart()
		avatarURL := b.getAvatarURL(ctx, evt.Sender)

		if profile, err := b.matrix.Client.GetProfile(ctx, evt.Sender); err == nil && profile.DisplayName != "" {
			displayName = profile.DisplayName
		}

		if err := b.discord.SendMessage(displayName, avatarURL, content.Body); err != nil {
			log.Println("discord send error:", err)
			return
		}

		if err := b.markEventSent(eventID); err != nil {
			log.Println("failed to mark matrix event as sent:", err)
		}

		log.Printf("matrix -> discord: %s (user=%s, avatar=%s)\n", content.Body, displayName, avatarURL)
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

	avatarURL := fmt.Sprintf("%s/_matrix/media/r0/download/%s/%s", b.cfg.Matrix.Homeserver, parts[0], parts[1])

	b.cacheLock.Lock()
	b.avatarCache[user] = avatarURL
	b.cacheLock.Unlock()
	return avatarURL
}

// postgres dedupe helper
func (b *Bridge) isEventSent(eventID string) bool {
	var id string
	err := b.db.QueryRow(`SELECT event_id FROM sent_events WHERE event_id=$1`, eventID).Scan(&id)
	return err == nil
}

func (b *Bridge) markEventSent(eventID string) error {
	_, err := b.db.Exec(`INSERT INTO sent_events(event_id) VALUES($1) ON CONFLICT DO NOTHING`, eventID)
	return err
}

// verify table exists
func (b *Bridge) ensureTables() error {
	_, err := b.db.Exec(`
		CREATE TABLE IF NOT EXISTS sent_events (
			event_id TEXT PRIMARY KEY
		);
	`)
	return err
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
