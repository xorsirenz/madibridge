package bridge

import (
	"database/sql"
	"fmt"
	"log"
	"sync"

	_ "github.com/lib/pq"
	"maunium.net/go/mautrix/id"

	"github.com/xorsirenz/madibridge/internal/config"
	"github.com/xorsirenz/madibridge/internal/discord"
	"github.com/xorsirenz/madibridge/internal/matrix"
)

type Bridge struct {
	cfg             *config.Config
	matrix          *matrix.Client
	matrixToDiscord map[string]string
	discord         *discord.Client
	discordToMatrix map[string]string
	avatarCache     map[id.UserID]string
	cacheLock       sync.RWMutex
	db              *sql.DB
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

	d, err := discord.New(cfg.Discord.Token)
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

	discordToMatrix := make(map[string]string)
	matrixToDiscord := make(map[string]string)

	for i, br := range cfg.Bridges {
		resolvedMatrixRoom, err := m.ResolveMatrixRoom(br.MatrixRoomID)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve room %s %w", br.MatrixRoomID, err)
		}

		cfg.Bridges[i].MatrixRoomID = resolvedMatrixRoom

		discordToMatrix[br.DiscordChannelID] = resolvedMatrixRoom
		matrixToDiscord[resolvedMatrixRoom] = br.DiscordChannelID
	}

	b := &Bridge{
		cfg:             cfg,
		matrix:          m,
		matrixToDiscord: matrixToDiscord,
		discord:         d,
		discordToMatrix: discordToMatrix,
		avatarCache:     make(map[id.UserID]string),
		db:              db,
	}

	if err := b.ensureTables(); err != nil {
		return nil, fmt.Errorf("failed creating tables: %w", err)
	}

	b.registerDiscordHandlers()
	b.registerMatrixHandlers()

	for d, m := range discordToMatrix {
		log.Println("bridge mapping:", d, "->", m)
	}

	return b, nil
}
