package state

import (
	"context"

	"github.com/tendermint/tendermint/types"
)

type UpdateProvider interface {
	UpdateFunc(context.Context, State) (State, error)
}

// UpdateFunc is a function that can be used to update state
type UpdateFunc func(context.Context, State) (State, error)

// PrepareStateUpdates generates state updates that will set Dash-related state fields.
func PrepareStateUpdates(
	ctx context.Context,
	blockHeader types.Header,
	state State,
	updateProviders ...UpdateProvider) ([]UpdateFunc, error) {
	updates := []UpdateFunc{}
	for _, update := range updateProviders {
		updates = append(updates, update.UpdateFunc)
	}
	return updates, nil
}

func executeStateUpdates(ctx context.Context, state State, updates ...UpdateFunc) (State, error) {
	var err error
	for _, update := range updates {
		state, err = update(ctx, state)
		if err != nil {
			return State{}, err
		}
	}
	return state, nil
}
