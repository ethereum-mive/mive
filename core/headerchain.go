package core

import (
	crand "crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	mrand "math/rand"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/lru"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"

	miveconsensus "github.com/ethereum-mive/mive/consensus"
	miverawdb "github.com/ethereum-mive/mive/core/rawdb"
	mivetypes "github.com/ethereum-mive/mive/core/types"
	"github.com/ethereum-mive/mive/params"
)

const (
	headerCacheLimit = 512
	numberCacheLimit = 2048
)

// HeaderChain implements the basic block header chain logic that is used by
// BlockChain.
//
// HeaderChain is responsible for maintaining the header chain including the
// header query and updating.
//
// The components maintained by headerchain includes: (1) header
// (2) block hash -> number mapping (3) canonical number -> hash mapping
// and (4) head header flag.
//
// It is not thread safe either, the encapsulating chain structures should do
// the necessary mutex locking/unlocking.
type HeaderChain struct {
	config        *params.ChainConfig
	chainDb       ethdb.Database
	genesisHeader *mivetypes.Header

	currentHeader atomic.Value // Current head of the header chain (may be above the block chain!)

	headerCache *lru.Cache[common.Hash, *mivetypes.Header]
	numberCache *lru.Cache[common.Hash, uint64] // most recent block numbers

	procInterrupt func() bool

	rand   *mrand.Rand
	engine miveconsensus.Engine
}

// NewHeaderChain creates a new HeaderChain structure. ProcInterrupt points
// to the parent's interrupt semaphore.
func NewHeaderChain(chainDb ethdb.Database, config *params.ChainConfig, engine miveconsensus.Engine, procInterrupt func() bool) (*HeaderChain, error) {
	// Seed a fast but crypto originating random generator
	seed, err := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return nil, err
	}
	hc := &HeaderChain{
		config:        config,
		chainDb:       chainDb,
		headerCache:   lru.NewCache[common.Hash, *mivetypes.Header](headerCacheLimit),
		numberCache:   lru.NewCache[common.Hash, uint64](numberCacheLimit),
		procInterrupt: procInterrupt,
		rand:          mrand.New(mrand.NewSource(seed.Int64())),
		engine:        engine,
	}
	hc.genesisHeader = hc.GetHeaderByNumber(config.Mive.GenesisBlock.Uint64())
	if hc.genesisHeader == nil {
		return nil, core.ErrNoGenesis
	}
	hc.currentHeader.Store(hc.genesisHeader)
	if head := rawdb.ReadHeadBlockHash(chainDb); head != (common.Hash{}) {
		if chead := hc.GetHeaderByHash(head); chead != nil {
			hc.currentHeader.Store(chead)
		}
	}
	headHeaderGauge.Update(hc.CurrentHeader().Number.Int64())
	return hc, nil
}

// GetBlockNumber retrieves the block number belonging to the given hash
// from the cache or database
func (hc *HeaderChain) GetBlockNumber(hash common.Hash) *uint64 {
	if cached, ok := hc.numberCache.Get(hash); ok {
		return &cached
	}
	number := rawdb.ReadHeaderNumber(hc.chainDb, hash)
	if number != nil {
		hc.numberCache.Add(hash, *number)
	}
	return number
}

type headerWriteResult struct {
	status     core.WriteStatus
	ignored    int
	imported   int
	lastHash   common.Hash
	lastHeader *mivetypes.Header
}

