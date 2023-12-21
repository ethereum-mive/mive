package params

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

type ChainConfig struct {
	Eth  *params.ChainConfig `json:"eth,omitempty"`
	Mive *MiveChainConfig    `json:"mive,omitempty"`
}

type MiveChainConfig struct {
	// Genesis block at which Mive starts indexing and executing.
	// For any specific network, it should not be changed after Mive launched.
	GenesisBlock *big.Int `json:"genesisBlock,omitempty"`

	BeaconAddress      common.Address `json:"beaconAddress"`
	BeneficiaryAddress common.Address `json:"beneficiaryAddress"`
}

var (
	// DefaultBeaconAddress is the default beacon address, which has suffix "315e" (a variant of "mive").
	DefaultBeaconAddress = common.HexToAddress("0x000000000000000000000000000000000000315e")
)

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
