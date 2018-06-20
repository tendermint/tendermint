package cli

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// WriteConfigVals writes a toml file with the given values.
// It returns an error if writing was impossible.
func WriteConfigVals(dir string, vals map[string]string) error {
	data := ""
	for k, v := range vals {
		data = data + fmt.Sprintf("%s = \"%s\"\n", k, v)
	}
	cfile := filepath.Join(dir, "config.toml")
	return ioutil.WriteFile(cfile, []byte(data), 0666)
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

// RunCaptureWithArgs executes the given command with the specified command
// line args and environmental variables set. It returns string fields
// representing output written to stdout and stderr, additionally any error
// from cmd.Execute() is also returned
func RunCaptureWithArgs(cmd Executable, args []string, env map[string]string) (stdout, stderr string, err error) {
	oldout, olderr := os.Stdout, os.Stderr // keep backup of the real stdout
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout, os.Stderr = wOut, wErr
	defer func() {
		os.Stdout, os.Stderr = oldout, olderr // restoring the real stdout
	}()

	// copy the output in a separate goroutine so printing can't block indefinitely
	copyStd := func(reader *os.File) *(chan string) {
		stdC := make(chan string)
		go func() {
			var buf bytes.Buffer
			// io.Copy will end when we call reader.Close() below
			io.Copy(&buf, reader)
			stdC <- buf.String()
		}()
		return &stdC
	}
	outC := copyStd(rOut)
	errC := copyStd(rErr)

	// now run the command
	err = RunWithArgs(cmd, args, env)

	// and grab the stdout to return
	wOut.Close()
	wErr.Close()
	stdout = <-*outC
	stderr = <-*errC
	return stdout, stderr, err
}
