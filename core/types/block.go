package types

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// MiveHeader represents a block header in Mive.
type MiveHeader struct {
	Hash        common.Hash `json:"hash"       gencodec:"required"`
	Root        common.Hash `json:"stateRoot"        gencodec:"required"`
	ReceiptHash common.Hash `json:"receiptsRoot"     gencodec:"required"`
	Bloom       types.Bloom `json:"logsBloom"        gencodec:"required"`
	GasUsed     uint64      `json:"gasUsed"          gencodec:"required"`
}
