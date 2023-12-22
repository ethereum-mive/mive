package core

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethdb"

	"github.com/ethereum-mive/mive/params"
)

type BlockChain struct {
	chainConfig *params.ChainConfig // Chain & network configuration

	wg            sync.WaitGroup //
	quit          chan struct{}  // shutdown signal, closed in Stop.
	stopping      atomic.Bool    // false if chain is running, true when stopped
	procInterrupt atomic.Bool    // interrupt signaler for block processing

	engine   consensus.Engine
	vmConfig vm.Config

	ethClient *ethclient.Client

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func NewBlockChain(db ethdb.Database, engine consensus.Engine, vmConfig vm.Config, etcClient *ethclient.Client) (*BlockChain, error) {
	ctx, ctxCancel := context.WithCancel(context.Background())

	bc := &BlockChain{
		engine:    engine,
		vmConfig:  vmConfig,
		ethClient: etcClient,
		ctx:       ctx,
		ctxCancel: ctxCancel,
	}

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
