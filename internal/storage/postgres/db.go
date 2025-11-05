package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/vultisig/feeplugin/internal/storage"
)

var _ storage.DatabaseStorage = (*PostgresBackend)(nil)

type PostgresBackend struct {
	logger *logrus.Logger
	pool   *pgxpool.Pool
}

func NewPostgresBackend(logger *logrus.Logger, dsn string) (*PostgresBackend, error) {
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	backend := &PostgresBackend{
		logger: logger,
		pool:   pool,
	}

	if err := backend.Migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return backend, nil
}

func (p *PostgresBackend) Close() error {
	p.pool.Close()

	return nil
}

func (p *PostgresBackend) Migrate() error {
	p.logger.Info("Starting database migration...")

	feeMgr := NewFeePluginMigrationManager(p.pool)
	if err := feeMgr.Migrate(); err != nil {
		return fmt.Errorf("failed to run feePlugin migrations: %w", err)
	}

	p.logger.Info("Database migration completed successfully")
	return nil
}

func (p *PostgresBackend) WithTransaction(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Roll back on error *or* panic.
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	if err := fn(ctx, tx); err != nil {
		er := tx.Rollback(ctx)
		return errors.Join(err, er)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (p *PostgresBackend) Pool() *pgxpool.Pool {
	return p.pool
}