// Reorg reorgs the local canonical chain into the specified chain. The reorg
// can be classified into two cases: (a) extend the local chain (b) switch the
// head to the given header.
func (hc *HeaderChain) Reorg(headers []*mivetypes.Header) error {
	// Short circuit if nothing to reorg.
	if len(headers) == 0 {
		return nil
	}
	// If the parent of the (first) block is already the canon header,
	// we don't have to go backwards to delete canon blocks, but simply
	// pile them onto the existing chain. Otherwise, do the necessary
	// reorgs.
	var (
		first = headers[0]
		last  = headers[len(headers)-1]
		batch = hc.chainDb.NewBatch()
	)
	if first.ParentHash != hc.CurrentHeader().Hash {
		// Delete any canonical number assignments above the new head
		for i := last.Number.Uint64() + 1; ; i++ {
			hash := rawdb.ReadCanonicalHash(hc.chainDb, i)
			if hash == (common.Hash{}) {
				break
			}
			rawdb.DeleteCanonicalHash(batch, i)
		}
		// Overwrite any stale canonical number assignments, going
		// backwards from the first header in this import until the
		// cross link between two chains.
		var (
			header     = first
			headNumber = header.Number.Uint64()
			headHash   = header.Hash
		)
		for rawdb.ReadCanonicalHash(hc.chainDb, headNumber) != headHash {
			rawdb.WriteCanonicalHash(batch, headHash, headNumber)
			if headNumber == 0 {
				break // It shouldn't be reached
			}
			headHash, headNumber = header.ParentHash, header.Number.Uint64()-1
			header = hc.GetHeader(headHash, headNumber)
			if header == nil {
				return fmt.Errorf("missing parent %d %x", headNumber, headHash)
			}
		}
	}
	// Extend the canonical chain with the new headers
	for i := 0; i < len(headers); i++ {
		hash := headers[i].Hash
		num := headers[i].Number.Uint64()
		rawdb.WriteCanonicalHash(batch, hash, num)
		rawdb.WriteHeadHeaderHash(batch, hash)
	}

	if err := batch.Write(); err != nil {
		return err
	}
	// Last step update all in-memory head header markers
	hc.SetCurrentHeader(mivetypes.CopyHeader(last))
	return nil
}

// WriteHeaders writes a chain of headers into the local chain, given that the
// parents are already known. The chain head header won't be updated in this
// function, the additional SetCanonical is expected in order to finish the entire
// procedure.
func (hc *HeaderChain) WriteHeaders(headers []*mivetypes.Header) (int, error) {
	if len(headers) == 0 {
		return 0, nil
	}
	p := hc.GetHeader(headers[0].ParentHash, headers[0].Number.Uint64()-1)
	if p == nil {
		return 0, consensus.ErrUnknownAncestor
	}
	var (
		inserted    []rawdb.NumberHash // Ephemeral lookup of number/hash for the chain
		parentKnown = true             // Set to true to force hc.HasHeader check the first iteration
		batch       = hc.chainDb.NewBatch()
	)
	for _, header := range headers {
		hash := header.Hash
		number := header.Number.Uint64()

		// If the parent was not present, store it
		// If the header is already known, skip it, otherwise store
		alreadyKnown := parentKnown && hc.HasHeader(hash, number)
		if !alreadyKnown {
			// Irrelevant of the canonical status, write header to the database.
			miverawdb.WriteHeader(batch, header)

			inserted = append(inserted, rawdb.NumberHash{Number: number, Hash: hash})
			hc.headerCache.Add(hash, header)
			hc.numberCache.Add(hash, number)
		}
		parentKnown = alreadyKnown
	}
	// Skip the slow disk write of all headers if interrupted.
	if hc.procInterrupt() {
		log.Debug("Premature abort during headers import")
		return 0, errors.New("aborted")
	}
	// Commit to disk!
	if err := batch.Write(); err != nil {
		log.Crit("Failed to write headers", "error", err)
	}
	return len(inserted), nil
}

// writeHeadersAndSetHead writes a batch of block headers and applies the last
// header as the chain head if the fork choicer says it's ok to update the chain.
// Note: This method is not concurrent-safe with inserting blocks simultaneously
// into the chain, as side effects caused by reorganisations cannot be emulated
// without the real blocks. Hence, writing headers directly should only be done
// in two scenarios: pure-header mode of operation (light clients), or properly
// separated header/block phases (non-archive clients).
func (hc *HeaderChain) writeHeadersAndSetHead(headers []*mivetypes.Header) (*headerWriteResult, error) {
	inserted, err := hc.WriteHeaders(headers)
	if err != nil {
		return nil, err
	}
	var (
		lastHeader = headers[len(headers)-1]
		lastHash   = headers[len(headers)-1].Hash
		result     = &headerWriteResult{
			status:     core.NonStatTy,
			ignored:    len(headers) - inserted,
			imported:   inserted,
			lastHash:   lastHash,
			lastHeader: lastHeader,
		}
	)
	// Special case, all the inserted headers are already on the canonical
	// header chain, skip the reorg operation.
	if hc.GetCanonicalHash(lastHeader.Number.Uint64()) == lastHash && lastHeader.Number.Uint64() <= hc.CurrentHeader().Number.Uint64() {
		return result, nil
	}
	// Apply the reorg operation
	if err := hc.Reorg(headers); err != nil {
		return nil, err
	}
	result.status = core.CanonStatTy
	return result, nil
}

