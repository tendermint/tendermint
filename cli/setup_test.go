package cli

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Executable is the minimal interface to *corba.Command, so we can
// wrap if desired before the test
type Executable interface {
	Execute() error
}

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
		cmd := &cobra.Command{
			Use: "demo",
			RunE: func(cmd *cobra.Command, args []string) error {
				foo = viper.GetString("foobar")
				return nil
			},
		}
		cmd.Flags().String("foobar", "", "Some test value from config")
		PrepareBaseCmd(cmd, "DEMO", "/qwerty/asdfgh") // some missing dir..

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
	}

	for idx, tc := range cases {
		i := strconv.Itoa(idx)
		// test command that store value of foobar in local variable
		var foo string
		cmd := &cobra.Command{
			Use: "reader",
			RunE: func(cmd *cobra.Command, args []string) error {
				foo = viper.GetString("boo")
				return nil
			},
		}
		cmd.Flags().String("boo", "", "Some test value from config")
		PrepareBaseCmd(cmd, "RD", "/qwerty/asdfgh") // some missing dir...

		viper.Reset()
		args := append([]string{cmd.Use}, tc.args...)
		err := runWithArgs(cmd, args, tc.env)
		require.Nil(err, i)
		assert.Equal(tc.expected, foo, i)
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
