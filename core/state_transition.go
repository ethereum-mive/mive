package core

import (
	"math/big"

	cmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
)

// TransactionToMessage converts a transaction into a Message.
func TransactionToMessage(tx *types.Transaction, s types.Signer, baseFee *big.Int) (*core.Message, error) {
	// TODO
	feeReductionFactor := FeeReductionFactor()
	msg := &core.Message{
		Nonce:             tx.Nonce(), // Note: the nonce won't be checked while handling message
		GasLimit:          tx.Gas(),   // TODO
		GasPrice:          new(big.Int).Div(tx.GasPrice(), feeReductionFactor),
		GasFeeCap:         new(big.Int).Div(tx.GasFeeCap(), feeReductionFactor),
		GasTipCap:         new(big.Int).Div(tx.GasTipCap(), feeReductionFactor),
		To:                tx.To(),
		Value:             tx.Value(),
		Data:              tx.Data(),
		AccessList:        tx.AccessList(),
		SkipAccountChecks: true, // Skip checks
		BlobHashes:        tx.BlobHashes(),
		BlobGasFeeCap:     new(big.Int).Div(tx.BlobGasFeeCap(), feeReductionFactor),
	}
	// If baseFee provided, set gasPrice to effectiveGasPrice.
	if baseFee != nil {
		msg.GasPrice = cmath.BigMin(msg.GasPrice.Add(msg.GasTipCap, baseFee), msg.GasFeeCap)
	}
	var err error
	msg.From, err = types.Sender(s, tx)
	return msg, err
}

func FeeReductionFactor() *big.Int {
	return big.NewInt(20)
}
