package core

import (
	"testing"
	"time"

	"github.com/tendermint/tendermint/types"
)

func TestGetValidatorsWithTimeout(t *testing.T) {
	height, vs := getValidatorsWithTimeout(
		testValidatorReceiver{},
		time.Millisecond,
	)

	if height != -1 {
		t.Errorf("expected negative height")
	}

	if len(vs) != 0 {
		t.Errorf("expected no validators")
	}
}

type testValidatorReceiver struct{}

func (tr testValidatorReceiver) GetValidators() (int64, []*types.Validator) {
	vs := []*types.Validator{}

	for i := 0; i < 3; i++ {
		v, _ := types.RandValidator(true, 10)

		vs = append(vs, v)
	}

	time.Sleep(time.Millisecond)

	return 10, vs
}
