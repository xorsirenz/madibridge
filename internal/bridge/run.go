package bridge

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"maunium.net/go/mautrix/id"
)

func (b *Bridge) Run() error {
	var channelIDs []string

	for _, bridges := range b.cfg.Bridges {
		channelIDs = append(channelIDs, bridges.DiscordChannelID)
	}

	if err := b.discord.Open(channelIDs); err != nil {
		return err
	}

	defer b.discord.Close()
	log.Println("discord connected")

	stop := make(chan os.Signal, 1)
	syncDone := make(chan struct{})

	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	b.matrix.Store.SaveNextBatch(
		context.Background(),
		id.UserID(b.cfg.Matrix.UserID),
		b.getSyncToken(),
	)

	go b.runMatrixSync(syncDone)
	go b.persistSyncToken()

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

func (b *Bridge) runMatrixSync(done chan struct{}) {
	defer close(done)

	for {
		err := b.matrix.Sync()

		if err != nil {
			log.Println("matrix sync error:", err)
			time.Sleep(5 * time.Second)
			continue
		}

		break
	}
}

func (b *Bridge) persistSyncToken() {
	for {
		time.Sleep(5 * time.Second)
		token, err := b.matrix.Store.LoadNextBatch(context.Background(), id.UserID(b.cfg.Matrix.UserID))

		if err != nil || token == "" {
			continue
		}

		if err := b.setSyncToken(token); err != nil {
			log.Println("failed saving sync token:", err)
		}
	}
}
