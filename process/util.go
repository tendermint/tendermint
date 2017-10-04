package process

import (
	. "github.com/tendermint/tmlibs/common"
)

// Runs a command and gets the result.
func Run(dir string, command string, args []string) (string, bool, error) {
	outFile := NewBufferCloser(nil)
	proc, err := StartProcess("", dir, command, args, nil, outFile)
	if err != nil {
		return "", false, err
	}

	<-proc.WaitCh

	if proc.ExitState.Success() {
		return outFile.String(), true, nil
	} else {
		return outFile.String(), false, nil
	}
}
