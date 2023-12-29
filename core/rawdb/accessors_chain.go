package rawdb

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"

	mivetypes "github.com/ethereum-mive/mive/core/types"
)

// ReadHeader retrieves the block header corresponding to the hash.
func ReadHeader(db ethdb.Reader, hash common.Hash, number uint64) *mivetypes.MiveHeader {
	data := rawdb.ReadHeaderRLP(db, hash, number)
	if len(data) == 0 {
		return nil
	}
	header := new(mivetypes.MiveHeader)
	if err := rlp.DecodeBytes(data, header); err != nil {
		log.Error("Invalid block header RLP", "hash", hash, "err", err)
		return nil
	}
	return header
}

// WriteHeader stores a block header into the database and also stores the hash-
// to-number mapping.
func WriteHeader(db ethdb.KeyValueWriter, header *mivetypes.MiveHeader) {
	var (
		hash   = header.Hash
		number = header.Number.Uint64()
	)
	// Write the hash -> number mapping
	rawdb.WriteHeaderNumber(db, hash, number)

	// Write the encoded header
	data, err := rlp.EncodeToBytes(header)
	if err != nil {
		log.Crit("Failed to RLP encode header", "err", err)
	}
	key := rawdb.HeaderKey(number, hash)
	if err := db.Put(key, data); err != nil {
		log.Crit("Failed to store header", "err", err)
	}
}

// ReadHeadHeader returns the current canonical head header.
func ReadHeadHeader(db ethdb.Reader) *mivetypes.MiveHeader {
	headHeaderHash := rawdb.ReadHeadHeaderHash(db)
	if headHeaderHash == (common.Hash{}) {
		return nil
	}
	headHeaderNumber := rawdb.ReadHeaderNumber(db, headHeaderHash)
	if headHeaderNumber == nil {
		return nil
	}
	return ReadHeader(db, headHeaderHash, *headHeaderNumber)
}
