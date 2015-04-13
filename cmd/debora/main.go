package main

// TODO: Nonrepudiable command log

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"sync"

	acm "github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	pcm "github.com/tendermint/tendermint/process"
	"github.com/tendermint/tendermint/rpc"
)

var Routes = map[string]*rpc.RPCFunc{
	"run_auth_command": rpc.NewRPCFunc(Run, []string{"auth_command"}),
	// NOTE: also, two special non-JSONRPC routes called "download" and "upload"
}

type Options struct {
	Validators    []Validator
	ListenAddress string
	StartNonce    uint64
}

// Global instance
var debora = struct {
	mtx        sync.Mutex
	processes  map[string]*pcm.Process
	validators []Validator
	nonce      uint64
}{sync.Mutex{}, make(map[string]*pcm.Process), nil, 0}

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
	debora.nonce = options.StartNonce
	debora.validators = options.Validators

	// Debug.
	fmt.Printf("Options: %v\n", options)
	fmt.Printf("Debora: %v\n", debora)

	// Start rpc server.
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
// RPC main function

func Run(authCommand AuthCommand) (interface{}, error) {
	command, err := parseValidateCommand(authCommand)
	if err != nil {
		return nil, err
	}
	log.Info(Fmt("Run() received command %v", reflect.TypeOf(command)))
	// Issue command
	switch c := command.(type) {
	case CommandRunProcess:
		return RunProcess(c.Wait, c.Label, c.ExecPath, c.Args, c.Input)
	case CommandStopProcess:
		return StopProcess(c.Label, c.Kill)
	case CommandListProcesses:
		return ListProcesses()
	default:
		return nil, errors.New("Invalid endpoint for command")
	}
}

func parseValidateCommandStr(authCommandStr string) (Command, error) {
	var err error
	authCommand := binary.ReadJSON(AuthCommand{}, []byte(authCommandStr), &err).(AuthCommand)
	if err != nil {
		fmt.Printf("Failed to parse auth_command")
		return nil, errors.New("AuthCommand parse error")
	}
	return parseValidateCommand(authCommand)
}

func parseValidateCommand(authCommand AuthCommand) (Command, error) {
	commandJSONStr := authCommand.CommandJSONStr
	signatures := authCommand.Signatures
	// Validate commandJSONStr
	if !validate([]byte(commandJSONStr), debora.validators, signatures) {
		fmt.Printf("Failed validation attempt")
		return nil, errors.New("Validation error")
	}
	// Parse command
	var err error
	command := binary.ReadJSON(NoncedCommand{}, []byte(commandJSONStr), &err).(NoncedCommand)
	if err != nil {
		fmt.Printf("Failed to parse command")
		return nil, errors.New("Command parse error")
	}
	// Prevent replays
	if debora.nonce+1 != command.Nonce {
		return nil, errors.New("Replay error")
	} else {
		debora.nonce += 1
	}
	return command.Command, nil
}

type AuthCommand struct {
	CommandJSONStr string
	Signatures     []acm.Signature
}

type NoncedCommand struct {
	Nonce uint64
	Command
}

type Command interface{}

// for binary.readReflect
var _ = binary.RegisterInterface(
	struct{ Command }{},
	binary.ConcreteType{CommandRunProcess{}},
	binary.ConcreteType{CommandStopProcess{}},
	binary.ConcreteType{CommandListProcesses{}},
	binary.ConcreteType{CommandServeFile{}},
)

const (
	typeByteRunProcess    = 0x01
	typeByteStopProcess   = 0x02
	typeByteListProcesses = 0x03
	typeByteServeFile     = 0x04
)

//------------------------------------------------------------------------------
// RPC base commands
// WARNING Not validated, do not export to routes.

type CommandRunProcess struct {
	Wait     bool
	Label    string
	ExecPath string
	Args     []string
	Input    string
}

func (_ CommandRunProcess) TypeByte() byte { return typeByteRunProcess }

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

type CommandStopProcess struct {
	Label string
	Kill  bool
}

func (_ CommandStopProcess) TypeByte() byte { return typeByteStopProcess }

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

//--------------------------------------

type CommandListProcesses struct{}

func (_ CommandListProcesses) TypeByte() byte { return typeByteListProcesses }

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

//------------------------------------------------------------------------------

type CommandServeFile struct {
	Path string
}

func (_ CommandServeFile) TypeByte() byte { return typeByteServeFile }

func ServeFile(w http.ResponseWriter, req *http.Request) {

	authCommandStr := req.FormValue("auth_command")
	command, err := parseValidateCommandStr(authCommandStr)
	if err != nil {
		http.Error(w, Fmt("Invalid command: %v", err), 400)
	}
	serveCommand, ok := command.(CommandServeFile)
	if !ok {
		http.Error(w, "Invalid command", 400)
	}
	path := serveCommand.Path
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
