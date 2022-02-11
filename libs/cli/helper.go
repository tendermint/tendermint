package cli

import (
	"os"

	"github.com/spf13/cobra"
)

// RunWithArgs executes the given command with the specified command line args
// and environmental variables set. It returns any error returned from cmd.Execute()
//
// This is only used in testing.
func RunWithArgs(cmd *cobra.Command, args []string, env map[string]string) error {
	oargs := os.Args
	oenv := map[string]string{}
	// defer returns the environment back to normal
	defer func() {
		os.Args = oargs
		for k, v := range oenv {
			os.Setenv(k, v)
		}
	}()

	// set the args and env how we want them
	os.Args = args
	for k, v := range env {
		// backup old value if there, to restore at end
		oenv[k] = os.Getenv(k)
		err := os.Setenv(k, v)
		if err != nil {
			return err
		}
	}

	// and finally run the command
	return cmd.Execute()
}
