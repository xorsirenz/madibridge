package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Matrix struct {
		Homeserver  string `yaml:"homeserver"`
		UserID      string `yaml:"user_id"`
		AccessToken string `yaml:"access_token"`
		RoomID      string `yaml:"room_id"`
	} `yaml:"matrix"`

	Discord struct {
		Token     string `yaml:"token"`
		ChannelID string `yaml:"channel_id"`
	} `yaml:"discord"`

	DB struct {
		DSN string `yaml:"dsn"`
	} `yaml:"db"`
}

func LoadConfig() (*Config, error) {
	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = "config.yaml"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("cannot parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func require(field, name string, missing *[]string) {
	if field == "" {
		*missing = append(*missing, name)
	}
}

func (c *Config) Validate() error {
	var missing []string

	require(c.Matrix.Homeserver, "matrix.homeserver", &missing)
	require(c.Matrix.UserID, "matrix.user_id", &missing)
	require(c.Matrix.AccessToken, "matrix.access_token", &missing)
	require(c.Matrix.RoomID, "matrix.room_id", &missing)
	require(c.Discord.Token, "discord.token", &missing)
	require(c.Discord.ChannelID, "discord.channel_id", &missing)
	require(c.DB.DSN, "db.dsn", &missing)

	if len(missing) > 0 {
		return fmt.Errorf("invalid config: missing required fields: %v", missing)
	}

	return nil
}
