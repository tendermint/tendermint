package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
)

const (
	ctxTimeout = 4 * time.Second
	HomeFlag   = "home"
	TraceFlag  = "trace"
	OutputFlag = "output"
)

// RootCommand constructs the root command-line entry point for Tendermint core.
func RootCommand(conf *config.Config, logger log.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tendermint",
		Short: "BFT state machine replication for applications in any programming languages",
	}
	cmd.PersistentFlags().StringP(HomeFlag, "", os.ExpandEnv(filepath.Join("$HOME", config.DefaultTendermintDir)), "directory for config and data")
	cmd.PersistentFlags().Bool(TraceFlag, false, "print out full stack trace on errors")
	cmd.PersistentFlags().String("log-level", conf.LogLevel, "log level")
	cobra.OnInitialize(func() { initEnv("TM") })
	return cmd
}

// InitEnv sets to use ENV variables if set.
func initEnv(prefix string) {
	// This copies all variables like TMROOT to TM_ROOT,
	// so we can support both formats for the user
	prefix = strings.ToUpper(prefix)
	ps := prefix + "_"
	for _, e := range os.Environ() {
		kv := strings.SplitN(e, "=", 2)
		if len(kv) == 2 {
			k, v := kv[0], kv[1]
			if strings.HasPrefix(k, prefix) && !strings.HasPrefix(k, ps) {
				k2 := strings.Replace(k, prefix, ps, 1)
				os.Setenv(k2, v)
			}
		}
	}

	// env variables with TM prefix (eg. TM_ROOT)
	viper.SetEnvPrefix(prefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv()
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
