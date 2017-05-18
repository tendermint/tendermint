package cli

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// WriteDemoConfig writes a toml file with the given values.
// It returns the RootDir the config.toml file is stored in,
// or an error if writing was impossible
func WriteDemoConfig(vals map[string]string) (string, error) {
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

// RunWithArgs executes the given command with the specified command line args
// and environmental variables set. It returns any error returned from cmd.Execute()
func RunWithArgs(cmd Executable, args []string, env map[string]string) error {
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

// RunCaptureWithArgs executes the given command with the specified command line args
// and environmental variables set. It returns whatever was writen to
// stdout along with any error returned from cmd.Execute()
func RunCaptureWithArgs(cmd Executable, args []string, env map[string]string) (output string, err error) {
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
	err = RunWithArgs(cmd, args, env)

	// and grab the stdout to return
	w.Close()
	output = <-outC
	return output, err
}
