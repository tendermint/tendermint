package cli

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupEnv(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	cases := []struct {
		args     []string
		env      map[string]string
		expected string
	}{
		{nil, nil, ""},
		{[]string{"--foobar", "bang!"}, nil, "bang!"},
		// make sure reset is good
		{nil, nil, ""},
		// test both variants of the prefix
		{nil, map[string]string{"DEMO_FOOBAR": "good"}, "good"},
		{nil, map[string]string{"DEMOFOOBAR": "silly"}, "silly"},
		// and that cli overrides env...
		{[]string{"--foobar", "important"},
			map[string]string{"DEMO_FOOBAR": "ignored"}, "important"},
	}

	for idx, tc := range cases {
		i := strconv.Itoa(idx)
		// test command that store value of foobar in local variable
		var foo string
		demo := &cobra.Command{
			Use: "demo",
			RunE: func(cmd *cobra.Command, args []string) error {
				foo = viper.GetString("foobar")
				return nil
			},
		}
		demo.Flags().String("foobar", "", "Some test value from config")
		cmd := PrepareBaseCmd(demo, "DEMO", "/qwerty/asdfgh") // some missing dir..
		cmd.Exit = func(int) {}

		viper.Reset()
		args := append([]string{cmd.Use}, tc.args...)
		err := RunWithArgs(cmd, args, tc.env)
		require.Nil(err, i)
		assert.Equal(tc.expected, foo, i)
	}
}

func TestSetupConfig(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// we pre-create two config files we can refer to in the rest of
	// the test cases.
	cval1, cval2 := "fubble", "wubble"
	conf1, err := WriteDemoConfig(map[string]string{"boo": cval1})
	require.Nil(err)
	// make sure it handles dashed-words in the config, and ignores random info
	conf2, err := WriteDemoConfig(map[string]string{"boo": cval2, "foo": "bar", "two-words": "WORD"})
	require.Nil(err)

	cases := []struct {
		args        []string
		env         map[string]string
		expected    string
		expectedTwo string
	}{
		{nil, nil, "", ""},
		// setting on the command line
		{[]string{"--boo", "haha"}, nil, "haha", ""},
		{[]string{"--two-words", "rocks"}, nil, "", "rocks"},
		{[]string{"--root", conf1}, nil, cval1, ""},
		// test both variants of the prefix
		{nil, map[string]string{"RD_BOO": "bang"}, "bang", ""},
		{nil, map[string]string{"RD_TWO_WORDS": "fly"}, "", "fly"},
		{nil, map[string]string{"RDTWO_WORDS": "fly"}, "", "fly"},
		{nil, map[string]string{"RD_ROOT": conf1}, cval1, ""},
		{nil, map[string]string{"RDROOT": conf2}, cval2, "WORD"},
		{nil, map[string]string{"RDHOME": conf1}, cval1, ""},
		// and when both are set??? HOME wins every time!
		{[]string{"--root", conf1}, map[string]string{"RDHOME": conf2}, cval2, "WORD"},
	}

	for idx, tc := range cases {
		i := strconv.Itoa(idx)
		// test command that store value of foobar in local variable
		var foo, two string
		boo := &cobra.Command{
			Use: "reader",
			RunE: func(cmd *cobra.Command, args []string) error {
				foo = viper.GetString("boo")
				two = viper.GetString("two-words")
				return nil
			},
		}
		boo.Flags().String("boo", "", "Some test value from config")
		boo.Flags().String("two-words", "", "Check out env handling -")
		cmd := PrepareBaseCmd(boo, "RD", "/qwerty/asdfgh") // some missing dir...
		cmd.Exit = func(int) {}

		viper.Reset()
		args := append([]string{cmd.Use}, tc.args...)
		err := RunWithArgs(cmd, args, tc.env)
		require.Nil(err, i)
		assert.Equal(tc.expected, foo, i)
		assert.Equal(tc.expectedTwo, two, i)
	}
}

type DemoConfig struct {
	Name   string `mapstructure:"name"`
	Age    int    `mapstructure:"age"`
	Unused int    `mapstructure:"unused"`
}

