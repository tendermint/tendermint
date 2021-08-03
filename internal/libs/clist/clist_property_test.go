package clist_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/tendermint/tendermint/internal/libs/clist"
)

func TestCListProperties(t *testing.T) {
	rapid.Check(t, rapid.Run(&clistModel{}))
}

type clistModel struct {
	clist *clist.CList

	model []*clist.CElement
}

func (m *clistModel) Init(t *rapid.T) {
	m.clist = clist.New()
	m.model = []*clist.CElement{}
}

func (m *clistModel) PushBack(t *rapid.T) {
	value := rapid.String().Draw(t, "value").(string)
	el := m.clist.PushBack(value)
	m.model = append(m.model, el)
}

func (m *clistModel) Remove(t *rapid.T) {
	if len(m.model) == 0 {
		return
	}
	ix := rapid.IntRange(0, len(m.model)-1).Draw(t, "index").(int)
	value := m.model[ix]
	m.model = append(m.model[:ix], m.model[ix+1:]...)
	m.clist.Remove(value)
}

func (m *clistModel) Check(t *rapid.T) {
	require.Equal(t, len(m.model), m.clist.Len())
	if len(m.model) == 0 {
		return
	}
	require.Equal(t, m.model[0], m.clist.Front())
	require.Equal(t, m.model[len(m.model)-1], m.clist.Back())

	iter := m.clist.Front()
	for _, val := range m.model {
		require.Equal(t, val, iter)
		iter = iter.Next()
	}
}
