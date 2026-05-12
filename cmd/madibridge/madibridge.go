package main

import (
	"log"
	"os"

	"github.com/xorsirenz/madibridge/internal/bridge"
	"github.com/xorsirenz/madibridge/internal/config"
	"github.com/xorsirenz/madibridge/internal/utils"
)

var version string

func main() {
	if len(os.Args) > 1 {
		utils.HandleCmd(version)
	}

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