func TestSetupUnmarshal(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// we pre-create two config files we can refer to in the rest of
	// the test cases.
	cval1, cval2 := "someone", "else"
	conf1, err := WriteDemoConfig(map[string]string{"name": cval1})
	require.Nil(err)
	// even with some ignored fields, should be no problem
	conf2, err := WriteDemoConfig(map[string]string{"name": cval2, "foo": "bar"})
	require.Nil(err)

	// unused is not declared on a flag and remains from base
	base := DemoConfig{
		Name:   "default",
		Age:    42,
		Unused: -7,
	}
	c := func(name string, age int) DemoConfig {
		r := base
		// anything set on the flags as a default is used over
		// the default config object
		r.Name = "from-flag"
		if name != "" {
			r.Name = name
		}
		if age != 0 {
			r.Age = age
		}
		return r
	}

	cases := []struct {
		args     []string
		env      map[string]string
		expected DemoConfig
	}{
		{nil, nil, c("", 0)},
		// setting on the command line
		{[]string{"--name", "haha"}, nil, c("haha", 0)},
		{[]string{"--root", conf1}, nil, c(cval1, 0)},
		// test both variants of the prefix
		{nil, map[string]string{"MR_AGE": "56"}, c("", 56)},
		{nil, map[string]string{"MR_ROOT": conf1}, c(cval1, 0)},
		{[]string{"--age", "17"}, map[string]string{"MRHOME": conf2}, c(cval2, 17)},
	}

	for idx, tc := range cases {
		i := strconv.Itoa(idx)
		// test command that store value of foobar in local variable
		cfg := base
		marsh := &cobra.Command{
			Use: "marsh",
			RunE: func(cmd *cobra.Command, args []string) error {
				return viper.Unmarshal(&cfg)
			},
		}
		marsh.Flags().String("name", "from-flag", "Some test value from config")
		// if we want a flag to use the proper default, then copy it
		// from the default config here
		marsh.Flags().Int("age", base.Age, "Some test value from config")
		cmd := PrepareBaseCmd(marsh, "MR", "/qwerty/asdfgh") // some missing dir...
		cmd.Exit = func(int) {}

		viper.Reset()
		args := append([]string{cmd.Use}, tc.args...)
		err := RunWithArgs(cmd, args, tc.env)
		require.Nil(err, i)
		assert.Equal(tc.expected, cfg, i)
	}
}

func TestSetupTrace(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	cases := []struct {
		args     []string
		env      map[string]string
		long     bool
		expected string
	}{
		{nil, nil, false, "Trace flag = false"},
		{[]string{"--trace"}, nil, true, "Trace flag = true"},
		{[]string{"--no-such-flag"}, nil, false, "unknown flag: --no-such-flag"},
		{nil, map[string]string{"DBG_TRACE": "true"}, true, "Trace flag = true"},
	}

	for idx, tc := range cases {
		i := strconv.Itoa(idx)
		// test command that store value of foobar in local variable
		trace := &cobra.Command{
			Use: "trace",
			RunE: func(cmd *cobra.Command, args []string) error {
				return errors.Errorf("Trace flag = %t", viper.GetBool(TraceFlag))
			},
		}
		cmd := PrepareBaseCmd(trace, "DBG", "/qwerty/asdfgh") // some missing dir..
		cmd.Exit = func(int) {}

		viper.Reset()
		args := append([]string{cmd.Use}, tc.args...)
		stdout, stderr, err := RunCaptureWithArgs(cmd, args, tc.env)
		require.NotNil(err, i)
		require.Equal("", stdout, i)
		require.NotEqual("", stderr, i)
		msg := strings.Split(stderr, "\n")
		desired := fmt.Sprintf("ERROR: %s", tc.expected)
		assert.Equal(desired, msg[0], i)
		if tc.long && assert.True(len(msg) > 2, i) {
			// the next line starts the stack trace...
			assert.Contains(msg[1], "TestSetupTrace", i)
			assert.Contains(msg[2], "setup_test.go", i)
		}
	}
}
