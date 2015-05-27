package core

import (
	"fmt"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

func NameRegEntry(name string) (*ctypes.ResponseNameRegEntry, error) {
	st := consensusState.GetState() // performs a copy
	entry := st.GetNameRegEntry(name)
	if entry == nil {
		return nil, fmt.Errorf("Name %s not found", name)
	}
	return &ctypes.ResponseNameRegEntry{entry}, nil
}
