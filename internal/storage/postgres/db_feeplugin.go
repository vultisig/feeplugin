package postgres

import (
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/sirupsen/logrus"
)

//go:embed migrations/feeplugin/*.sql
var feePluginMigrations embed.FS

// FeePluginMigrationManager handles fee-plugin-specific migrations
type FeePluginMigrationManager struct {
	pool *pgxpool.Pool
}

func NewFeePluginMigrationManager(pool *pgxpool.Pool) *FeePluginMigrationManager {
	return &FeePluginMigrationManager{pool: pool}
}

func (fp *FeePluginMigrationManager) Migrate() error {
	logrus.Info("Starting feePlugin database migration...")
	goose.SetBaseFS(feePluginMigrations)
	defer goose.SetBaseFS(nil)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	db := stdlib.OpenDBFromPool(fp.pool)
	defer db.Close()
	if err := goose.Up(db, "migrations/feeplugin", goose.WithAllowMissing()); err != nil {
		return fmt.Errorf("failed to run feePlugin migrations: %w", err)
	}
	logrus.Info("feePlugin database migration completed successfully")
	return nil
}
