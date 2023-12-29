package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

//go:generate go run github.com/fjl/gencodec -type MiveHeader -field-override headerMarshaling -out gen_header_json.go
//go:generate go run github.com/ethereum/go-ethereum/rlp/rlpgen -type MiveHeader -out gen_header_rlp.go

// MiveHeader represents a block header in Mive.
type MiveHeader struct {
	ParentHash common.Hash `json:"parentHash" gencodec:"required"`
	Hash       common.Hash `json:"hash"       gencodec:"required"`
	Number     *big.Int    `json:"number"     gencodec:"required"`
	Time       uint64      `json:"timestamp"  gencodec:"required"`

	Root        common.Hash `json:"stateRoot"    gencodec:"required"`
	ReceiptHash common.Hash `json:"receiptsRoot" gencodec:"required"`
	Bloom       types.Bloom `json:"logsBloom"    gencodec:"required"`
	GasUsed     uint64      `json:"gasUsed"      gencodec:"required"`
}

// field type overrides for gencodec
type headerMarshaling struct {
	Number  *hexutil.Big
	GasUsed hexutil.Uint64
}
