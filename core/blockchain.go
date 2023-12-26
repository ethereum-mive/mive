package core

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/prque"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/state/snapshot"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethdb"
	gethparams "github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/trie/triedb/hashdb"
	"github.com/ethereum/go-ethereum/trie/triedb/pathdb"

	"github.com/ethereum-mive/mive/params"
)

// triedbConfig derives the configures for trie database.
func triedbConfig(c *core.CacheConfig) *trie.Config {
	config := &trie.Config{Preimages: c.Preimages}
	if c.StateScheme == rawdb.HashScheme {
		config.HashDB = &hashdb.Config{
			CleanCacheSize: c.TrieCleanLimit * 1024 * 1024,
		}
	}
	if c.StateScheme == rawdb.PathScheme {
		config.PathDB = &pathdb.Config{
			StateHistory:   c.StateHistory,
			CleanCacheSize: c.TrieCleanLimit * 1024 * 1024,
			DirtyCacheSize: c.TrieDirtyLimit * 1024 * 1024,
		}
	}
	return config
}

type BlockChain struct {
	chainConfig *params.ChainConfig // Chain & network configuration
	cacheConfig *core.CacheConfig   // Cache configuration for pruning

	db            ethdb.Database                   // Low level persistent database to store final content in
	snaps         *snapshot.Tree                   // Snapshot tree for fast trie leaf access
	triegc        *prque.Prque[int64, common.Hash] // Priority queue mapping block numbers to tries to gc
	gcproc        time.Duration                    // Accumulates canonical block processing for trie dumping
	lastWrite     uint64                           // Last block when the state was flushed
	flushInterval atomic.Int64                     // Time interval (processing time) after which to flush a state
	triedb        *trie.Database                   // The database handler for maintaining trie nodes.
	stateCache    state.Database                   // State database to reuse between imports (contains state cache)

	// txLookupLimit is the maximum number of blocks from head whose tx indices
	// are reserved:
	//  * 0:   means no limit and regenerate any missing indexes
	//  * N:   means N block limit [HEAD-N+1, HEAD] and delete extra indexes
	//  * nil: disable tx reindexer/deleter, but still index new blocks
	txLookupLimit uint64

	wg            sync.WaitGroup //
	quit          chan struct{}  // shutdown signal, closed in Stop.
	stopping      atomic.Bool    // false if chain is running, true when stopped
	procInterrupt atomic.Bool    // interrupt signaler for block processing

	engine     consensus.Engine
	prefetcher core.Prefetcher
	vmConfig   vm.Config

	ethClient *ethclient.Client

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func NewBlockChain(db ethdb.Database, cacheConfig *core.CacheConfig, genesis *Genesis, overrides *core.ChainOverrides, engine consensus.Engine, vmConfig vm.Config, ethClient *ethclient.Client) (*BlockChain, error) {
	// Open trie database with provided config
	triedb := trie.NewDatabase(db, triedbConfig(cacheConfig))

	ctx, ctxCancel := context.WithCancel(context.Background())

	chainConfig, genesisHash, genesisErr := SetupGenesisBlockWithOverride(ctx, db, triedb, genesis, overrides, ethClient)
	if _, ok := genesisErr.(*gethparams.ConfigCompatError); genesisErr != nil && !ok {
		return nil, genesisErr
	}
	_ = genesisHash

	bc := &BlockChain{
		chainConfig: chainConfig,
		cacheConfig: cacheConfig,
		db:          db,
		triedb:      triedb,
		engine:      engine,
		vmConfig:    vmConfig,
		ethClient:   ethClient,
		ctx:         ctx,
		ctxCancel:   ctxCancel,
	}

	bc.flushInterval.Store(int64(cacheConfig.TrieTimeLimit))
	bc.stateCache = state.NewDatabaseWithNodeDB(bc.db, bc.triedb)
	bc.prefetcher = newStatePrefetcher(chainConfig, bc, engine)

	return bc, nil
}

func (bc *BlockChain) insertChain(chain types.Blocks, setHead bool) (int, error) {
	// If the chain is terminating, don't even bother starting up.
	if bc.insertStopped() {
		return 0, nil
	}

	// Start a parallel signature recovery (signer will fluke on fork transition, minimal perf loss)
	core.SenderCacher.RecoverFromBlocks(types.MakeSigner(bc.chainConfig.Eth, chain[0].Number(), chain[0].Time()), chain)

	return 0, nil
}

// StopInsert interrupts all insertion methods, causing them to return
// errInsertionInterrupted as soon as possible. Insertion is permanently disabled after
// calling this method.
func (bc *BlockChain) StopInsert() {
	bc.procInterrupt.Store(true)
}

// insertStopped returns true after StopInsert has been called.
func (bc *BlockChain) insertStopped() bool {
	return bc.procInterrupt.Load()
}