func (hc *HeaderChain) ValidateHeaderChain(chain []*mivetypes.Header) (int, error) {
	// Do a sanity check that the provided chain is actually ordered and linked
	for i := 1; i < len(chain); i++ {
		if chain[i].Number.Uint64() != chain[i-1].Number.Uint64()+1 {
			hash := chain[i].Hash
			parentHash := chain[i-1].Hash
			// Chain broke ancestry, log a message (programming error) and skip insertion
			log.Error("Non contiguous header insert", "number", chain[i].Number, "hash", hash,
				"parent", chain[i].ParentHash, "prevnumber", chain[i-1].Number, "prevhash", parentHash)

			return 0, fmt.Errorf("non contiguous insert: item %d is #%d [%x..], item %d is #%d [%x..] (parent [%x..])", i-1, chain[i-1].Number,
				parentHash.Bytes()[:4], i, chain[i].Number, hash.Bytes()[:4], chain[i].ParentHash[:4])
		}
		// If the header is a banned one, straight out abort
		if core.BadHashes[chain[i].ParentHash] {
			return i - 1, core.ErrBannedHash
		}
		// If it's the last header in the cunk, we need to check it too
		if i == len(chain)-1 && core.BadHashes[chain[i].Hash] {
			return i, core.ErrBannedHash
		}
	}
	// Start the parallel verifier
	abort, results := hc.engine.VerifyHeaders(hc, chain)
	defer close(abort)

	// Iterate over the headers and ensure they all check out
	for i := range chain {
		// If the chain is terminating, stop processing blocks
		if hc.procInterrupt() {
			log.Debug("Premature abort during headers verification")
			return 0, errors.New("aborted")
		}
		// Otherwise wait for headers checks and ensure they pass
		if err := <-results; err != nil {
			return i, err
		}
	}

	return 0, nil
}

// InsertHeaderChain inserts the given headers and does the reorganisations.
//
// The validity of the headers is NOT CHECKED by this method, i.e. they need to be
// validated by ValidateHeaderChain before calling InsertHeaderChain.
//
// This insert is all-or-nothing. If this returns an error, no headers were written,
// otherwise they were all processed successfully.
//
// The returned 'write status' says if the inserted headers are part of the canonical chain
// or a side chain.
func (hc *HeaderChain) InsertHeaderChain(chain []*mivetypes.Header, start time.Time) (core.WriteStatus, error) {
	if hc.procInterrupt() {
		return 0, errors.New("aborted")
	}
	res, err := hc.writeHeadersAndSetHead(chain)
	if err != nil {
		return 0, err
	}
	// Report some public statistics so the user has a clue what's going on
	context := []interface{}{
		"count", res.imported,
		"elapsed", common.PrettyDuration(time.Since(start)),
	}
	if last := res.lastHeader; last != nil {
		context = append(context, "number", last.Number, "hash", res.lastHash)
		if timestamp := time.Unix(int64(last.Time), 0); time.Since(timestamp) > time.Minute {
			context = append(context, []interface{}{"age", common.PrettyAge(timestamp)}...)
		}
	}
	if res.ignored > 0 {
		context = append(context, []interface{}{"ignored", res.ignored}...)
	}
	log.Debug("Imported new block headers", context...)
	return res.status, err
}

// GetAncestor retrieves the Nth ancestor of a given block. It assumes that either the given block or
// a close ancestor of it is canonical. maxNonCanonical points to a downwards counter limiting the
// number of blocks to be individually checked before we reach the canonical chain.
//
// Note: ancestor == 0 returns the same block, 1 returns its parent and so on.
func (hc *HeaderChain) GetAncestor(hash common.Hash, number, ancestor uint64, maxNonCanonical *uint64) (common.Hash, uint64) {
	if ancestor > number {
		return common.Hash{}, 0
	}
	if ancestor == 1 {
		// in this case it is cheaper to just read the header
		if header := hc.GetHeader(hash, number); header != nil {
			return header.ParentHash, number - 1
		}
		return common.Hash{}, 0
	}
	for ancestor != 0 {
		if rawdb.ReadCanonicalHash(hc.chainDb, number) == hash {
			ancestorHash := rawdb.ReadCanonicalHash(hc.chainDb, number-ancestor)
			if rawdb.ReadCanonicalHash(hc.chainDb, number) == hash {
				number -= ancestor
				return ancestorHash, number
			}
		}
		if *maxNonCanonical == 0 {
			return common.Hash{}, 0
		}
		*maxNonCanonical--
		ancestor--
		header := hc.GetHeader(hash, number)
		if header == nil {
			return common.Hash{}, 0
		}
		hash = header.ParentHash
		number--
	}
	return hash, number
}

