package fee

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	gcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	v1 "github.com/vultisig/commondata/go/vultisig/vault/v1"
	"github.com/vultisig/mobile-tss-lib/tss"
	"github.com/vultisig/recipes/sdk/evm"
	"github.com/vultisig/verifier/plugin/keysign"
	"github.com/vultisig/verifier/plugin/tx_indexer"
	vstorage "github.com/vultisig/verifier/plugin/tx_indexer/pkg/storage"
	vtypes "github.com/vultisig/verifier/types"
	"github.com/vultisig/verifier/vault"
	"github.com/vultisig/vultisig-go/address"
	"github.com/vultisig/vultisig-go/common"

	"github.com/vultisig/feeplugin/internal/metrics"
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

	eth    *evm.SDK
	ethRpc *ethclient.Client
	signer *keysign.Signer

	metrics *metrics.WorkerMetrics
}

func NewFeePlugin(
	config *FeeConfig,
	logger *logrus.Logger,
	vaultService *vault.ManagementService,
	vault vault.Storage,
	vaultSecret string,
	ethRpc *ethclient.Client,
	signer *keysign.Signer,
	txIndexerService *tx_indexer.Service,
	db storage.DatabaseStorage,
	verifierUrl string,
	pi time.Duration) (*FeePlugin, error) {
	verifierApi := verifierapi.NewVerifierApi(
		verifierUrl,
		config.VerifierToken,
		logger.WithField("pkg", "verifierapi").Logger,
	)
	var eth = new(evm.SDK)
	if ethRpc != nil {
		ethEvmChainID, err := common.Ethereum.EvmID()
		if err != nil {
			return nil, fmt.Errorf("failed to get Ethereum EVM ID: %w", err)
		}
		eth = evm.NewSDK(ethEvmChainID, ethRpc, ethRpc.Client())
	}
	return &FeePlugin{
		config:                config,
		logger:                logger,
		vaultService:          vaultService,
		vault:                 vault,
		vaultEncryptionSecret: vaultSecret,
		txIndexerService:      txIndexerService,
		db:                    db,
		verifierApi:           verifierApi,
		processingInterval:    pi,

		eth:    eth,
		ethRpc: ethRpc,
		signer: signer,

		metrics: metrics.NewWorkerMetrics(),
	}, nil
}

