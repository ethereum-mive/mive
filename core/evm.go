package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	cmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/ethereum-mive/mive/params"
)

// NewEVMBlockContext creates a new context for use in the EVM.
func NewEVMBlockContext(header *types.Header, chain core.ChainContext, author *common.Address, config *params.ChainConfig) vm.BlockContext {
	ctx := core.NewEVMBlockContext(header, chain, author)

	ctx.Coinbase = config.Mive.BeneficiaryAddress

	feeReductionDenom := new(big.Int).SetUint64(config.FeeReductionDenominator())
	if ctx.BaseFee != nil {
		ctx.BaseFee = new(big.Int).Div(ctx.BaseFee, feeReductionDenom)
	}
	if ctx.BlobBaseFee != nil {
		ctx.BlobBaseFee = new(big.Int).Div(ctx.BlobBaseFee, feeReductionDenom)
	}

	gasLimit, overflow := cmath.SafeMul(ctx.GasLimit, config.BlockGasLimitMultiplier())
	if overflow {
		gasLimit = cmath.MaxUint64
	}
	if gasLimit < config.MinBlockGasLimit() {
		gasLimit = config.MinBlockGasLimit()
	}
	ctx.GasLimit = gasLimit

	return ctx
}
