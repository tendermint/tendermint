package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sync"

	acm "github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	pcm "github.com/tendermint/tendermint/process"
	"github.com/tendermint/tendermint/rpc"
)

var Routes = map[string]*rpc.RPCFunc{
	"RunProcess":    rpc.NewRPCFunc(RunProcess, []string{"wait", "label", "execPath", "args"}),
	"ListProcesses": rpc.NewRPCFunc(ListProcesses, []string{}),
	"StopProcess":   rpc.NewRPCFunc(StopProcess, []string{"label", "kill"}),
	// NOTE: also, two special non-JSONRPC routes called
	// "download" and "upload".
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
	mux := http.NewServeMux()
	mux.HandleFunc("/download", ServeFile)
	// TODO: mux.HandleFunc("/upload", UploadFile)
	rpc.RegisterRPCFuncs(mux, Routes)
	rpc.StartHTTPServer(options.ListenAddress, mux)

	TrapSignal(func() {
		fmt.Println("Debora shutting down")
	})
}

//------------------------------------------------------------------------------
// RPC functions

type ResponseRunProcess struct {
}

func RunProcess(wait bool, label string, execPath string, args []string, input string) (*ResponseRunProcess, error) {
	debora.mtx.Lock()

	// First, see if there already is a process labeled 'label'
	existing := debora.processes[label]
	if existing != nil {
		debora.mtx.Unlock()
		return nil, Errorf("Process already exists: %v", label)
	}

	// Otherwise, create one.
	proc := pcm.Create(pcm.ProcessModeDaemon, label, execPath, args, input)
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

//------------------------------------------------------------------------------

func ServeFile(w http.ResponseWriter, req *http.Request) {
	path := req.FormValue("path")
	if path == "" {
		http.Error(w, "Must specify path", 400)
		return
	}
	file, err := os.Open(path)
	if err != nil {
		http.Error(w, Fmt("Error opening file: %v. %v", path, err), 400)
		return
	}
	_, err = io.Copy(w, file)
	if err != nil {
		fmt.Fprintf(os.Stderr, Fmt("Error serving file: %v. %v", path, err))
		return
	}
}