// GetHeader retrieves a block header from the database by hash and number,
// caching it if found.
func (hc *HeaderChain) GetHeader(hash common.Hash, number uint64) *mivetypes.Header {
	// Short circuit if the header's already in the cache, retrieve otherwise
	if header, ok := hc.headerCache.Get(hash); ok {
		return header
	}
	header := miverawdb.ReadHeader(hc.chainDb, hash, number)
	if header == nil {
		return nil
	}
	// Cache the found header for next time and return
	hc.headerCache.Add(hash, header)
	return header
}

// GetHeaderByHash retrieves a block header from the database by hash, caching it if
// found.
func (hc *HeaderChain) GetHeaderByHash(hash common.Hash) *mivetypes.Header {
	number := hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	return hc.GetHeader(hash, *number)
}

// HasHeader checks if a block header is present in the database or not.
// In theory, if header is present in the database, all relative components
// like td and hash->number should be present too.
func (hc *HeaderChain) HasHeader(hash common.Hash, number uint64) bool {
	if hc.numberCache.Contains(hash) || hc.headerCache.Contains(hash) {
		return true
	}
	return rawdb.HasHeader(hc.chainDb, hash, number)
}

// GetHeaderByNumber retrieves a block header from the database by number,
// caching it (associated with its hash) if found.
func (hc *HeaderChain) GetHeaderByNumber(number uint64) *mivetypes.Header {
	hash := rawdb.ReadCanonicalHash(hc.chainDb, number)
	if hash == (common.Hash{}) {
		return nil
	}
	return hc.GetHeader(hash, number)
}

// GetHeadersFrom returns a contiguous segment of headers, in rlp-form, going
// backwards from the given number.
// If the 'number' is higher than the highest local header, this method will
// return a best-effort response, containing the headers that we do have.
func (hc *HeaderChain) GetHeadersFrom(number, count uint64) []rlp.RawValue {
	// If the request is for future headers, we still return the portion of
	// headers that we are able to serve
	if current := hc.CurrentHeader().Number.Uint64(); current < number {
		if count > number-current {
			count -= number - current
			number = current
		} else {
			return nil
		}
	}
	var headers []rlp.RawValue
	// If we have some of the headers in cache already, use that before going to db.
	hash := rawdb.ReadCanonicalHash(hc.chainDb, number)
	if hash == (common.Hash{}) {
		return nil
	}
	for count > 0 {
		header, ok := hc.headerCache.Get(hash)
		if !ok {
			break
		}
		rlpData, _ := rlp.EncodeToBytes(header)
		headers = append(headers, rlpData)
		hash = header.ParentHash
		count--
		number--
	}
	// Read remaining from db
	if count > 0 {
		headers = append(headers, rawdb.ReadHeaderRange(hc.chainDb, number, count)...)
	}
	return headers
}

func (hc *HeaderChain) GetCanonicalHash(number uint64) common.Hash {
	return rawdb.ReadCanonicalHash(hc.chainDb, number)
}

// CurrentHeader retrieves the current head header of the canonical chain. The
// header is retrieved from the HeaderChain's internal cache.
func (hc *HeaderChain) CurrentHeader() *mivetypes.Header {
	return hc.currentHeader.Load().(*mivetypes.Header)
}

// SetCurrentHeader sets the in-memory head header marker of the canonical chan
// as the given header.
func (hc *HeaderChain) SetCurrentHeader(head *mivetypes.Header) {
	hc.currentHeader.Store(head)
	headHeaderGauge.Update(head.Number.Int64())
}

// Config retrieves the header chain's chain configuration.
func (hc *HeaderChain) Config() *params.ChainConfig { return hc.config }
