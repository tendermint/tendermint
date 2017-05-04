package cli

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
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

		viper.Reset()
		args := append([]string{cmd.Use}, tc.args...)
		err := runWithArgs(cmd, args, tc.env)
		require.Nil(err, i)
		assert.Equal(tc.expected, foo, i)
	}
}

func writeConfig(vals map[string]string) (string, error) {
	cdir, err := ioutil.TempDir("", "test-cli")
	if err != nil {
		return "", err
	}
	data := ""
	for k, v := range vals {
		data = data + fmt.Sprintf("%s = \"%s\"\n", k, v)
	}
	cfile := filepath.Join(cdir, "config.toml")
	err = ioutil.WriteFile(cfile, []byte(data), 0666)
	return cdir, err
}

func TestSetupConfig(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// we pre-create two config files we can refer to in the rest of
	// the test cases.
	cval1, cval2 := "fubble", "wubble"
	conf1, err := writeConfig(map[string]string{"boo": cval1})
	require.Nil(err)
	// even with some ignored fields, should be no problem
	conf2, err := writeConfig(map[string]string{"boo": cval2, "foo": "bar"})
	require.Nil(err)

	cases := []struct {
		args     []string
		env      map[string]string
		expected string
	}{
		{nil, nil, ""},
		// setting on the command line
		{[]string{"--boo", "haha"}, nil, "haha"},
		{[]string{"--root", conf1}, nil, cval1},
		// test both variants of the prefix
		{nil, map[string]string{"RD_BOO": "bang"}, "bang"},
		{nil, map[string]string{"RD_ROOT": conf1}, cval1},
		{nil, map[string]string{"RDROOT": conf2}, cval2},
		{nil, map[string]string{"RDHOME": conf1}, cval1},
		// and when both are set??? HOME wins every time!
		{[]string{"--root", conf1}, map[string]string{"RDHOME": conf2}, cval2},
	}

	for idx, tc := range cases {
		i := strconv.Itoa(idx)
		// test command that store value of foobar in local variable
		var foo string
		boo := &cobra.Command{
			Use: "reader",
			RunE: func(cmd *cobra.Command, args []string) error {
				foo = viper.GetString("boo")
				return nil
			},
		}
		boo.Flags().String("boo", "", "Some test value from config")
		cmd := PrepareBaseCmd(boo, "RD", "/qwerty/asdfgh") // some missing dir...

		viper.Reset()
		args := append([]string{cmd.Use}, tc.args...)
		err := runWithArgs(cmd, args, tc.env)
		require.Nil(err, i)
		assert.Equal(tc.expected, foo, i)
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
	conf1, err := writeConfig(map[string]string{"name": cval1})
	require.Nil(err)
	// even with some ignored fields, should be no problem
	conf2, err := writeConfig(map[string]string{"name": cval2, "foo": "bar"})
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

		viper.Reset()
		args := append([]string{cmd.Use}, tc.args...)
		err := runWithArgs(cmd, args, tc.env)
		require.Nil(err, i)
		assert.Equal(tc.expected, cfg, i)
	}
}

func TestSetupDebug(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	cases := []struct {
		args     []string
		env      map[string]string
		long     bool
		expected string
	}{
		{nil, nil, false, "Debug flag = false"},
		{[]string{"--debug"}, nil, true, "Debug flag = true"},
		{[]string{"--no-such-flag"}, nil, false, "unknown flag: --no-such-flag"},
		{nil, map[string]string{"DBG_DEBUG": "true"}, true, "Debug flag = true"},
	}

	for idx, tc := range cases {
		i := strconv.Itoa(idx)
		// test command that store value of foobar in local variable
		debug := &cobra.Command{
			Use: "debug",
			RunE: func(cmd *cobra.Command, args []string) error {
				return errors.Errorf("Debug flag = %t", viper.GetBool(DebugFlag))
			},
		}
		cmd := PrepareBaseCmd(debug, "DBG", "/qwerty/asdfgh") // some missing dir..

		viper.Reset()
		args := append([]string{cmd.Use}, tc.args...)
		out, err := runCaptureWithArgs(cmd, args, tc.env)
		require.NotNil(err, i)
		msg := strings.Split(out, "\n")
		desired := fmt.Sprintf("ERROR: %s", tc.expected)
		assert.Equal(desired, msg[0], i)
		if tc.long && assert.True(len(msg) > 2, i) {
			// the next line starts the stack trace...
			assert.Contains(msg[1], "TestSetupDebug", i)
			assert.Contains(msg[2], "setup_test.go", i)
		}
	}
}

// runWithArgs executes the given command with the specified command line args
// and environmental variables set. It returns any error returned from cmd.Execute()
func runWithArgs(cmd Executable, args []string, env map[string]string) error {
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

// runCaptureWithArgs executes the given command with the specified command line args
// and environmental variables set. It returns whatever was writen to
// stdout along with any error returned from cmd.Execute()
func runCaptureWithArgs(cmd Executable, args []string, env map[string]string) (output string, err error) {
	old := os.Stdout // keep backup of the real stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		os.Stdout = old // restoring the real stdout
	}()

	outC := make(chan string)
	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		var buf bytes.Buffer
		// io.Copy will end when we call w.Close() below
		io.Copy(&buf, r)
		outC <- buf.String()
	}()

	// now run the command
	err = runWithArgs(cmd, args, env)

	// and grab the stdout to return
	w.Close()
	output = <-outC
	return output, err
}
