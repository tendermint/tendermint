package certifiers

import (
	"encoding/hex"
	"sort"

	certerr "github.com/tendermint/tendermint/certifiers/errors"
)

type memStoreProvider struct {
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

func NewMemStoreProvider() Provider {
	return &memStoreProvider{
		byHeight: fullCommits{},
		byHash:   map[string]FullCommit{},
	}
}

func (m *memStoreProvider) encodeHash(hash []byte) string {
	return hex.EncodeToString(hash)
}

func (m *memStoreProvider) StoreCommit(fc FullCommit) error {
	// make sure the fc is self-consistent before saving
	err := fc.ValidateBasic(fc.Commit.Header.ChainID)
	if err != nil {
		return err
	}

	// store the valid fc
	key := m.encodeHash(fc.ValidatorsHash())
	m.byHash[key] = fc
	m.byHeight = append(m.byHeight, fc)
	sort.Sort(m.byHeight)
	return nil
}

func (m *memStoreProvider) GetByHeight(h int) (FullCommit, error) {
	// search from highest to lowest
	for i := len(m.byHeight) - 1; i >= 0; i-- {
		fc := m.byHeight[i]
		if fc.Height() <= h {
			return fc, nil
		}
	}
	return FullCommit{}, certerr.ErrCommitNotFound()
}

func (m *memStoreProvider) GetByHash(hash []byte) (FullCommit, error) {
	var err error
	fc, ok := m.byHash[m.encodeHash(hash)]
	if !ok {
		err = certerr.ErrCommitNotFound()
	}
	return fc, err
}

func (m *memStoreProvider) LatestCommit() (FullCommit, error) {
	l := len(m.byHeight)
	if l == 0 {
		return FullCommit{}, certerr.ErrCommitNotFound()
	}
	return m.byHeight[l-1], nil
}
