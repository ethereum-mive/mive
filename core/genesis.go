package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/trie/triedb/pathdb"

	miverawdb "github.com/ethereum-mive/mive/core/rawdb"
	mivetypes "github.com/ethereum-mive/mive/core/types"
	"github.com/ethereum-mive/mive/params"
)

//go:generate go run github.com/fjl/gencodec -type Genesis -field-override genesisSpecMarshaling -out gen_genesis.go

var errGenesisNoConfig = errors.New("genesis has no chain configuration")

type Genesis struct {
	Config *params.ChainConfig `json:"config"`
	Alloc  GenesisAlloc        `json:"alloc" gencodec:"required"`
}

// field type overrides for gencodec
type genesisSpecMarshaling struct {
	Alloc map[common.UnprefixedAddress]core.GenesisAccount
}

// GenesisAlloc specifies the initial state that is part of the genesis block.
type GenesisAlloc core.GenesisAlloc

func (ga *GenesisAlloc) UnmarshalJSON(data []byte) error {
	m := make(map[common.UnprefixedAddress]core.GenesisAccount)
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*ga = make(GenesisAlloc)
	for addr, a := range m {
		(*ga)[common.Address(addr)] = a
	}
	return nil
}

// hash computes the state root according to the genesis specification.
func (ga *GenesisAlloc) hash(isVerkle bool, blocknum uint64) (common.Hash, error) {
	// If a genesis-time verkle trie is requested, create a trie config
	// with the verkle trie enabled so that the tree can be initialized
	// as such.
	var config *trie.Config
	if isVerkle {
		config = &trie.Config{
			PathDB:   pathdb.Defaults,
			IsVerkle: true,
		}
	}
	// Create an ephemeral in-memory database for computing hash,
	// all the derived states will be discarded to not pollute disk.
	db := state.NewDatabaseWithConfig(rawdb.NewMemoryDatabase(), config)
	statedb, err := state.New(types.EmptyRootHash, db, nil)
	if err != nil {
		return common.Hash{}, err
	}
	for addr, account := range *ga {
		if account.Balance != nil {
			statedb.AddBalance(addr, account.Balance)
		}
		statedb.SetCode(addr, account.Code)
		statedb.SetNonce(addr, account.Nonce)
		for key, value := range account.Storage {
			statedb.SetState(addr, key, value)
		}
	}
	return statedb.Commit(blocknum, false)
}

