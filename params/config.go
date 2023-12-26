package params

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

var (
	// MainnetChainConfig is the chain parameters to run a node on the main network.
	MainnetChainConfig = &ChainConfig{
		Eth: params.MainnetChainConfig,
		Mive: &MiveChainConfig{
			GenesisBlock:  new(big.Int), // TODO
			BeaconAddress: DefaultBeaconAddress,
		},
	}
)

type ChainConfig struct {
	Eth  *params.ChainConfig `json:"eth,omitempty"`
	Mive *MiveChainConfig    `json:"mive,omitempty"`
}

type MiveChainConfig struct {
	// Genesis block at which Mive starts indexing and executing.
	// For any specific network, it should not be changed after Mive launched.
	GenesisBlock *big.Int `json:"genesisBlock,omitempty"`

	// Beacon address that will be observed by Mive for transactions sent to it.
	// These transactions will be interpreted and executed by the Mive EVM.
	// For any specific network, it should not be changed after Mive launched.
	BeaconAddress common.Address `json:"beaconAddress"`
}

// FeeReductionDenominator bounds the reduction amount the various fees may have in Mive.
func (c *ChainConfig) FeeReductionDenominator() uint64 {
	return DefaultFeeReductionDenominator
}

// BlockGasLimitMultiplier bounds the maximum gas limit a Mive block may have.
func (c *ChainConfig) BlockGasLimitMultiplier() uint64 {
	return DefaultBlockGasLimitMultiplier
}

// MinBlockGasLimit is the minimum gas limit for a Mive block.
func (c *ChainConfig) MinBlockGasLimit() uint64 {
	return DefaultMinBlockGasLimit
}
