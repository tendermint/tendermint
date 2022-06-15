package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
)

// writeConfigVals writes a toml file with the given values.
// It returns an error if writing was impossible.
func writeConfigVals(dir string, vals map[string]string) error {
	data := ""
	for k, v := range vals {
		data += fmt.Sprintf("%s = \"%s\"\n", k, v)
	}
	cfile := filepath.Join(dir, "config.toml")
	return os.WriteFile(cfile, []byte(data), 0600)
}

// clearConfig clears env vars, the given root dir, and resets viper.
func clearConfig(t *testing.T, dir string) *cfg.Config {
	t.Helper()
	require.NoError(t, os.Unsetenv("TMHOME"))
	require.NoError(t, os.Unsetenv("TM_HOME"))
	require.NoError(t, os.RemoveAll(dir))

	viper.Reset()
	conf := cfg.DefaultConfig()
	conf.SetRoot(dir)

	return conf
}

// prepare new rootCmd
func testRootCmd(conf *cfg.Config) *cobra.Command {
	logger := log.NewNopLogger()
	cmd := RootCommand(conf, logger)
	var l string
	cmd.PersistentFlags().String("log", l, "Log")
	return cmd
}

func testSetup(ctx context.Context, t *testing.T, conf *cfg.Config, args []string, env map[string]string) error {
	t.Helper()

	cmd := testRootCmd(conf)
	viper.Set(HomeFlag, conf.RootDir)

	// run with the args and env
	args = append([]string{cmd.Use}, args...)
	return RunWithArgs(ctx, cmd, args, env)
}

func TestRootHome(t *testing.T) {
	defaultRoot := t.TempDir()
	newRoot := filepath.Join(defaultRoot, "something-else")
	cases := []struct {
		args []string
		env  map[string]string
		root string
	}{
		{nil, nil, defaultRoot},
		{[]string{"--home", newRoot}, nil, newRoot},
		{nil, map[string]string{"TMHOME": newRoot}, newRoot},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i, tc := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			conf := clearConfig(t, tc.root)

			err := testSetup(ctx, t, conf, tc.args, tc.env)
			require.NoError(t, err)

			require.Equal(t, tc.root, conf.RootDir)
			require.Equal(t, tc.root, conf.P2P.RootDir)
			require.Equal(t, tc.root, conf.Consensus.RootDir)
			require.Equal(t, tc.root, conf.Mempool.RootDir)
		})
	}
}

func TestRootFlagsEnv(t *testing.T) {
	// defaults
	defaults := cfg.DefaultConfig()
	defaultDir := t.TempDir()

	defaultLogLvl := defaults.LogLevel

	cases := []struct {
		args     []string
		env      map[string]string
		logLevel string
	}{
		{[]string{"--log", "debug"}, nil, defaultLogLvl}, // wrong flag
		{[]string{"--log-level", "debug"}, nil, "debug"}, // right flag
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i, tc := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			conf := clearConfig(t, defaultDir)

			err := testSetup(ctx, t, conf, tc.args, tc.env)
			require.NoError(t, err)

			assert.Equal(t, tc.logLevel, conf.LogLevel)
		})

	}
}

func TestRootConfig(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// write non-default config
	nonDefaultLogLvl := "debug"
	cvals := map[string]string{
		"log-level": nonDefaultLogLvl,
	}

	cases := []struct {
		args   []string
		env    map[string]string
		logLvl string
	}{
		{nil, nil, nonDefaultLogLvl},                // should load config
		{[]string{"--log-level=info"}, nil, "info"}, // flag over rides
	}

	for i, tc := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			defaultRoot := t.TempDir()
			conf := clearConfig(t, defaultRoot)
			conf.LogLevel = tc.logLvl

			// XXX: path must match cfg.defaultConfigPath
			configFilePath := filepath.Join(defaultRoot, "config")
			err := tmos.EnsureDir(configFilePath, 0700)
			require.NoError(t, err)

			// write the non-defaults to a different path
			// TODO: support writing sub configs so we can test that too
			err = writeConfigVals(configFilePath, cvals)
			require.NoError(t, err)

			cmd := testRootCmd(conf)

			// run with the args and env
			tc.args = append([]string{cmd.Use}, tc.args...)
			err = RunWithArgs(ctx, cmd, tc.args, tc.env)
			require.NoError(t, err)

			require.Equal(t, tc.logLvl, conf.LogLevel)
		})
	}
}

// WriteConfigVals writes a toml file with the given values.
// It returns an error if writing was impossible.
func WriteConfigVals(dir string, vals map[string]string) error {
	data := ""
	for k, v := range vals {
		data += fmt.Sprintf("%s = \"%s\"\n", k, v)
	}
	cfile := filepath.Join(dir, "config.toml")
	return os.WriteFile(cfile, []byte(data), 0600)
}

// RunWithArgs executes the given command with the specified command line args
// and environmental variables set. It returns any error returned from cmd.Execute()
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
