package main

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/vultisig/verifier/safety"

	"github.com/vultisig/verifier/plugin"
	"github.com/vultisig/verifier/plugin/config"
	"github.com/vultisig/verifier/plugin/keysign"
	"github.com/vultisig/verifier/plugin/tasks"
	"github.com/vultisig/verifier/plugin/tx_indexer"
	"github.com/vultisig/verifier/plugin/tx_indexer/pkg/storage"
	"github.com/vultisig/verifier/safety"
	"github.com/vultisig/verifier/vault"
	"github.com/vultisig/verifier/vault_config"
	"github.com/vultisig/vultisig-go/relay"

	"github.com/vultisig/feeplugin/internal/fee"
	"github.com/vultisig/feeplugin/internal/health"
	"github.com/vultisig/feeplugin/internal/logging"
	"github.com/vultisig/feeplugin/internal/metrics"
	"github.com/vultisig/feeplugin/internal/storage/postgres"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := GetConfigure()
	if err != nil {
		logrus.Fatalf("failed to load config: %v", err)
	}

	logger := logging.NewLogger(cfg.LogFormat)

	// Start metrics server for tx_indexer
	metricsServer := metrics.StartMetricsServer(cfg.Metrics, []string{metrics.ServiceWorker}, logger)
	defer func() {
		if metricsServer != nil {
			if err := metricsServer.Stop(ctx); err != nil {
				logger.Errorf("failed to stop metrics server: %v", err)
			}
		}
	}()

	vaultStorage, err := vault.NewBlockStorageImp(cfg.BlockStorage)
	if err != nil {
		logger.Fatalf("failed to initialize vault storage: %v", err)
	}

	redisConnOpt, err := asynq.ParseRedisURI(cfg.Redis.URI)
	if err != nil {
		logger.Fatalf("failed to parse redis URI: %v", err)
	}

	client := asynq.NewClient(redisConnOpt)
	consumer := asynq.NewServer(
		redisConnOpt,
		asynq.Config{
			Logger:      logger,
			Concurrency: 10,
			Queues: map[string]int{
				tasks.QUEUE_NAME: 10,
			},
		},
	)
	db, err := postgres.NewPostgresBackend(logger, cfg.Database.DSN)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}

	pgPool, err := pgxpool.New(ctx, cfg.Database.DSN)
	if err != nil {
		logger.Fatalf("failed to initialize Postgres pool: %v", err)
	}

	txStorage, err := plugin.WithMigrations(
		logger,
		pgPool,
		storage.NewRepo,
		"tx_indexer/pkg/storage/migrations",
	)
	if err != nil {
		logger.Fatalf("failed to initialize Postgres pool: %v", err)
	}

	supportedChains, err := tx_indexer.Chains()
	if err != nil {
		logger.Fatalf("failed to get supported chains: %v", err)
	}

	rpcClient, err := ethclient.Dial(cfg.FeeConfig.EthProvider)
	if err != nil {
		panic(fmt.Errorf("failed to create eth client: %w", err))
	}

	txIndexerService := tx_indexer.NewService(
		logger,
		txStorage,
		supportedChains,
	)

	vaultService, err := vault.NewManagementService(
		cfg.VaultServiceConfig,
		client,
		vaultStorage,
		txIndexerService,
		safety.NewManager(Temp{}, logger),
	)
	if err != nil {
		logger.Fatalf("failed to create vault service: %v", err)
	}

	feeConfig := fee.DefaultFeeConfig()
	feeConfig.VerifierToken = cfg.Verifier.Token
	feeConfig.EthProvider = cfg.FeeConfig.EthProvider
	feeConfig.TreasuryAddress = cfg.FeeConfig.TreasuryAddress
	feeConfig.UsdcAddress = cfg.FeeConfig.UsdcAddress

	err = feeConfig.Validate()
	if err != nil {
		logger.Fatalf("invalid fee config: %v", err)
	}

	feePlugin, err := fee.NewFeePlugin(
		feeConfig,
		logger,
		vaultService,
		vaultStorage,
		cfg.VaultServiceConfig.EncryptionSecret,
		rpcClient,
		keysign.NewSigner(
			logger.WithField("pkg", "keysign.Signer").Logger,
			relay.NewRelayClient(cfg.VaultServiceConfig.Relay.Server),
			[]keysign.Emitter{
				keysign.NewVerifierEmitter(cfg.Verifier.URL, cfg.Verifier.Token),
				keysign.NewPluginEmitter(client, tasks.TypeKeySignDKLS, tasks.QUEUE_NAME),
			},
			[]string{
				cfg.Verifier.PartyPrefix,
				cfg.VaultServiceConfig.LocalPartyPrefix,
			},
		),
		txIndexerService,
		db,
		cfg.Verifier.URL,
		cfg.ProcessingInterval,
	)
	if err != nil {
		logger.Fatalf("failed to initialize feePlugin: %v", err)
	}

	healthServer := health.New(cfg.HealthPort)
	go func() {
		err = healthServer.Start(ctx, logger)
		if err != nil {
			logger.Errorf("health server failed: %v", err)
		}
	}()

	go feePlugin.Run(ctx)

	mux := asynq.NewServeMux()
	mux.HandleFunc(tasks.TypeKeySignDKLS, vaultService.HandleKeySignDKLS)
	mux.HandleFunc(tasks.TypeReshareDKLS, feePlugin.HandleReshareDKLS)
	err = consumer.Run(mux)
	if err != nil {
		logger.Fatalf("failed to run consumer: %v", err)
	}
}

type FeeWorkerConfig struct {
	LogFormat          logging.LogFormat         `mapstructure:"log_format" json:"log_format,omitempty" default:"text"`
	Redis              config.Redis              `mapstructure:"redis" json:"redis,omitempty"`
	Verifier           config.Verifier           `mapstructure:"verifier" json:"verifier,omitempty"`
	BlockStorage       vault_config.BlockStorage `mapstructure:"block_storage" json:"block_storage,omitempty"`
	VaultServiceConfig vault_config.Config       `mapstructure:"vault_service" json:"vault_service,omitempty"`
	BaseConfigPath     string                    `mapstructure:"base_config_path" json:"base_config_path,omitempty"`
	Database           config.Database           `mapstructure:"database" json:"database,omitempty"`
	FeeConfig          fee.FeeConfig             `mapstructure:"fee_config" json:"fee_config,omitempty"`
	ProcessingInterval time.Duration             `mapstructure:"processing_interval" json:"processing_interval,omitempty"`
	HealthPort         int                       `mapstructure:"health_port" json:"health_port,omitempty"`
	Metrics            metrics.Config            `mapstructure:"metrics" json:"metrics,omitempty"`
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

	viper.SetDefault("LogFormat", "text")

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

// TODO: rework
type Temp struct {
}

func (Temp) GetControlFlags(ctx context.Context, k1 string, k2 string) (map[string]bool, error) {
	return map[string]bool{}, nil
}
