package fee

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"
	"github.com/vultisig/feeplugin/internal/verifierapi"
	rtypes "github.com/vultisig/recipes/types"
	"github.com/vultisig/verifier/plugin"
	"github.com/vultisig/verifier/plugin/tx_indexer"
	vtypes "github.com/vultisig/verifier/types"
	"github.com/vultisig/verifier/vault"
)

var _ plugin.Spec = (*FeePlugin)(nil)

type FeePlugin struct {
	logger                *logrus.Logger
	vault                 vault.Storage
	vaultEncryptionSecret string
	txIndexerService      *tx_indexer.Service
	verifierApi           *verifierapi.VerifierApi
	config                *FeeConfig
	encryptionSecret      string
}

func NewFeePlugin(
	config *FeeConfig,
	logger *logrus.Logger,
	vault vault.Storage,
	vaultSecret string,
	txIndexerService *tx_indexer.Service,
	verifierUrl string) *FeePlugin {
	verifierApi := verifierapi.NewVerifierApi(
		verifierUrl,
		config.VerifierToken,
		logger.WithField("pkg", "verifierapi").Logger,
	)
	return &FeePlugin{
		config:                config,
		logger:                logger.WithField("pkg", "fee").Logger,
		vault:                 vault,
		vaultEncryptionSecret: vaultSecret,
		txIndexerService:      txIndexerService,
		verifierApi:           verifierApi,
	}
}

// Policy creation is not expected for this plugin
func (fp *FeePlugin) GetRecipeSpecification() (*rtypes.RecipeSchema, error) {
	return nil, nil
}

func (fp *FeePlugin) ValidatePluginPolicy(policyDoc vtypes.PluginPolicy) error {
	return errors.New("not implemented")
}

// Suggest implements plugin.Spec.
func (fp *FeePlugin) Suggest(configuration map[string]any) (*rtypes.PolicySuggest, error) {
	return nil, errors.New("not implemented")
}

func (fp *FeePlugin) executeFeeTransaction(ctx context.Context, publickey string) error {
	//TODO: implement
	return nil
}
