package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
	"github.com/vultisig/verifier/plugin/config"
	"github.com/vultisig/verifier/plugin/server"
	"github.com/vultisig/verifier/vault_config"
)

type FeeServerConfig struct {
	Server         server.Config             `mapstructure:"server" json:"server"`
	Postgres       config.Database           `mapstructure:"database" json:"database,omitempty"`
	BaseConfigPath string                    `mapstructure:"base_config_path" json:"base_config_path,omitempty"`
	Redis          config.Redis              `mapstructure:"redis" json:"redis,omitempty"`
	BlockStorage   vault_config.BlockStorage `mapstructure:"block_storage" json:"block_storage,omitempty"`
}

func GetConfigure() (*FeeServerConfig, error) {
	configName := os.Getenv("VS_CONFIG_NAME")
	if configName == "" {
		configName = "config"
	}

	return ReadConfig(configName)
}

func ReadConfig(configName string) (*FeeServerConfig, error) {
	viper.SetConfigName(configName)
	viper.AddConfigPath(".")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	viper.SetDefault("Server.VaultsFilePath", "vaults")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file, %w", err)
	}
	var cfg FeeServerConfig
	err := viper.Unmarshal(&cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to decode into struct, %w", err)
	}
	return &cfg, nil
}
