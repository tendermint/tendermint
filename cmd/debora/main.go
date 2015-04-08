package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	acm "github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	pcm "github.com/tendermint/tendermint/process"
	rpc "github.com/tendermint/tendermint/rpc"
)

var Routes = map[string]*rpc.RPCFunc{
	"RunProcess":    rpc.NewRPCFunc(RunProcess, []string{"wait", "label", "execPath", "args"}),
	"ListProcesses": rpc.NewRPCFunc(ListProcesses, []string{}),
	"StopProcess":   rpc.NewRPCFunc(StopProcess, []string{"label", "kill"}),
}

type Validator struct {
	VotingPower uint64
	PubKey      acm.PubKey
}

type Options struct {
	Validators    []Validator
	ListenAddress string
}

// Global instance
var debora = struct {
	mtx       sync.Mutex
	processes map[string]*pcm.Process
}{sync.Mutex{}, make(map[string]*pcm.Process)}

func main() {

	// read options from stdin.
	var err error
	optionsBytes, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		panic(Fmt("Error reading input: %v", err))
	}
	options := binary.ReadJSON(&Options{}, optionsBytes, &err).(*Options)
	if err != nil {
		panic(Fmt("Error parsing input: %v", err))
	}

	// Debug.
	fmt.Printf("Validators: %v\n", options.Validators)

	// start rpc server.
	fmt.Println("Listening HTTP-JSONRPC on", options.ListenAddress)
	rpc.StartHTTPServer(options.ListenAddress, Routes, nil)

	TrapSignal(func() {
		fmt.Println("Debora shutting down")
	})
}

//------------------------------------------------------------------------------
// RPC functions

type ResponseRunProcess struct {
}

func RunProcess(wait bool, label string, execPath string, args []string) (*ResponseRunProcess, error) {
	debora.mtx.Lock()

	// First, see if there already is a process labeled 'label'
	existing := debora.processes[label]
	if existing != nil {
		debora.mtx.Unlock()
		return nil, Errorf("Process already exists: %v", label)
	}

	// Otherwise, create one.
	proc := pcm.Create(pcm.ProcessModeDaemon, label, execPath, args...)
	debora.processes[label] = proc
	debora.mtx.Unlock()

	if wait {
		exitErr := pcm.Wait(proc)
		return nil, exitErr
	} else {
		return &ResponseRunProcess{}, nil
	}
}

//--------------------------------------

type ResponseListProcesses struct {
	Processes []*pcm.Process
}

func ListProcesses() (*ResponseListProcesses, error) {
	var procs = []*pcm.Process{}
	debora.mtx.Lock()
	for _, proc := range debora.processes {
		procs = append(procs, proc)
	}
	debora.mtx.Unlock()

	return &ResponseListProcesses{
		Processes: procs,
	}, nil
}

//--------------------------------------

type ResponseStopProcess struct {
}

func StopProcess(label string, kill bool) (*ResponseStopProcess, error) {
	debora.mtx.Lock()
	proc := debora.processes[label]
	debora.mtx.Unlock()

	if proc == nil {
		return nil, Errorf("Process does not exist: %v", label)
	}

	err := pcm.Stop(proc, kill)
	return &ResponseStopProcess{}, err
}
