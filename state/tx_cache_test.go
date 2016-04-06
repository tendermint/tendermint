package state

import (
	"bytes"
	"testing"

	stypes "github.com/eris-ltd/tendermint/state/types"
	"github.com/eris-ltd/tendermint/wire"
)

func TestStateToFromVMAccount(t *testing.T) {
	acmAcc1, _ := stypes.RandAccount(true, 456)
	vmAcc := toVMAccount(acmAcc1)
	acmAcc2 := toStateAccount(vmAcc)

	acmAcc1Bytes := wire.BinaryBytes(acmAcc1)
	acmAcc2Bytes := wire.BinaryBytes(acmAcc2)
	if !bytes.Equal(acmAcc1Bytes, acmAcc2Bytes) {
		t.Errorf("Unexpected account wire bytes\n%X vs\n%X",
			acmAcc1Bytes, acmAcc2Bytes)
	}

}
