package main

import (
	"fmt"
	"log"

	"github.com/xorsirenz/madibridge/internal/utils"
)

func main() {
	cfg, err := utils.LoadConfig()
	if err != nil {
		log.Fatal("failed to load config:", err)
	}

	fmt.Println(cfg.Matrix.Homeserver)
}
