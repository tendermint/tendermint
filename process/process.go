package process

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

func makeFile(prefix string) *os.File {
	now := time.Now()
	filename := fmt.Sprintf("%v_%v.out", prefix, now.Format("2006_01_02_15_04_05_MST"))
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	return f
}

type Process struct {
	Label     string
	ExecPath  string
	StartTime time.Time
	Cmd       *exec.Cmd        `json:"-"`
	Output    *os.File         `json:"-"`
	ExitState *os.ProcessState `json:"-"`
}

const (
	ProcessModeStd = iota
	ProcessModeDaemon
)

// execPath: command name
// args: args to command. (should not include name)
func Create(mode int, label string, execPath string, args ...string) *Process {
	out := makeFile(label)
	cmd := exec.Command(execPath, args...)
	switch mode {
	case ProcessModeStd:
		cmd.Stdout = io.MultiWriter(os.Stdout, out)
		cmd.Stderr = io.MultiWriter(os.Stderr, out)
		cmd.Stdin = nil
	case ProcessModeDaemon:
		cmd.Stdout = out
		cmd.Stderr = out
		cmd.Stdin = nil
	}
	if err := cmd.Start(); err != nil {
		fmt.Printf("Failed to run command. %v\n", err)
		return nil
	} else {
		fmt.Printf("Success!")
	}
	return &Process{
		Label:     label,
		ExecPath:  execPath,
		StartTime: time.Now(),
		Cmd:       cmd,
		Output:    out,
	}
}

func Wait(proc *Process) error {
	exitErr := proc.Cmd.Wait()
	if exitErr != nil {
		fmt.Printf("Process exit: %v\n", exitErr)
		proc.ExitState = exitErr.(*exec.ExitError).ProcessState
	}
	return exitErr
}

func Stop(proc *Process, kill bool) error {
	if kill {
		return proc.Cmd.Process.Kill()
	} else {
		return proc.Cmd.Process.Signal(os.Interrupt)
	}
}
