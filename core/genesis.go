package core

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie"

	"github.com/ethereum-mive/mive/params"
)

type Genesis struct {
	Config *params.ChainConfig `json:"config"`
	Alloc  core.GenesisAlloc   `json:"alloc" gencodec:"required"`
}

var errGenesisNoConfig = errors.New("genesis has no chain configuration")

func SetupGenesisBlockWithOverride(ctx context.Context, db ethdb.Database, triedb *trie.Database, genesis *Genesis, overrides *core.ChainOverrides, ethClient *ethclient.Client) (*params.ChainConfig, common.Hash, error) {
	if genesis != nil && genesis.Config == nil {
		return &params.ChainConfig{}, common.Hash{}, errGenesisNoConfig
	}
	applyOverrides := func(config *params.ChainConfig) {
		if config != nil {
			if overrides != nil && overrides.OverrideCancun != nil {
				config.Eth.CancunTime = overrides.OverrideCancun
			}
			if overrides != nil && overrides.OverrideVerkle != nil {
				config.Eth.VerkleTime = overrides.OverrideVerkle
			}
		}
	}
	if genesis == nil {
		genesis = DefaultGenesisBlock()
	}
	applyOverrides(genesis.Config)
	genesisNum := genesis.Config.Mive.GenesisBlock
	genesisBlock, err := ethClient.BlockByNumber(ctx, genesisNum)
	if err != nil {
		return &params.ChainConfig{}, common.Hash{}, err
	}
	genesisHash := genesisBlock.Hash()

	header := rawdb.ReadHeader(db, genesisHash, genesisNum.Uint64())
	if header.Root != types.EmptyRootHash && !triedb.Initialized(header.Root) {
	}

	return &params.ChainConfig{}, common.Hash{}, nil
}

// DefaultGenesisBlock returns the Mive genesis block for the Ethereum mainnet.
func DefaultGenesisBlock() *Genesis {
	return &Genesis{
		Config: params.MainnetChainConfig,
		Alloc:  make(core.GenesisAlloc),
	}
}
