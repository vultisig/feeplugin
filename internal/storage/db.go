package storage

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DatabaseStorage interface {
	Close() error

	GetPublicKeys(ctx context.Context) ([]string, error)
	InsertPublicKey(ctx context.Context, publicKey string) error

	Pool() *pgxpool.Pool
}
