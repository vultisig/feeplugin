package verifierapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/vultisig/verifier/types"
)

func (v *VerifierApi) GetPublicKeysFees(ecdsaPublicKey string) ([]*types.Fee, error) {
	response, err := v.getAuth(fmt.Sprintf("/fees/publickey/%s", ecdsaPublicKey))
	if err != nil {
		return nil, fmt.Errorf("failed to get public key fees: %w", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			v.logger.WithError(err).Error("Failed to close response body")
		}
	}()
	if response.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("public key not found")
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get public key fees, status code: %d", response.StatusCode)
	}

	var feeHistory APIResponse[[]*types.Fee]
	if err := json.NewDecoder(response.Body).Decode(&feeHistory); err != nil {
		return nil, fmt.Errorf("failed to decode public key fees response: %w", err)
	}

	if feeHistory.Error.Message != "" {
		return nil, fmt.Errorf("failed to get public key fees, error: %s, details: %s", feeHistory.Error.Message, feeHistory.Error.DetailedResponse)
	}

	return feeHistory.Data, nil
}

func (v *VerifierApi) MarkFeeAsCollected(amount uint64, txHash, network string, feeIds ...uint64) error {

	var body = struct {
		ID      uint64 `json:"id"`
		TxHash  string `json:"tx_hash"`
		Network string `json:"network"`
		Amount  uint64 `json:"amount"`
	}{
		ID:      feeIds[0],
		TxHash:  txHash,
		Network: network,
		Amount:  amount,
	}

	url := "/fees/collected"
	response, err := v.postAuth(url, body)
	if err != nil {
		return fmt.Errorf("failed to mark fee as collected: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to mark fee as collected, status code: %d", response.StatusCode)
	}

	return nil
}
