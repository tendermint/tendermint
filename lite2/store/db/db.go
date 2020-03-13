package db

import (
	"encoding/binary"
	"fmt"
	"regexp"
	"strconv"
	"sync"

	"github.com/pkg/errors"
	"github.com/tendermint/go-amino"
	dbm "github.com/tendermint/tm-db"

	cryptoAmino "github.com/tendermint/tendermint/crypto/encoding/amino"
	"github.com/tendermint/tendermint/lite2/store"
	"github.com/tendermint/tendermint/types"
)

var (
	sizeKey = []byte("size")
)

type dbs struct {
	db     dbm.DB
	prefix string

	mtx  sync.RWMutex
	size uint16

	cdc *amino.Codec
}

// New returns a Store that wraps any DB (with an optional prefix in case you
// want to use one DB with many light clients).
//
// Objects are marshalled using amino (github.com/tendermint/go-amino)
func New(db dbm.DB, prefix string) store.Store {
	cdc := amino.NewCodec()
	cryptoAmino.RegisterAmino(cdc)

	size := uint16(0)
	bz, err := db.Get(sizeKey)
	if err == nil && len(bz) > 0 {
		size = unmarshalSize(bz)
	}

	return &dbs{db: db, prefix: prefix, cdc: cdc, size: size}
}

