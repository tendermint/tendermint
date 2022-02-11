package cli

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// RunWithArgs executes the given command with the specified command line args
// and environmental variables set. It returns any error returned from cmd.Execute()
//
// This is only used in testing.
func RunWithArgs(ctx context.Context, cmd *cobra.Command, args []string, env map[string]string) error {
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
	return RunWithTrace(ctx, cmd)
}

func RunWithTrace(ctx context.Context, cmd *cobra.Command) error {
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.ExecuteContext(ctx); err != nil {
		if viper.GetBool(TraceFlag) {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			fmt.Fprintf(os.Stderr, "ERROR: %v\n%s\n", err, buf)
		} else {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		}

		return err
	}
	return nil
}
