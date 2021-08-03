package clist_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/internal/libs/clist"
	"pgregory.net/rapid"
)

func TestCListProperties(t *testing.T) {
	rapid.Check(t, rapid.Run(&clistModel{}))
}

type clistModel struct {
	clist *clist.CList

	state []*clist.CElement
}

func (m *clistModel) Init(t *rapid.T) {
	m.clist = clist.New()
	m.state = []*clist.CElement{}
}

func (m *clistModel) PushBack(t *rapid.T) {
	value := rapid.String().Draw(t, "value").(string)
	el := m.clist.PushBack(value)
	m.state = append(m.state, el)
}

func (m *clistModel) Remove(t *rapid.T) {
	if len(m.state) == 0 {
		return
	}
	ix := rapid.IntRange(0, len(m.state)-1).Draw(t, "index").(int)
	value := m.state[ix]
	m.state = append(m.state[:ix], m.state[ix+1:]...)
	m.clist.Remove(value)
}

func (m *clistModel) Check(t *rapid.T) {
	require.Equal(t, len(m.state), m.clist.Len())
	if len(m.state) == 0 {
		return
	}
	require.Equal(t, m.state[0], m.clist.Front())
	require.Equal(t, m.state[len(m.state)-1], m.clist.Back())

	iter := m.clist.Front()
	for _, val := range m.state {
		require.Equal(t, val, iter)
		iter = iter.Next()
	}
}
