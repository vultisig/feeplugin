package fee

import (
	"errors"
	"fmt"
	"math/big"
)

// These are properties and parameters specific to the fee plugin config. They should be distinct from system/core config
type FeeConfig struct {
	Type          string `mapstructure:"type"`
	Version       string `mapstructure:"version"`
	MaxFeeAmount  uint64 `mapstructure:"max_fee_amount"` // Policies that are created/submitted which do not have this amount will be rejected.
	UsdcAddress   string `mapstructure:"usdc_address"`   // The address of the USDC token on the Ethereum blockchain.
	VerifierToken string `mapstructure:"verifier_token"` // The token to use for the verifier API.
	chainId       uint64 `mapstructure:"chain_id"`       // The chain ID of the Ethereum blockchain.
	ChainId       *big.Int
	EthProvider   string `mapstructure:"eth_provider"` // The Ethereum provider to use for the fee plugin.
	Jobs          struct {
		Load struct {
			MaxConcurrentJobs uint64 `mapstructure:"max_concurrent_jobs"` //How many consecutive tasks can take place
			Cronexpr          string `mapstructure:"cronexpr"`            // Cron link expression on how often these tasks should run
		} `mapstructure:"load"`
		Transact struct {
			MaxConcurrentJobs uint64 `mapstructure:"max_concurrent_jobs"` //How many consecutive tasks can take place
			Cronexpr          string `mapstructure:"cronexpr"`            // Cron link expression on how often these tasks should run
		} `mapstructure:"transact"`
		Post struct {
			SuccessConfirmations uint64 `mapstructure:"success_confirmations"` //How many consecutive tasks can take place
			Cronexpr             string `mapstructure:"cronexpr"`              // Cron link expression on how often these tasks should run
			MaxConcurrentJobs    uint64 `mapstructure:"max_concurrent_jobs"`
		} `mapstructure:"post"`
	}
}

func DefaultFeeConfig() *FeeConfig {
	c := new(FeeConfig)
	c.ChainId = big.NewInt(1)
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

	if c.ChainId == nil {
		return errors.New("chain_id is required")
	}

	if c.EthProvider == "" {
		return errors.New("eth_provider is required")
	}

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
