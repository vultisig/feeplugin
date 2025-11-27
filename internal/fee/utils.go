package fee

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/vultisig/mobile-tss-lib/tss"
	"github.com/vultisig/recipes/ethereum"
)

func ComputeTxHash(proposedTx []byte, sigs map[string]tss.KeysignResponse, chainId *big.Int) (string, error) {
	if len(sigs) != 1 {
		return "", fmt.Errorf("expected exactly one signature, got %d", len(sigs))
	}

	var sigRes tss.KeysignResponse
	for _, s := range sigs {
		sigRes = s
	}

	payloadDecoded, err := ethereum.DecodeUnsignedPayload(proposedTx)
	if err != nil {
		return "", fmt.Errorf("DecodeUnsignedPayload: %w", err)
	}

	var sig []byte
	sig = append(sig, common.FromHex(sigRes.R)...)
	sig = append(sig, common.FromHex(sigRes.S)...)
	sig = append(sig, common.FromHex(sigRes.RecoveryID)...)

	tx, err := types.NewTx(payloadDecoded).WithSignature(types.LatestSignerForChainID(chainId), sig)
	if err != nil {
		return "", fmt.Errorf("NewTx.WithSignature: %w", err)
	}
	return tx.Hash().Hex(), nil
}
