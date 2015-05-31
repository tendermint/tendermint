package process

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
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
	OutputPath string
	Cmd        *exec.Cmd        `json:"-"`
	ExitState  *os.ProcessState `json:"-"`
	OutputFile *os.File         `json:"-"`
	WaitCh     chan struct{}    `json:"-"`
}

const (
	ProcessModeStd = iota
	ProcessModeDaemon
)

// execPath: command name
// args: args to command. (should not include name)
func Create(mode int, label string, execPath string, args []string, input string, outPath string) (*Process, error) {
	outFile, err := os.OpenFile(outPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(execPath, args...)
	switch mode {
	case ProcessModeStd:
		cmd.Stdout = io.MultiWriter(os.Stdout, outFile)
		cmd.Stderr = io.MultiWriter(os.Stderr, outFile)
		cmd.Stdin = nil
	case ProcessModeDaemon:
		cmd.Stdout = outFile
		cmd.Stderr = outFile
		cmd.Stdin = nil
	}
	if input != "" {
		cmd.Stdin = bytes.NewReader([]byte(input))
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	proc := &Process{
		Label:      label,
		ExecPath:   execPath,
		Args:       args,
		Pid:        cmd.Process.Pid,
		StartTime:  time.Now(),
		OutputPath: outPath,
		Cmd:        cmd,
		ExitState:  nil,
		OutputFile: outFile,
		WaitCh:     make(chan struct{}),
	}
	go func() {
		err := proc.Cmd.Wait()
		if err != nil {
			fmt.Printf("Process exit: %v\n", err)
			if exitError, ok := err.(*exec.ExitError); ok {
				proc.ExitState = exitError.ProcessState
			}
		}
		proc.EndTime = time.Now() // TODO make this goroutine-safe
		err = proc.OutputFile.Close()
		if err != nil {
			fmt.Printf("Error closing output file for %v: %v\n", proc.Label, err)
		}
		close(proc.WaitCh)
	}()
	return proc, nil
}

func ReadOutput(proc *Process) string {
	output, err := ioutil.ReadFile(proc.OutputPath)
	if err != nil {
		return fmt.Sprintf("ERROR READING OUTPUT: %v", err)
	}
	return string(output)
}

func Stop(proc *Process, kill bool) error {
	defer proc.OutputFile.Close()
	if kill {
		fmt.Printf("Killing process %v\n", proc.Cmd.Process)
		return proc.Cmd.Process.Kill()
	} else {
		fmt.Printf("Stopping process %v\n", proc.Cmd.Process)
		return proc.Cmd.Process.Signal(os.Interrupt)
	}
}
