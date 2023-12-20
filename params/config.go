package params

import (
	"math/big"

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
}
