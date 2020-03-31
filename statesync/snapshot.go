package statesync

import (
	"crypto/sha256"
	"encoding/binary"
)

// snapshotHash is a snapshot hash, used for e.g. map keys
type snapshotHash [sha256.Size]byte

// snapshot contains data about a snapshot
type snapshot struct {
	Height      uint64
	Format      uint32
	ChunkHashes [][]byte
	Metadata    []byte
}

// Hash generates a hash for the snapshot, used for e.g. map keys
func (s *snapshot) Hash() snapshotHash {
	var buf []byte
	hasher := sha256.New()

	buf = make([]byte, 8)
	binary.BigEndian.PutUint64(buf, s.Height)
	_, err := hasher.Write(buf)
	if err != nil {
		panic(err)
	}

	buf = make([]byte, 4)
	binary.BigEndian.PutUint32(buf, s.Format)
	_, err = hasher.Write(buf)
	if err != nil {
		panic(err)
	}

	for _, h := range s.ChunkHashes {
		_, err = hasher.Write(h)
		if err != nil {
			panic(err)
		}
	}

	if s.Metadata != nil {
		_, err = hasher.Write(s.Metadata)
		if err != nil {
			panic(err)
		}
	}

	var hash snapshotHash
	copy(hash[:], hasher.Sum(nil))
	return hash
}

// chunk contains data about a chunk
type chunk struct {
	Index uint32
	Body  []byte
}

// Hash generates a hash for the chunk body, used for verification
func (c *chunk) Hash() []byte {
	hash := sha256.Sum256(c.Body)
	return hash[:]
}
