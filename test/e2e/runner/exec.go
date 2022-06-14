package main

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
)

// execute executes a shell command.
func exec(ctx context.Context, args ...string) error {
	// nolint: gosec
	// G204: Subprocess launched with a potential tainted input or cmd arguments
	cmd := osexec.CommandContext(ctx, args[0], args[1:]...)
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
func execVerbose(ctx context.Context, args ...string) error {
	// nolint: gosec
	// G204: Subprocess launched with a potential tainted input or cmd arguments
	cmd := osexec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
