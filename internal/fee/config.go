package fee

import (
	"errors"
	"fmt"

	"github.com/vultisig/vultisig-go/common"
)

var supportedChains = []common.Chain{
	common.Ethereum,
}

func getSupportedChainStrings() []string {
	var cc []string
	for _, c := range supportedChains {
		cc = append(cc, c.String())
	}
	return cc
}

// These are properties and parameters specific to the fee plugin config. They should be distinct from system/core config
type FeeConfig struct {
	Type            string `mapstructure:"type,omitempty"`
	Version         string `mapstructure:"version,omitempty"`
	MaxFeeAmount    uint64 `mapstructure:"max_fee_amount,omitempty"`   // Policies that are created/submitted which do not have this amount will be rejected.
	UsdcAddress     string `mapstructure:"usdc_address,omitempty"`     // The address of the USDC token on the Ethereum blockchain.
	TreasuryAddress string `mapstructure:"treasury_address,omitempty"` // The address of the Vultisig Treasury on the Ethereum blockchain.
	VerifierToken   string `mapstructure:"verifier_token,omitempty"`   // The token to use for the verifier API.
	ChainId         uint64 `mapstructure:"chain_id,omitempty"`         // The chain ID of the Ethereum blockchain.
	EthProvider     string `mapstructure:"eth_provider,omitempty"`     // The Ethereum provider to use for the fee plugin.
	Jobs            struct {
		Load struct {
			MaxConcurrentJobs uint64 `mapstructure:"max_concurrent_jobs,omitempty"` //How many consecutive tasks can take place
			Cronexpr          string `mapstructure:"cronexpr,omitempty"`            // Cron link expression on how often these tasks should run
		} `mapstructure:"load,omitempty"`
		Transact struct {
			MaxConcurrentJobs uint64 `mapstructure:"max_concurrent_jobs,omitempty"` //How many consecutive tasks can take place
			Cronexpr          string `mapstructure:"cronexpr,omitempty"`            // Cron link expression on how often these tasks should run
		} `mapstructure:"transact,omitempty"`
		Post struct {
			SuccessConfirmations uint64 `mapstructure:"success_confirmations,omitempty"` //How many consecutive tasks can take place
			Cronexpr             string `mapstructure:"cronexpr,omitempty"`              // Cron link expression on how often these tasks should run
			MaxConcurrentJobs    uint64 `mapstructure:"max_concurrent_jobs,omitempty"`
		} `mapstructure:"post,omitempty"`
	}
}

func DefaultFeeConfig() *FeeConfig {
	c := new(FeeConfig)
	c.ChainId = 1
	c.Type = PLUGIN_TYPE
	c.Version = "1.0.0"
	c.MaxFeeAmount = 500e6 // 500 USDC

	c.Jobs.Load.MaxConcurrentJobs = 10
	c.Jobs.Transact.MaxConcurrentJobs = 10
	c.Jobs.Post.MaxConcurrentJobs = 10
	c.Jobs.Post.SuccessConfirmations = 20

	c.Jobs.Load.Cronexpr = "@every 2m"
	c.Jobs.Transact.Cronexpr = "0 12 * * 5"
	c.Jobs.Post.Cronexpr = "@every 5m"
	return c
}

func (c *FeeConfig) Validate() error {
	// Validate configuration
	if c.Type != PLUGIN_TYPE {
		return fmt.Errorf("invalid plugin type: %s", c.Type)
	}

	if c.VerifierToken == "" {
		return errors.New("verifier_token is required")
	}

	if c.ChainId == 0 {
		return errors.New("chain_id is required")
	}

	//if c.EthProvider == "" {
	//	return errors.New("eth_provider is required")
	//}

	if c.Jobs.Load.MaxConcurrentJobs < 1 ||
		c.Jobs.Load.MaxConcurrentJobs > 100 ||
		c.Jobs.Transact.MaxConcurrentJobs < 1 ||
		c.Jobs.Transact.MaxConcurrentJobs > 100 ||
		c.Jobs.Post.MaxConcurrentJobs < 1 ||
		c.Jobs.Post.MaxConcurrentJobs > 100 {
		return errors.New("max_concurrent_jobs must be greater than 0 and less than 100")
	}

	return nil
}
