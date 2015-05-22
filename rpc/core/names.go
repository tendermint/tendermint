package core

import (
	"fmt"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

// XXX: need we be careful about rendering bytes as string or is that their problem ?
func NameRegEntry(name []byte) (*ctypes.ResponseNameRegEntry, error) {
	st := consensusState.GetState() // performs a copy
	entry := st.GetNameRegEntry(name)
	if entry == nil {
		return nil, fmt.Errorf("Name %s not found", name)
	}
	return &ctypes.ResponseNameRegEntry{entry}, nil
}
