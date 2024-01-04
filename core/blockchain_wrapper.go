package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
)

type BlockChainWrapper struct {
	*BlockChain
}

func (bc *BlockChainWrapper) Engine() consensus.Engine {
	return nil
}

func (bc *BlockChainWrapper) GetHeader(hash common.Hash, number uint64) *types.Header {
	return bc.EthGetHeader(hash, number)
}
