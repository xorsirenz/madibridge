package main

import (
	"log"

	"github.com/xorsirenz/madibridge/internal/bridge"
	"github.com/xorsirenz/madibridge/internal/config"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("failed to load config:", err)
	}

	bridge, err := bridge.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Bridge running")

	if err := bridge.Run(); err != nil {
		log.Fatal(err)
	}
}
