package main

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/vultisig/feeplugin/internal/fee"
	"github.com/vultisig/verifier/plugin/config"
	"github.com/vultisig/verifier/vault_config"
)

type FeeWorkerConfig struct {
	Redis              config.Redis              `mapstructure:"redis" json:"redis,omitempty"`
	Verifier           config.Verifier           `mapstructure:"verifier" json:"verifier,omitempty"`
	BlockStorage       vault_config.BlockStorage `mapstructure:"block_storage" json:"block_storage,omitempty"`
	VaultServiceConfig vault_config.Config       `mapstructure:"vault_service" json:"vault_service,omitempty"`
	BaseConfigPath     string                    `mapstructure:"base_config_path" json:"base_config_path,omitempty"`
	Database           config.Database           `mapstructure:"database" json:"database,omitempty"`
	FeeConfig          fee.FeeConfig             `mapstructure:"fee_config" json:"fee_config,omitempty"`
	ProcessingInterval time.Duration             `mapstructure:"processing_interval" json:"processing_interval,omitempty"`
}

func GetConfigure() (*FeeWorkerConfig, error) {
	configName := os.Getenv("VS_CONFIG_NAME")
	if configName == "" {
		configName = "config"
	}
	return ReadConfig(configName)
}

func ReadConfig(configName string) (*FeeWorkerConfig, error) {
	if configName == "" {
		configName = "config"
	}
	addKeysToViper(viper.GetViper(), reflect.TypeOf(FeeWorkerConfig{}))
	viper.SetConfigName(configName)
	viper.AddConfigPath(".")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("fail to reading config file, %w", err)
		}
		// This is required for ENV configs
	}
	var cfg FeeWorkerConfig
	err := viper.Unmarshal(&cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to decode into struct, %w", err)
	}
	return &cfg, nil
}

func addKeysToViper(v *viper.Viper, t reflect.Type) {
	keys := getAllKeys(t)
	for _, key := range keys {
		v.SetDefault(key, "")
	}
}

func getAllKeys(t reflect.Type) []string {
	var result []string

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		// Try mapstructure tag first
		tagName := f.Tag.Get("mapstructure")
		if tagName == "" || tagName == "-" {
			// Fallback to JSON tag
			jsonTag := f.Tag.Get("json")
			if jsonTag != "" && jsonTag != "-" {
				// Handle comma-separated options (e.g., "field_name,omitempty")
				tagName = strings.Split(jsonTag, ",")[0]
			}
		} else {
			// Handle comma-separated options in mapstructure tag
			tagName = strings.Split(tagName, ",")[0]
		}

		// Final fallback to field name if no valid tags found
		if tagName == "" || tagName == "-" {
			tagName = f.Name
		}

		n := strings.ToUpper(tagName)

		if reflect.Struct == f.Type.Kind() {
			subKeys := getAllKeys(f.Type)
			for _, k := range subKeys {
				result = append(result, n+"."+k)
			}
		} else {
			result = append(result, n)
		}
	}

	return result
}
