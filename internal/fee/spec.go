package fee

import (
	"strings"

	"github.com/vultisig/recipes/types"
	"github.com/vultisig/verifier/plugin"
)

type Spec struct {
	plugin.Unimplemented
}

func NewSpec() *Spec {
	return &Spec{}
}

func (s *Spec) GetRecipeSpecification() (*types.RecipeSchema, error) {
	return &types.RecipeSchema{
		Version:            1,
		PluginId:           "vultisig-fees-feee",
		PluginName:         "Billing",
		PluginVersion:      1,
		SupportedResources: s.buildSupportedResources(),
		Requirements: &types.PluginRequirements{
			MinVultisigVersion: 1,
			SupportedChains:    getSupportedChainStrings(),
		},
		Permissions: []*types.Permission{
			{
				Id:          "transaction_signing",
				Label:       "Access to transaction signing",
				Description: "The app can initiate transactions to send assets in your Vault",
			},
			{
				Id:          "token_swap",
				Label:       "Automatic token conversion",
				Description: "The app can swap collected fees to USDC via DEX aggregators",
			},
			{
				Id:          "balance_visibility",
				Label:       "Vault balance visibility",
				Description: "The app can view Vault balances",
			},
		},
	}, nil
}

func (s *Spec) buildSupportedResources() []*types.ResourcePattern {
	var resources []*types.ResourcePattern
	for _, chain := range supportedChains {
		chainNameLower := strings.ToLower(chain.String())

		resources = append(resources, &types.ResourcePattern{
			ResourcePath: &types.ResourcePath{
				ChainId:    chainNameLower,
				ProtocolId: "send",
				FunctionId: "Access to transaction signing",
				Full:       chainNameLower + ".send",
			},
			Target: types.TargetType_TARGET_TYPE_UNSPECIFIED,
			ParameterCapabilities: []*types.ParameterConstraintCapability{
				{
					ParameterName:  "asset",
					SupportedTypes: types.ConstraintType_CONSTRAINT_TYPE_FIXED,
					Required:       false,
				},
				{
					ParameterName:  "from_address",
					SupportedTypes: types.ConstraintType_CONSTRAINT_TYPE_FIXED,
					Required:       true,
				},
				{
					ParameterName:  "amount",
					SupportedTypes: types.ConstraintType_CONSTRAINT_TYPE_FIXED,
					Required:       true,
				},
				{
					ParameterName:  "to_address",
					SupportedTypes: types.ConstraintType_CONSTRAINT_TYPE_MAGIC_CONSTANT,
					Required:       true,
				},
				{
					ParameterName:  "memo",
					SupportedTypes: types.ConstraintType_CONSTRAINT_TYPE_ANY,
					Required:       false,
				},
			},
			Required: true,
		})
	}

	return resources
}
