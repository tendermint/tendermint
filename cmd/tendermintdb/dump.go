package main

import "github.com/spf13/cobra"

var dumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Continiously poll a Tendermint process and dump debugging data into a single location",
	RunE: func(cmd *cobra.Command, args []string) error {
		panic("not implemented")
	},
}
