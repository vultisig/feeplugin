package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/vultisig/verifier/plugin"
	"github.com/vultisig/verifier/plugin/config"
	"github.com/vultisig/verifier/plugin/policy"
	"github.com/vultisig/verifier/plugin/policy/policy_pg"
	"github.com/vultisig/verifier/plugin/redis"
	"github.com/vultisig/verifier/plugin/scheduler/scheduler_pg"
	"github.com/vultisig/verifier/plugin/server"
	"github.com/vultisig/verifier/vault"
	"github.com/vultisig/verifier/vault_config"

	"github.com/vultisig/feeplugin/internal/fee"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := logrus.New()
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.DebugLevel)

	cfg, err := GetConfigure()
	if err != nil {
		logger.Fatalf("failed to load config: %v", err)
	}

	redisClient, err := redis.NewRedis(cfg.Redis)
	if err != nil {
		logger.Fatalf("failed to initialize Redis client: %v", err)
	}

	asynqConnOpt, err := asynq.ParseRedisURI(cfg.Redis.URI)
	if err != nil {
		logger.Fatalf("failed to parse redis URI: %v", err)
	}

	asynqClient := asynq.NewClient(asynqConnOpt)
	asynqInspector := asynq.NewInspector(asynqConnOpt)

	vaultStorage, err := vault.NewBlockStorageImp(cfg.BlockStorage)
	if err != nil {
		logger.Fatalf("failed to initialize Vault storage: %v", err)
	}

	pgPool, err := pgxpool.New(ctx, cfg.Postgres.DSN)
	if err != nil {
		logger.Fatalf("failed to initialize Postgres pool: %v", err)
	}

	policyStorage, err := plugin.WithMigrations(
		logger,
		pgPool,
		policy_pg.NewRepo,
		"policy/policy_pg/migrations",
	)
	if err != nil {
		logger.Fatalf("failed to initialize policy storage: %v", err)
	}

	schedulerStorage, err := plugin.WithMigrations(
		logger,
		pgPool,
		scheduler_pg.NewRepo,
		"scheduler/scheduler_pg/migrations",
	)
	if err != nil {
		logger.Fatalf("failed to initialize scheduler storage: %v", err)
	}

	policyService, err := policy.NewPolicyService(
		policyStorage,
		fee.NewSchedulerService(schedulerStorage),
		logger,
	)
	if err != nil {
		logger.Fatalf("failed to initialize policy service: %v", err)
	}

	srv := server.NewServer(
		cfg.Server,
		policyService,
		redisClient,
		vaultStorage,
		asynqClient,
		asynqInspector,
		fee.NewSpec(),
		server.DefaultMiddlewares(),
	)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Infof("received shutdown signal, shutting down gracefully")
		cancel()
	}()

	if err := srv.Start(ctx); err != nil {
		logger.Fatalf("failed to start server: %v", err)
	}
}

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
	if configName == "" {
		configName = "config"
	}
	addKeysToViper(viper.GetViper(), reflect.TypeOf(FeeServerConfig{}))
	viper.SetConfigName(configName)
	viper.AddConfigPath(".")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	viper.SetDefault("Server.VaultsFilePath", "vaults")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("fail to reading config file, %w", err)
		}
		// This is required for ENV configs
	}
	var cfg FeeServerConfig
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
