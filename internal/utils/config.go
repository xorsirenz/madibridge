package utils

import (
	"encoding/json"
	"os"
)

type Config struct {
	Discord DiscordConfig `json:"discord"`
	Matrix  MatrixConfig  `json:"matrix"`
}

type DiscordConfig struct {
	Token     string `json:"token"`
	ChannelID string `json:"channel_id"`
}

type MatrixConfig struct {
	Homeserver  string `json:"homeserver"`
	UserID      string `json:"user_id"`
	AccessToken string `json:"access_token"`
	RoomID      string `json:"room_id"`
}

func LoadConfig() (*Config, error) {
	configFile, err := os.ReadFile("config.json")
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = json.Unmarshal(configFile, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}
