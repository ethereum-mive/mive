package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
)

// TransactionToMessage converts a transaction into a Message.
func TransactionToMessage(tx *types.Transaction, s types.Signer) (*core.Message, error) {
	// TODO
	msg := &core.Message{
		Nonce:             tx.Nonce(),   // TODO
		GasLimit:          tx.Gas(),     // TODO
		GasPrice:          new(big.Int), // zero
		GasFeeCap:         new(big.Int), // zero
		GasTipCap:         new(big.Int), // zero
		To:                tx.To(),
		Value:             tx.Value(),
		Data:              tx.Data(),
		AccessList:        tx.AccessList(),
		SkipAccountChecks: true, // Skip checks
		BlobHashes:        tx.BlobHashes(),
		BlobGasFeeCap:     tx.BlobGasFeeCap(),
	}
	var err error
	msg.From, err = types.Sender(s, tx)
	return msg, err
}