// SaveSignedHeaderAndValidatorSet persists SignedHeader and ValidatorSet to
// the db.
//
// Safe for concurrent use by multiple goroutines.
func (s *dbs) SaveSignedHeaderAndValidatorSet(sh *types.SignedHeader, valSet *types.ValidatorSet) error {
	if sh.Height <= 0 {
		panic("negative or zero height")
	}

	shBz, err := s.cdc.MarshalBinaryLengthPrefixed(sh)
	if err != nil {
		return errors.Wrap(err, "marshalling header")
	}

	valSetBz, err := s.cdc.MarshalBinaryLengthPrefixed(valSet)
	if err != nil {
		return errors.Wrap(err, "marshalling validator set")
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	b := s.db.NewBatch()
	b.Set(s.shKey(sh.Height), shBz)
	b.Set(s.vsKey(sh.Height), valSetBz)
	b.Set(sizeKey, marshalSize(s.size+1))

	err = b.WriteSync()
	b.Close()

	if err == nil {
		s.size++
	}

	return err
}

// DeleteSignedHeaderAndValidatorSet deletes SignedHeader and ValidatorSet from
// the db.
//
// Safe for concurrent use by multiple goroutines.
func (s *dbs) DeleteSignedHeaderAndValidatorSet(height int64) error {
	if height <= 0 {
		panic("negative or zero height")
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	b := s.db.NewBatch()
	b.Delete(s.shKey(height))
	b.Delete(s.vsKey(height))
	b.Set(sizeKey, marshalSize(s.size-1))

	err := b.WriteSync()
	b.Close()

	if err == nil {
		s.size--
	}

	return err
}

// SignedHeader loads SignedHeader at the given height.
//
// Safe for concurrent use by multiple goroutines.
func (s *dbs) SignedHeader(height int64) (*types.SignedHeader, error) {
	if height <= 0 {
		panic("negative or zero height")
	}

	bz, err := s.db.Get(s.shKey(height))
	if err != nil {
		panic(err)
	}
	if len(bz) == 0 {
		return nil, store.ErrSignedHeaderNotFound
	}

	var signedHeader *types.SignedHeader
	err = s.cdc.UnmarshalBinaryLengthPrefixed(bz, &signedHeader)
	return signedHeader, err
}

// ValidatorSet loads ValidatorSet at the given height.
//
// Safe for concurrent use by multiple goroutines.
func (s *dbs) ValidatorSet(height int64) (*types.ValidatorSet, error) {
	if height <= 0 {
		panic("negative or zero height")
	}

	bz, err := s.db.Get(s.vsKey(height))
	if err != nil {
		panic(err)
	}
	if len(bz) == 0 {
		return nil, store.ErrValidatorSetNotFound
	}

	var valSet *types.ValidatorSet
	err = s.cdc.UnmarshalBinaryLengthPrefixed(bz, &valSet)
	return valSet, err
}

// LastSignedHeaderHeight returns the last SignedHeader height stored.
//
// Safe for concurrent use by multiple goroutines.
func (s *dbs) LastSignedHeaderHeight() (int64, error) {
	itr, err := s.db.ReverseIterator(
		s.shKey(1),
		append(s.shKey(1<<63-1), byte(0x00)),
	)
	if err != nil {
		panic(err)
	}
	defer itr.Close()

	for itr.Valid() {
		key := itr.Key()
		_, height, ok := parseShKey(key)
		if ok {
			return height, nil
		}
		itr.Next()
	}

	return -1, nil
}

// FirstSignedHeaderHeight returns the first SignedHeader height stored.
//
// Safe for concurrent use by multiple goroutines.
func (s *dbs) FirstSignedHeaderHeight() (int64, error) {
	itr, err := s.db.Iterator(
		s.shKey(1),
		append(s.shKey(1<<63-1), byte(0x00)),
	)
	if err != nil {
		panic(err)
	}
	defer itr.Close()

	for itr.Valid() {
		key := itr.Key()
		_, height, ok := parseShKey(key)
		if ok {
			return height, nil
		}
		itr.Next()
	}

	return -1, nil
}

// SignedHeaderBefore iterates over headers until it finds a header before
// the given height. It returns ErrSignedHeaderNotFound if no such header exists.
//
// Safe for concurrent use by multiple goroutines.
func (s *dbs) SignedHeaderBefore(height int64) (*types.SignedHeader, error) {
	if height <= 0 {
		panic("negative or zero height")
	}

	itr, err := s.db.ReverseIterator(
		nil,
		s.shKey(height),
	)
	if err != nil {
		panic(err)
	}
	defer itr.Close()

	for itr.Valid() {
		key := itr.Key()
		_, existingHeight, ok := parseShKey(key)
		if ok {
			return s.SignedHeader(existingHeight)
		}
		itr.Next()
	}

	return nil, store.ErrSignedHeaderNotFound
}

// Prune prunes header & validator set pairs until there are only size pairs
// left.
//
// Safe for concurrent use by multiple goroutines.
func (s *dbs) Prune(size uint16) error {
	// 1) Check how many we need to prune.
	s.mtx.RLock()
	sSize := s.size
	s.mtx.RUnlock()

	if sSize <= size { // nothing to prune
		return nil
	}
	numToPrune := sSize - size

	// 2) Iterate over headers and perform a batch operation.
	itr, err := s.db.Iterator(
		s.shKey(1),
		append(s.shKey(1<<63-1), byte(0x00)),
	)
	if err != nil {
		panic(err)
	}

	b := s.db.NewBatch()

	pruned := 0
	for itr.Valid() && numToPrune > 0 {
		key := itr.Key()
		_, height, ok := parseShKey(key)
		if ok {
			b.Delete(s.shKey(height))
			b.Delete(s.vsKey(height))
		}
		itr.Next()
		numToPrune--
		pruned++
	}

	itr.Close()

	err = b.WriteSync()
	b.Close()
	if err != nil {
		return err
	}

	// 3) Update size.
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.size -= uint16(pruned)

	if wErr := s.db.SetSync(sizeKey, marshalSize(s.size)); wErr != nil {
		return errors.Wrap(wErr, "failed to persist size")
	}

	return nil
}

// Size returns the number of header & validator set pairs.
//
// Safe for concurrent use by multiple goroutines.
func (s *dbs) Size() uint16 {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return s.size
}

func (s *dbs) shKey(height int64) []byte {
	return []byte(fmt.Sprintf("sh/%s/%020d", s.prefix, height))
}

func (s *dbs) vsKey(height int64) []byte {
	return []byte(fmt.Sprintf("vs/%s/%020d", s.prefix, height))
}

var keyPattern = regexp.MustCompile(`^(sh|vs)/([^/]*)/([0-9]+)$`)

func parseKey(key []byte) (part string, prefix string, height int64, ok bool) {
	submatch := keyPattern.FindSubmatch(key)
	if submatch == nil {
		return "", "", 0, false
	}
	part = string(submatch[1])
	prefix = string(submatch[2])
	height, err := strconv.ParseInt(string(submatch[3]), 10, 64)
	if err != nil {
		return "", "", 0, false
	}
	ok = true // good!
	return
}

func parseShKey(key []byte) (prefix string, height int64, ok bool) {
	var part string
	part, prefix, height, ok = parseKey(key)
	if part != "sh" {
		return "", 0, false
	}
	return
}

func marshalSize(size uint16) []byte {
	bs := make([]byte, 2)
	binary.LittleEndian.PutUint16(bs, size)
	return bs
}

func unmarshalSize(bz []byte) uint16 {
	return binary.LittleEndian.Uint16(bz)
}
