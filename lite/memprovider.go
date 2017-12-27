package lite

import (
	"encoding/hex"
	"sort"
	"sync"

	liteErr "github.com/tendermint/tendermint/lite/errors"
)

type memStoreProvider struct {
	mtx sync.RWMutex
	// byHeight is always sorted by Height... need to support range search (nil, h]
	// btree would be more efficient for larger sets
	byHeight fullCommits
	byHash   map[string]FullCommit
}

// fullCommits just exists to allow easy sorting
type fullCommits []FullCommit

func (s fullCommits) Len() int      { return len(s) }
func (s fullCommits) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s fullCommits) Less(i, j int) bool {
	return s[i].Height() < s[j].Height()
}

// NewMemStoreProvider returns a new in-memory provider.
func NewMemStoreProvider() Provider {
	return &memStoreProvider{
		byHeight: fullCommits{},
		byHash:   map[string]FullCommit{},
	}
}

func (m *memStoreProvider) encodeHash(hash []byte) string {
	return hex.EncodeToString(hash)
}

// StoreCommit stores a FullCommit after verifying it.
func (m *memStoreProvider) StoreCommit(fc FullCommit) error {
	// make sure the fc is self-consistent before saving
	err := fc.ValidateBasic(fc.Commit.Header.ChainID)
	if err != nil {
		return err
	}

	// store the valid fc
	key := m.encodeHash(fc.ValidatorsHash())

	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.byHash[key] = fc
	m.byHeight = append(m.byHeight, fc)
	sort.Sort(m.byHeight)
	return nil
}

// GetByHeight returns the FullCommit for height h or an error if the commit is not found.
func (m *memStoreProvider) GetByHeight(h int64) (FullCommit, error) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	// search from highest to lowest
	for i := len(m.byHeight) - 1; i >= 0; i-- {
		fc := m.byHeight[i]
		if fc.Height() <= h {
			return fc, nil
		}
	}
	return FullCommit{}, liteErr.ErrCommitNotFound()
}

// GetByHash returns the FullCommit for the hash or an error if the commit is not found.
func (m *memStoreProvider) GetByHash(hash []byte) (FullCommit, error) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	fc, ok := m.byHash[m.encodeHash(hash)]
	if !ok {
		return fc, liteErr.ErrCommitNotFound()
	}
	return fc, nil
}

// LatestCommit returns the latest FullCommit or an error if no commits exist.
func (m *memStoreProvider) LatestCommit() (FullCommit, error) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	l := len(m.byHeight)
	if l == 0 {
		return FullCommit{}, liteErr.ErrCommitNotFound()
	}
	return m.byHeight[l-1], nil
}
