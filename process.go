package process

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

type Process struct {
	Label      string
	ExecPath   string
	Args       []string
	Pid        int
	StartTime  time.Time
	EndTime    time.Time
	Cmd        *exec.Cmd        `json:"-"`
	ExitState  *os.ProcessState `json:"-"`
	InputFile  io.Reader        `json:"-"`
	OutputFile io.WriteCloser   `json:"-"`
	WaitCh     chan struct{}    `json:"-"`
}

// execPath: command name
// args: args to command. (should not include name)
func StartProcess(label string, dir string, execPath string, args []string, inFile io.Reader, outFile io.WriteCloser) (*Process, error) {
	cmd := exec.Command(execPath, args...)
	cmd.Dir = dir
	cmd.Stdout = outFile
	cmd.Stderr = outFile
	cmd.Stdin = inFile
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	proc := &Process{
		Label:      label,
		ExecPath:   execPath,
		Args:       args,
		Pid:        cmd.Process.Pid,
		StartTime:  time.Now(),
		Cmd:        cmd,
		ExitState:  nil,
		InputFile:  inFile,
		OutputFile: outFile,
		WaitCh:     make(chan struct{}),
	}
	go func() {
		err := proc.Cmd.Wait()
		if err != nil {
			// fmt.Printf("Process exit: %v\n", err)
			if exitError, ok := err.(*exec.ExitError); ok {
				proc.ExitState = exitError.ProcessState
			}
		}
		proc.ExitState = proc.Cmd.ProcessState
		proc.EndTime = time.Now() // TODO make this goroutine-safe
		err = proc.OutputFile.Close()
		if err != nil {
			fmt.Printf("Error closing output file for %v: %v\n", proc.Label, err)
		}
		close(proc.WaitCh)
	}()
	return proc, nil
}

func (proc *Process) StopProcess(kill bool) error {
	defer proc.OutputFile.Close()
	if kill {
		// fmt.Printf("Killing process %v\n", proc.Cmd.Process)
		return proc.Cmd.Process.Kill()
	} else {
		// fmt.Printf("Stopping process %v\n", proc.Cmd.Process)
		return proc.Cmd.Process.Signal(os.Interrupt)
	}
}
