package main

import "github.com/spf13/cobra"

var killCmd = &cobra.Command{
	Use:   "kill",
	Short: "Kill a Tendermint process while aggregating and packaging debugging data",
	RunE: func(cmd *cobra.Command, args []string) error {
		panic("not implemented")
	},
}
