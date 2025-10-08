package main

import (
	"context"
	"net"
	"os"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/vultisig/verifier/plugin"
	"github.com/vultisig/verifier/plugin/tasks"
	"github.com/vultisig/verifier/plugin/tx_indexer"
	"github.com/vultisig/verifier/plugin/tx_indexer/pkg/storage"
	"github.com/vultisig/verifier/vault"

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

	sdClient, err := statsd.New(cfg.Datadog.Host + ":" + cfg.Datadog.Port)
	if err != nil {
		logger.Fatalf("failed to initialize StatsD client: %v", err)
	}
	vaultStorage, err := vault.NewBlockStorageImp(cfg.BlockStorage)
	if err != nil {
		logger.Fatalf("failed to initialize vault storage: %v", err)
	}

	redisOptions := asynq.RedisClientOpt{
		Addr:     net.JoinHostPort(cfg.Redis.Host, cfg.Redis.Port),
		Username: cfg.Redis.User,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}

	client := asynq.NewClient(redisOptions)
	consumer := asynq.NewServer(
		redisOptions,
		asynq.Config{
			Logger:      logger,
			Concurrency: 10,
			Queues: map[string]int{
				tasks.QUEUE_NAME: 10,
			},
		},
	)

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

	txIndexerService := tx_indexer.NewService(
		logger,
		txStorage,
		supportedChains,
	)

	vaultService, err := vault.NewManagementService(
		cfg.VaultServiceConfig,
		client,
		sdClient,
		vaultStorage,
		txIndexerService,
	)
	if err != nil {
		logger.Fatalf("failed to create vault service: %v", err)
	}

	feeConfig := fee.DefaultFeeConfig()
	feeConfig.VerifierToken = cfg.Verifier.Token
	feeConfig.EthProvider = cfg.FeeConfig.EthProvider

	err = feeConfig.Validate()
	if err != nil {
		logger.Fatalf("invalid fee config: %v", err)
	}

	feePlugin := fee.NewFeePlugin(
		feeConfig,
		logger,
		vaultStorage,
		cfg.VaultServiceConfig.EncryptionSecret,
		txIndexerService,
		cfg.Verifier.URL,
	)

	_ = feePlugin

	mux := asynq.NewServeMux()
	mux.HandleFunc(tasks.TypeKeySignDKLS, vaultService.HandleKeySignDKLS)
	mux.HandleFunc(tasks.TypeReshareDKLS, vaultService.HandleReshareDKLS)
	err = consumer.Run(mux)
	if err != nil {
		logger.Fatalf("failed to run consumer: %v", err)
	}
}
