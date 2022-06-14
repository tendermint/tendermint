//nolint: gosec
package main

import (
	"fmt"
	"os"
	osexec "os/exec"
)

// execute executes a shell command.
func exec(args ...string) error {
	cmd := osexec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	switch err := err.(type) {
	case nil:
		return nil
	case *osexec.ExitError:
		return fmt.Errorf("failed to run %q:\n%v", args, string(out))
	default:
		return err
	}
}

// execVerbose executes a shell command while displaying its output.
func execVerbose(args ...string) error {
	cmd := osexec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
