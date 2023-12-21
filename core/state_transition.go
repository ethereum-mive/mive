package core

import (
	"math/big"

	cmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/ethereum-mive/mive/core/types"
	"github.com/ethereum-mive/mive/params"
)

// TransactionToMessage converts a transaction into a Message.
func TransactionToMessage(tx *gethtypes.Transaction, s gethtypes.Signer, baseFee *big.Int, config *params.ChainConfig) (*core.Message, error) {
	if tx.To() == nil || *tx.To() != config.Mive.BeaconAddress {
		// The transaction is not sent to the beacon address.
		return nil, nil
	}
	if tx.Type() == gethtypes.BlobTxType {
		// We don't support blob transaction type.
		return nil, nil
	}
	if len(tx.Data()) == 0 {
		return nil, nil
	}

	// Decode Mive transaction from the data payload of the original Ethereum transaction.
	var mtx types.MiveTx
	err := rlp.DecodeBytes(tx.Data(), &mtx)
	if err != nil {
		log.Warn("Decode Mive transaction", "hash", tx.Hash(), "err", err)
		// Skip it if it's not a valid Mive transaction.
		return nil, nil
	}

	feeReductionDenom := new(big.Int).SetUint64(config.FeeReductionDenominator())

	msg := &core.Message{
		Nonce:             tx.Nonce(), // Note: the nonce won't be checked while handling message
		GasLimit:          mtx.Gas,
		GasPrice:          new(big.Int).Div(tx.GasPrice(), feeReductionDenom),
		GasFeeCap:         new(big.Int).Div(tx.GasFeeCap(), feeReductionDenom),
		GasTipCap:         new(big.Int).Div(tx.GasTipCap(), feeReductionDenom),
		To:                mtx.To,
		Value:             mtx.Value,
		Data:              mtx.Data,
		AccessList:        mtx.AccessList,
		SkipAccountChecks: true, // Skip checks
		BlobHashes:        nil,
		BlobGasFeeCap:     nil,
	}
	// If baseFee provided, set gasPrice to effectiveGasPrice.
	if baseFee != nil {
		reductedBaseFee := new(big.Int).Div(baseFee, feeReductionDenom)
		msg.GasPrice = cmath.BigMin(msg.GasPrice.Add(msg.GasTipCap, reductedBaseFee), msg.GasFeeCap)
	}
	msg.From, err = gethtypes.Sender(s, tx)
	return msg, err
}