// flush is very similar with hash, but the main difference is all the generated
// states will be persisted into the given database. Also, the genesis state
// specification will be flushed as well.
func (ga *GenesisAlloc) flush(db ethdb.Database, triedb *trie.Database, blockhash common.Hash, blocknum uint64) error {
	statedb, err := state.New(types.EmptyRootHash, state.NewDatabaseWithNodeDB(db, triedb), nil)
	if err != nil {
		return err
	}
	for addr, account := range *ga {
		if account.Balance != nil {
			statedb.AddBalance(addr, account.Balance)
		}
		statedb.SetCode(addr, account.Code)
		statedb.SetNonce(addr, account.Nonce)
		for key, value := range account.Storage {
			statedb.SetState(addr, key, value)
		}
	}
	root, err := statedb.Commit(blocknum, false)
	if err != nil {
		return err
	}
	// Commit newly generated states into disk if it's not empty.
	if root != types.EmptyRootHash {
		if err := triedb.Commit(root, true); err != nil {
			return err
		}
	}
	// Marshal the genesis state specification and persist.
	blob, err := json.Marshal(ga)
	if err != nil {
		return err
	}
	rawdb.WriteGenesisStateSpec(db, blockhash, blob)
	return nil
}

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

	// Just commit the new block if there is no stored genesis block.
	stored := rawdb.ReadCanonicalHash(db, genesisNum.Uint64())
	if (stored == common.Hash{}) {
		header, err := genesis.Commit(db, triedb, genesisBlock)
		return genesis.Config, header.Hash, err
	}

	// Ensure the stored genesis matches with the given one.
	if genesisHash != stored {
		return genesis.Config, genesisHash, &core.GenesisMismatchError{stored, genesisHash}
	}

	// The genesis block is present(perhaps in ancient database) while the
	// state database is not initialized yet. It can happen that the node
	// is initialized with an external ancient store. Commit genesis state
	// in this case.
	header := miverawdb.ReadHeader(db, stored, genesisNum.Uint64())
	if header.Root != types.EmptyRootHash && !triedb.Initialized(header.Root) {
		header, err := genesis.Commit(db, triedb, genesisBlock)
		return genesis.Config, header.Hash, err
	}

	newcfg := genesis.Config
	if err := newcfg.CheckConfigForkOrder(); err != nil {
		return newcfg, common.Hash{}, err
	}
	storedcfg := miverawdb.ReadChainConfig(db, stored)
	if storedcfg == nil {
		log.Warn("Found genesis block without chain config")
		miverawdb.WriteChainConfig(db, stored, newcfg)
		return newcfg, stored, nil
	}
	storedData, _ := json.Marshal(storedcfg)

	// Check config compatibility and write the config. Compatibility errors
	// are returned to the caller unless we're already at block zero.
	head := miverawdb.ReadHeadHeader(db)
	if head == nil {
		return newcfg, stored, errors.New("missing head header")
	}
	compatErr := storedcfg.CheckCompatible(newcfg, head.Number.Uint64(), head.Time)
	if compatErr != nil && ((head.Number.Uint64() != 0 && compatErr.RewindToBlock != 0) || (head.Time != 0 && compatErr.RewindToTime != 0)) {
		return newcfg, stored, compatErr
	}

	// Don't overwrite if the old is identical to the new
	if newData, _ := json.Marshal(newcfg); !bytes.Equal(storedData, newData) {
		miverawdb.WriteChainConfig(db, stored, newcfg)
	}
	return newcfg, stored, nil
}

// IsVerkle indicates whether the state is already stored in a verkle
// tree at genesis time.
func (g *Genesis) IsVerkle(block *types.Block) bool {
	return g.Config.Eth.IsVerkle(block.Number(), block.Time())
}

// ToHeader returns the genesis block header according to genesis specification.
func (g *Genesis) ToHeader(block *types.Block) *mivetypes.MiveHeader {
	root, err := g.Alloc.hash(g.IsVerkle(block), block.NumberU64())
	if err != nil {
		panic(err)
	}
	return &mivetypes.MiveHeader{
		ParentHash:  block.ParentHash(),
		Hash:        block.Hash(),
		Number:      block.Number(),
		Time:        block.Time(),
		Root:        root,
		ReceiptHash: types.EmptyReceiptsHash,
	}
}

func (g *Genesis) Commit(db ethdb.Database, triedb *trie.Database, block *types.Block) (*mivetypes.MiveHeader, error) {
	header := g.ToHeader(block)
	config := g.Config
	if err := config.CheckConfigForkOrder(); err != nil {
		return nil, err
	}
	if config.Eth.Clique != nil && len(block.Extra()) < 32+crypto.SignatureLength {
		return nil, errors.New("can't start clique chain without signers")
	}
	// All the checks has passed, flush the states derived from the genesis
	// specification as well as the specification itself into the provided
	// database.
	if err := g.Alloc.flush(db, triedb, block.Hash(), block.NumberU64()); err != nil {
		return nil, err
	}
	miverawdb.WriteHeader(db, header)
	rawdb.WriteReceipts(db, block.Hash(), block.NumberU64(), nil)
	rawdb.WriteCanonicalHash(db, block.Hash(), block.NumberU64())
	rawdb.WriteHeadBlockHash(db, block.Hash())
	rawdb.WriteHeadFastBlockHash(db, block.Hash())
	rawdb.WriteHeadHeaderHash(db, block.Hash())
	miverawdb.WriteChainConfig(db, block.Hash(), config)
	return header, nil
}

// DefaultGenesisBlock returns the Mive genesis block for the Ethereum mainnet.
func DefaultGenesisBlock() *Genesis {
	return &Genesis{
		Config: params.MainnetChainConfig,
		Alloc:  make(GenesisAlloc),
	}
}
