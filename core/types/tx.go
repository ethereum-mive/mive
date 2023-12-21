package types

import (
	"io"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

// MiveTx represents a Mive transaction.
type MiveTx struct {
	Gas        uint64           // gas limit
	To         *common.Address  `rlp:"nil"` // nil means contract creation
	Value      *big.Int         // wei amount
	Data       []byte           // contract invocation input data
	AccessList types.AccessList // EIP-2930 access list
}

// EncodeRLP implements rlp.Encoder
func (tx *MiveTx) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, tx)
}

// DecodeRLP implements rlp.Decoder
func (tx *MiveTx) DecodeRLP(s *rlp.Stream) error {
	return s.Decode(tx)
}
