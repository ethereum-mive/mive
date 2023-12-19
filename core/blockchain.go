package core

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

type BlockChain struct {
	ctx       context.Context
	ethClient *ethclient.Client

	engine   consensus.Engine
	vmConfig vm.Config
}

func (bc *BlockChain) Config() *params.ChainConfig {
	// TODO
	return &params.ChainConfig{}
}

func (bc *BlockChain) CurrentHeader() *types.Header {
	header, err := bc.ethClient.HeaderByNumber(bc.ctx, nil)
	if err != nil {
		log.Error("Get current block header", "err", err)
		return nil
	}
	return header
}

// GetHeader retrieves a block header from the database by hash and number.
func (bc *BlockChain) GetHeader(hash common.Hash, number uint64) *types.Header {
	header, err := bc.ethClient.HeaderByHash(bc.ctx, hash)
	if err != nil {
		log.Error("Get block header", "hash", hash, "err", err)
		return nil
	}
	if header.Number.Cmp(new(big.Int).SetUint64(number)) != 0 {
		log.Error("Get block header", "hash", hash, "err", consensus.ErrInvalidNumber)
		return nil
	}
	return header
}

func (bc *BlockChain) GetHeaderByNumber(number uint64) *types.Header {
	header, err := bc.ethClient.HeaderByNumber(bc.ctx, new(big.Int).SetUint64(number))
	if err != nil {
		log.Error("Get block header", "number", number, "err", err)
		return nil
	}
	return header
}

func (bc *BlockChain) GetHeaderByHash(hash common.Hash) *types.Header {
	header, err := bc.ethClient.HeaderByHash(bc.ctx, hash)
	if err != nil {
		log.Error("Get block header", "hash", hash, "err", err)
		return nil
	}
	return header
}

func (bc *BlockChain) GetTd(hash common.Hash, number uint64) *big.Int {
	// TODO
	return new(big.Int)
}

// Engine retrieves the blockchain's consensus engine.
func (bc *BlockChain) Engine() consensus.Engine { return bc.engine }
