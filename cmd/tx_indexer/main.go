package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kelseyhightower/envconfig"

	"github.com/vultisig/verifier/plugin"
	"github.com/vultisig/verifier/plugin/tx_indexer"
	"github.com/vultisig/verifier/plugin/tx_indexer/pkg/config"
	"github.com/vultisig/verifier/plugin/tx_indexer/pkg/storage"

	"github.com/vultisig/feeplugin/internal/health"
	"github.com/vultisig/feeplugin/internal/logging"
	"github.com/vultisig/feeplugin/internal/metrics"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := newConfig()
	if err != nil {
		panic(fmt.Errorf("config.ReadTxIndexerConfig: %w", err))
	}

	logger := logging.NewLogger(cfg.LogFormat)

	// Start metrics server with HTTP metrics for server
	metricsServer := metrics.StartMetricsServer(cfg.Metrics, []string{metrics.ServiceHTTP}, logger)
	defer func() {
		if metricsServer != nil {
			if err := metricsServer.Stop(ctx); err != nil {
				logger.Errorf("failed to stop metrics server: %v", err)
			}
		}
	}()

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
		logger.Fatalf("failed to initialize tx_indexer storage: %v", err)
	}

	rpcs, err := tx_indexer.Rpcs(ctx, cfg.Rpc)
	if err != nil {
		logger.Fatalf("failed to initialize RPCs: %v", err)
	}

	txMetrics := metrics.NewTxIndexerMetrics()

	worker := tx_indexer.NewWorker(
		logger,
		cfg.Interval,
		cfg.IterationTimeout,
		cfg.MarkLostAfter,
		cfg.Concurrency,
		txStorage,
		rpcs,
		txMetrics,
	)

	healthServer := health.New(cfg.HealthPort)
	go func() {
		err = healthServer.Start(ctx, logger)
		if err != nil {
			logger.Errorf("health server failed: %v", err)
		}
	}()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Infof("received shutdown signal, shutting down gracefully")
		cancel()
	}()

	err = worker.Run()
	if err != nil {
		logger.Fatalf("failed to run worker: %v", err)
	}
}

type indexerConfig struct {
	LogFormat  logging.LogFormat `envconfig:"log_format" default:"text"`
	HealthPort int               `envconfig:"health_port" default:"80"`
	Metrics    metrics.Config
	config.Config
}

func newConfig() (indexerConfig, error) {
	var cfg indexerConfig
	err := envconfig.Process("", &cfg)
	if err != nil {
		return indexerConfig{}, fmt.Errorf("failed to process env var: %w", err)
	}
	return cfg, nil
}
