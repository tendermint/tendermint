package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/state"
)

var RollbackStateCmd = &cobra.Command{
	Use:   "rollback",
	Short: "rollback tendermint state by one height",
	Long: `
Rollbacking state should be performed in the event of an incorrect applicate state transition
whereby Tendermint has persisted an incorrect app hash and is thus unable to make 
progress. This takes a state at height n and overwrites it with the state at height n - 1. 
It is expected that the application also rolls back to height n - 1. No blocks are removed 
so upon restarting Tendermint, the transactions in block n will be reexecuted against the 
application.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		height, hash, err := RollbackState(config)
		if err != nil {
			return err
		}

		fmt.Printf("Rolled back state to height %d and hash %v", height, hash)
		return nil
	},
}

// RollbackState takes a state at height n and overwrites it with the state
// at height n - 1. Note state here refers to tendermint state not application state.
// Returns the latest state height and app hash alongside an error if there was one.
func RollbackState(config *cfg.Config) (int64, []byte, error) {
	// use the parsed config to load the block and state store
	bs, ss, err := loadStateAndBlockStore(config)
	if err != nil {
		return -1, []byte{}, err
	}

	// rollback the last state
	return state.Rollback(bs, ss)
}
