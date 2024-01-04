package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	cmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	miveconsensus "github.com/ethereum-mive/mive/consensus"
	mivetypes "github.com/ethereum-mive/mive/core/types"
	"github.com/ethereum-mive/mive/params"
)

// ChainContext supports retrieving headers and consensus parameters from the
// current blockchain to be used during transaction processing.
type ChainContext interface {
	// Engine retrieves the chain's consensus engine.
	Engine() miveconsensus.Engine

	// GetHeader returns the header corresponding to the hash/number argument pair.
	GetHeader(common.Hash, uint64) *mivetypes.Header
}

// NewEVMBlockContext creates a new context for use in the EVM.
func NewEVMBlockContext(header *types.Header, chain *BlockChain, author *common.Address, config *params.ChainConfig) vm.BlockContext {
	// Set coinbase to beneficiary address.
	if author == nil {
		author = &params.BeneficiaryAddress
	}

	ctx := core.NewEVMBlockContext(header, &BlockChainWrapper{chain}, author)

	// Overwrite GetHash to retrieve Mive header hashes.
	ctx.GetHash = GetHashFn(header, chain)

	feeReductionDenom := new(big.Int).SetUint64(config.FeeReductionDenominator())
	if ctx.BaseFee != nil {
		ctx.BaseFee = new(big.Int).Div(ctx.BaseFee, feeReductionDenom)
	}
	if ctx.BlobBaseFee != nil {
		ctx.BlobBaseFee = new(big.Int).Div(ctx.BlobBaseFee, feeReductionDenom)
	}

	ctx.GasLimit = blockGasLimit(ctx.GasLimit, config)

	return ctx
}

// GetHashFn returns a GetHashFunc which retrieves header hashes by number
func GetHashFn(ref *types.Header, chain ChainContext) func(n uint64) common.Hash {
	// Cache will initially contain [refHash.parent],
	// Then fill up with [refHash.p, refHash.pp, refHash.ppp, ...]
	var cache []common.Hash

	return func(n uint64) common.Hash {
		if ref.Number.Uint64() <= n {
			// This situation can happen if we're doing tracing and using
			// block overrides.
			return common.Hash{}
		}
		// If there's no hash cache yet, make one
		if len(cache) == 0 {
			cache = append(cache, ref.ParentHash)
		}
		if idx := ref.Number.Uint64() - n - 1; idx < uint64(len(cache)) {
			return cache[idx]
		}
		// No luck in the cache, but we can start iterating from the last element we already know
		lastKnownHash := cache[len(cache)-1]
		lastKnownNumber := ref.Number.Uint64() - uint64(len(cache))

		for {
			header := chain.GetHeader(lastKnownHash, lastKnownNumber)
			if header == nil {
				break
			}
			cache = append(cache, header.ParentHash)
			lastKnownHash = header.ParentHash
			lastKnownNumber = header.Number.Uint64() - 1
			if n == lastKnownNumber {
				return lastKnownHash
			}
		}
		return common.Hash{}
	}
}

func blockGasLimit(gasLimit uint64, config *params.ChainConfig) uint64 {
	gasLimit, overflow := cmath.SafeMul(gasLimit, config.BlockGasLimitMultiplier())
	if overflow {
		gasLimit = cmath.MaxUint64
	}
	if gasLimit < config.MinBlockGasLimit() {
		gasLimit = config.MinBlockGasLimit()
	}
	return gasLimit
}