func (fp *FeePlugin) Run(ctx context.Context) {
	if fp.processingInterval == 0 {
		fp.processingInterval = 1 * time.Minute
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
	var count atomic.Int64
	for _, pk := range pks {
		fees, err := fp.verifierApi.GetPublicKeysFees(pk)
		if err != nil {
			return fmt.Errorf("failed to get fees: %w", err)
		}

		var eg errgroup.Group
		for _, fee := range fees {
			fp.logger.WithFields(logrus.Fields{
				"fee_id":     fee.ID,
				"amount":     fee.Amount,
				"pubkey":     pk,
				"fee_type":   fee.FeeType,
				"created_at": fee.CreatedAt,
			}).Info("processing fee")

			eg.Go(func() error {
				err := fp.executeFeeTransaction(ctx, pk, fee.Amount, fee.ID)
				success := err == nil
				if fp.metrics != nil {
					fp.metrics.RecordSendTransaction(fp.config.TreasuryAddress, common.Ethereum.String(), success)
				}

				if err != nil {
					fp.logger.WithError(err).Error("failed to process fee transaction")
					if fp.metrics != nil {
						fp.metrics.RecordError(metrics.ErrorTypeExecution)
					}
					return err
				}

				count.Add(1)
				return nil
			})
		}

		err = eg.Wait()
		if err != nil {
			return fmt.Errorf("eg.Wait: %w", err)
		}
	}

	fp.logger.Info("processed fees: ", count.Load())

	return nil
}

func (fp *FeePlugin) executeFeeTransaction(ctx context.Context, publickey string, amount uint64, feeId ...uint64) error {
	vault, err := getVaultForPubKey(fp.vault, publickey, fp.vaultEncryptionSecret)
	if err != nil {
		return fmt.Errorf("failed to get vault: %w", err)
	}

	ethAddress, _, _, err := address.GetAddress(vault.PublicKeyEcdsa, vault.HexChainCode, common.Ethereum)
	if err != nil {
		return fmt.Errorf("failed to get eth address: %w", err)
	}

	chain := common.Ethereum

	tx, e := fp.genUnsignedTx(
		ctx,
		ethAddress,
		fp.config.TreasuryAddress,
		fp.config.UsdcAddress,
		new(big.Int).SetUint64(amount),
	)
	if e != nil {
		return fmt.Errorf("p.genUnsignedTx: %w", e)
	}

	txHex := base64.StdEncoding.EncodeToString(tx)

	txToTrack, e := fp.txIndexerService.CreateTx(ctx, vstorage.CreateTxDto{
		PluginID:      vtypes.PluginVultisigFees_feee,
		ChainID:       chain,
		FromPublicKey: publickey,
		ToPublicKey:   ethAddress,
		ProposedTxHex: txHex,
	})
	if e != nil {
		return fmt.Errorf("p.txIndexerService.CreateTx: %w", e)
	}

	signRequest, e := vtypes.NewPluginKeysignRequestEvm(
		vtypes.PluginPolicy{
			PluginID:  vtypes.PluginVultisigFees_feee,
			PublicKey: publickey,
		}, txToTrack.ID.String(), chain, tx)

	return fp.initSign(ctx, signRequest, amount, false, feeId...)
}

func (fp *FeePlugin) initSign(
	ctx context.Context,
	req *vtypes.PluginKeysignRequest,
	amount uint64,
	waitMined bool,
	feeId ...uint64,
) error {
	if req == nil {
		return fmt.Errorf("req is nil")
	}
	sigs, err := fp.signer.Sign(ctx, *req)
	if err != nil {
		fp.logger.WithError(err).Error("Keysign failed")
		return fmt.Errorf("failed to sign transaction: %w", err)
	}

	if len(sigs) != 1 {
		fp.logger.
			WithField("sigs_count", len(sigs)).
			Error("expected only 1 message+sig per request for evm")
		return fmt.Errorf("failed to sign transaction: invalid signature count: %d", len(sigs))
	}
	var sig tss.KeysignResponse
	for _, s := range sigs {
		sig = s
	}

	txBytes, err := base64.StdEncoding.DecodeString(req.Transaction)
	if err != nil {
		return fmt.Errorf("failed to decode b64 proposed tx: %w", err)
	}
	txHex := gcommon.Bytes2Hex(txBytes)

	err = fp.verifierApi.MarkFeeAsCollected(amount, txHex, common.Ethereum.String(), feeId...)
	if err != nil {
		return fmt.Errorf("failed to mark fee as collected: %w", err)
	}

	tx, err := fp.broadcast(ctx, txBytes, sig, *req)
	if err != nil {
		fp.logger.WithError(err).Error("failed to complete signing process (broadcast tx)")
		return fmt.Errorf("failed to complete signing process: %w", err)
	}

	if waitMined {
		fp.logger.Println("waiting for tx being mined")
		receipt, err := bind.WaitMined(ctx, fp.ethRpc, tx)
		if err != nil {
			fp.logger.WithError(err).Error("failed to wait tx being mined")
			return fmt.Errorf("failed to wait tx being mined: %w", err)
		}
		if receipt.Status != types.ReceiptStatusSuccessful {
			return fmt.Errorf("tx failed: receipt status %v", receipt.Status)
		}
	}
	return nil
}

func (fp *FeePlugin) broadcast(
	ctx context.Context,
	txBytes []byte,
	signature tss.KeysignResponse,
	signRequest vtypes.PluginKeysignRequest,
) (*types.Transaction, error) {
	tx, err := fp.eth.Send(
		ctx,
		txBytes,
		gcommon.Hex2Bytes(signature.R),
		gcommon.Hex2Bytes(signature.S),
		gcommon.Hex2Bytes(signature.RecoveryID),
	)
	if err != nil {
		return nil, fmt.Errorf("p.eth.Send(tx_hex=%s): %w", gcommon.Bytes2Hex(txBytes), err)
	}

	fp.logger.WithFields(logrus.Fields{
		"from_public_key": signRequest.PublicKey,
		"to_address":      tx.To().Hex(),
		"hash":            tx.Hash().Hex(),
		"chain":           common.Ethereum.String(),
	}).Info("tx successfully signed and broadcasted")
	return tx, nil
}

func (fp *FeePlugin) genUnsignedTx(
	ctx context.Context,
	fromAddress string,
	toAddress string,
	contractAddress string,
	amount *big.Int,
) ([]byte, error) {
	tx, err := fp.eth.MakeTxTransferERC20(
		ctx,
		gcommon.HexToAddress(fromAddress),
		gcommon.HexToAddress(toAddress),
		gcommon.HexToAddress(contractAddress),
		amount,
	)
	if err != nil {
		return nil, fmt.Errorf("p.eth.MakeTxTransferNative: %v", err)
	}
	return tx, nil
}

func getVaultForPubKey(s vault.Storage, pubKey, encryptionSecret string) (*v1.Vault, error) {
	vaultFileName := common.GetVaultBackupFilename(pubKey, vtypes.PluginVultisigFees_feee.String())
	vaultContent, err := s.GetVault(vaultFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault")
	}

	if vaultContent == nil {
		return nil, fmt.Errorf("vault not found")
	}

	return common.DecryptVaultFromBackup(encryptionSecret, vaultContent)
}
