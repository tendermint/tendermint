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
	Cmd    *exec.Cmd
	Output *os.File
}

const (
	ProcessModeStd = iota
	ProcessModeDaemon
)

func CreateProcess(mode int, name string, args ...string) *Process {
	out := makeFile(name)
	cmd := exec.Command(name, args...)
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
		Cmd:    cmd,
		Output: out,
	}
}

func Watch(proc *Process) {
	exitErr := proc.Cmd.Wait()
	if exitErr != nil {
		fmt.Println("%v", exitErr)
	}
}
