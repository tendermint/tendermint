package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	data "github.com/tendermint/go-wire/data"
	"github.com/tendermint/go-wire/data/base58"
)

const (
	RootFlag     = "root"
	HomeFlag     = "home"
	OutputFlag   = "output"
	EncodingFlag = "encoding"
)

// PrepareBaseCmd is meant for tendermint and other servers
func PrepareBaseCmd(cmd *cobra.Command, envPrefix, defautRoot string) func() {
	cobra.OnInitialize(func() { initEnv(envPrefix) })
	cmd.PersistentFlags().StringP(RootFlag, "r", defautRoot, "DEPRECATED. Use --home")
	cmd.PersistentFlags().StringP(HomeFlag, "h", defautRoot, "root directory for config and data")
	cmd.PersistentPreRunE = multiE(bindFlags, cmd.PersistentPreRunE)
	return func() { execute(cmd) }
}

// PrepareMainCmd is meant for client side libs that want some more flags
func PrepareMainCmd(cmd *cobra.Command, envPrefix, defautRoot string) func() {
	cmd.PersistentFlags().StringP(EncodingFlag, "e", "hex", "Binary encoding (hex|b64|btc)")
	cmd.PersistentFlags().StringP(OutputFlag, "o", "text", "Output format (text|json)")
	cmd.PersistentPreRunE = multiE(setEncoding, validateOutput, cmd.PersistentPreRunE)
	return PrepareBaseCmd(cmd, envPrefix, defautRoot)
}

// initEnv sets to use ENV variables if set.
func initEnv(prefix string) {
	copyEnvVars(prefix)

	// env variables with TM prefix (eg. TM_ROOT)
	viper.SetEnvPrefix(prefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
}

// This copies all variables like TMROOT to TM_ROOT,
// so we can support both formats for the user
func copyEnvVars(prefix string) {
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
}

// execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func execute(cmd *cobra.Command) {
	// TODO: this can do something cooler with debug and log-levels
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

type wrapE func(cmd *cobra.Command, args []string) error

func multiE(fs ...wrapE) wrapE {
	return func(cmd *cobra.Command, args []string) error {
		for _, f := range fs {
			if f != nil {
				if err := f(cmd, args); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

func bindFlags(cmd *cobra.Command, args []string) error {
	// cmd.Flags() includes flags from this command and all persistent flags from the parent
	if err := viper.BindPFlags(cmd.Flags()); err != nil {
		return err
	}

	// rootDir is command line flag, env variable, or default $HOME/.tlc
	// NOTE: we support both --root and --home for now, but eventually only --home
	rootDir := viper.GetString(HomeFlag)
	if !viper.IsSet(HomeFlag) && viper.IsSet(RootFlag) {
		rootDir = viper.GetString(RootFlag)
	}
	viper.SetConfigName("config") // name of config file (without extension)
	viper.AddConfigPath(rootDir)  // search root directory

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		// stderr, so if we redirect output to json file, this doesn't appear
		// fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	} else if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
		// we ignore not found error, only parse error
		// stderr, so if we redirect output to json file, this doesn't appear
		fmt.Fprintf(os.Stderr, "%#v", err)
	}
	return nil
}

// setEncoding reads the encoding flag
func setEncoding(cmd *cobra.Command, args []string) error {
	// validate and set encoding
	enc := viper.GetString("encoding")
	switch enc {
	case "hex":
		data.Encoder = data.HexEncoder
	case "b64":
		data.Encoder = data.B64Encoder
	case "btc":
		data.Encoder = base58.BTCEncoder
	default:
		return errors.Errorf("Unsupported encoding: %s", enc)
	}
	return nil
}

func validateOutput(cmd *cobra.Command, args []string) error {
	// validate output format
	output := viper.GetString(OutputFlag)
	switch output {
	case "text", "json":
	default:
		return errors.Errorf("Unsupported output format: %s", output)
	}
	return nil
}
