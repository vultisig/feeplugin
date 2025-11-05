package fee

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/vultisig/verifier/plugin/tx_indexer"
	"github.com/vultisig/verifier/vault"

	"github.com/vultisig/feeplugin/internal/storage"
	"github.com/vultisig/feeplugin/internal/verifierapi"
)

type FeePlugin struct {
	logger                *logrus.Logger
	vaultService          *vault.ManagementService
	vault                 vault.Storage
	vaultEncryptionSecret string
	txIndexerService      *tx_indexer.Service
	verifierApi           *verifierapi.VerifierApi
	db                    storage.DatabaseStorage
	config                *FeeConfig
	encryptionSecret      string
	processingInterval    time.Duration
}

func NewFeePlugin(
	config *FeeConfig,
	logger *logrus.Logger,
	vaultService *vault.ManagementService,
	vault vault.Storage,
	vaultSecret string,
	txIndexerService *tx_indexer.Service,
	db storage.DatabaseStorage,
	verifierUrl string,
	pi time.Duration) *FeePlugin {
	verifierApi := verifierapi.NewVerifierApi(
		verifierUrl,
		config.VerifierToken,
		logger.WithField("pkg", "verifierapi").Logger,
	)
	return &FeePlugin{
		config:                config,
		logger:                logger.WithField("pkg", "fee").Logger,
		vaultService:          vaultService,
		vault:                 vault,
		vaultEncryptionSecret: vaultSecret,
		txIndexerService:      txIndexerService,
		db:                    db,
		verifierApi:           verifierApi,
		processingInterval:    pi,
	}
}

func (fp *FeePlugin) Run(ctx context.Context) {
	if fp.processingInterval == 0 {
		fp.processingInterval = 5 * time.Minute
	}
	ticker := time.NewTicker(fp.processingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := fp.ProcessFees(ctx)
			if err != nil {
				fp.logger.WithError(err).Error("failed to process fees")
			}
		case <-ctx.Done():
			return
		}
	}
}

func (fp *FeePlugin) ProcessFees(ctx context.Context) error {
	pks, err := fp.db.GetPublicKeys(ctx)
	if err != nil {
		return fmt.Errorf("failed to get public keys: %w", err)
	}
	for _, pk := range pks {
		fees, err := fp.verifierApi.GetPublicKeysFees(pk)
		if err != nil {
			return fmt.Errorf("failed to get fees: %w", err)
		}

		for _, fee := range fees {
			fp.logger.Info("processing fee:", fee)
		}
	}

	return nil
}

func (fp *FeePlugin) executeFeeTransaction(ctx context.Context, publickey string) error {
	//TODO: implement
	return nil
}
