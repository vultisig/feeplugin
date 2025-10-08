package storage

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vultisig/feeplugin/internal/verifierapi"
	"github.com/vultisig/verifier/types"
)

type DatabaseStorage interface {
	Close() error

	GetFees(ctx context.Context, feeIds ...uuid.UUID) ([]types.Fee, error)
	CreateFee(ctx context.Context, fee verifierapi.FeeDto) error

	//CreateFeeRun(ctx context.Context, policyId uuid.UUID, state types.FeeRunState, fees ...verifierapi.FeeDto) (*types.FeeRun, error)
	//SetFeeRunSent(ctx context.Context, runId uuid.UUID, txHash string) error
	//SetFeeRunSuccess(ctx context.Context, runId uuid.UUID) error
	//GetAllFeeRuns(ctx context.Context, statuses ...types.FeeRunState) ([]types.FeeRun, error) // If no statuses are provided, all fee runs are returned.
	//GetPendingFeeRun(ctx context.Context, policyId uuid.UUID) (*types.FeeRun, error)
	//GetFeeRuns(ctx context.Context, state types.FeeRunState) ([]types.FeeRun, error)

	Pool() *pgxpool.Pool
}
