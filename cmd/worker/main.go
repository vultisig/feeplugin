package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/vultisig/feeplugin/internal/health"

	"github.com/vultisig/verifier/plugin"
	"github.com/vultisig/verifier/plugin/keysign"
	"github.com/vultisig/verifier/plugin/tasks"
	"github.com/vultisig/verifier/plugin/tx_indexer"
	"github.com/vultisig/verifier/plugin/tx_indexer/pkg/storage"
	"github.com/vultisig/verifier/vault"
	"github.com/vultisig/vultisig-go/relay"

	"github.com/vultisig/feeplugin/internal/fee"
	"github.com/vultisig/feeplugin/internal/storage/postgres"
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
		er := healthServer.Start(ctx, logger)
		if er != nil {
			logger.Errorf("health server failed: %v", er)
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
