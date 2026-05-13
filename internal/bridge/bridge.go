package bridge

import (
	"database/sql"
	"fmt"
	"sync"

	_ "github.com/lib/pq"
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

func New(cfg *config.Config) (*Bridge, error) {
	m, err := matrix.New(
		cfg.Matrix.Homeserver,
		cfg.Matrix.UserID,
		cfg.Matrix.AccessToken,
	)
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
		return nil, fmt.Errorf("failed creating tables: %w", err)
	}

	b.registerDiscordHandlers()
	b.registerMatrixHandlers()

	return b, nil
}
