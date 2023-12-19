package mive

import (
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state/pruner"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-mive/mive/internal/shutdowncheck"
	"github.com/ethereum-mive/mive/mive/miveconfig"
	"github.com/ethereum-mive/mive/node"
)

// Mive implements the Mive indexer and execution layer service.
type Mive struct {
	config *miveconfig.Config

	ethClient *ethclient.Client

	// DB interfaces
	chainDb ethdb.Database // Block chain database

	shutdownTracker *shutdowncheck.ShutdownTracker // Tracks if and when the node has shutdown ungracefully
}

func New(stack *node.Node, config *miveconfig.Config) (*Mive, error) {
	ethClient, err := ethclient.Dial(config.EthRpcUrl)
	if err != nil {
		return nil, err
	}

	chainDb, err := stack.OpenDatabaseWithFreezer(
		"chaindata",
		config.DatabaseCache,
		config.DatabaseHandles,
		config.DatabaseFreezer,
		"eth/db/chaindata/",
		false,
	)
	if err != nil {
		return nil, err
	}
	scheme, err := rawdb.ParseStateScheme(config.StateScheme, chainDb)
	if err != nil {
		return nil, err
	}
	// Try to recover offline state pruning only in hash-based.
	if scheme == rawdb.HashScheme {
		if err := pruner.RecoverPruning(stack.ResolvePath(""), chainDb); err != nil {
			log.Error("Failed to recover state", "error", err)
		}
	}

	mive := &Mive{
		config:          config,
		ethClient:       ethClient,
		chainDb:         chainDb,
		shutdownTracker: shutdowncheck.NewShutdownTracker(chainDb),
	}

	var (
		vmConfig = vm.Config{
			EnablePreimageRecording: config.EnablePreimageRecording,
		}
		_ = vmConfig
	)

	stack.RegisterLifecycle(mive)

	// Successful startup; push a marker and check previous unclean shutdowns.
	mive.shutdownTracker.MarkStartup()

	return mive, nil
}

// Start implements node.Lifecycle, starting all internal goroutines needed by the
// Mive protocol implementation.
func (s *Mive) Start() error {
	// Regularly update shutdown marker
	s.shutdownTracker.Start()

	return nil
}

// Stop implements node.Lifecycle, terminating all internal goroutines used by the
// Mive protocol.
func (s *Mive) Stop() error {
	return nil
}
