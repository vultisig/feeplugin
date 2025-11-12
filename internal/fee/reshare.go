package fee

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/vultisig/verifier/types"
)

func (fp *FeePlugin) HandleReshareDKLS(ctx context.Context, t *asynq.Task) error {
	err := fp.vaultService.HandleReshareDKLS(ctx, t)
	if err != nil {
		return err
	}

	var req types.ReshareRequest
	if err := json.Unmarshal(t.Payload(), &req); err != nil {
		fp.logger.WithError(err).Error("json.Unmarshal failed")
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	return fp.db.InsertPublicKey(ctx, req.PublicKey)
}
