package core

import (
	"fmt"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

func GetName(name string) (*types.NameRegEntry, error) {
	st := consensusState.GetState() // performs a copy
	entry := st.GetNameRegEntry(name)
	if entry == nil {
		return nil, fmt.Errorf("Name %s not found", name)
	}
	return entry, nil
}

func ListNames() (*ctypes.ResponseListNames, error) {
	var blockHeight int
	var names []*types.NameRegEntry
	state := consensusState.GetState()
	blockHeight = state.LastBlockHeight
	state.GetNames().Iterate(func(key interface{}, value interface{}) bool {
		names = append(names, value.(*types.NameRegEntry))
		return false
	})
	return &ctypes.ResponseListNames{blockHeight, names}, nil
}
