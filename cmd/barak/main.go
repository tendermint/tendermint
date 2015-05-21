package main

// A note on the origin of the name.
// http://en.wikipedia.org/wiki/Barak
// TODO: Nonrepudiable command log

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/cmd/barak/types"
	. "github.com/tendermint/tendermint/common"
	cfg "github.com/tendermint/tendermint/config"
	pcm "github.com/tendermint/tendermint/process"
	"github.com/tendermint/tendermint/rpc/server"
)

var Routes map[string]*rpcserver.RPCFunc

func init() {
	Routes = map[string]*rpcserver.RPCFunc{
		"status": rpcserver.NewRPCFunc(Status, []string{}),
		"run":    rpcserver.NewRPCFunc(Run, []string{"auth_command"}),
		// NOTE: also, two special non-JSONRPC routes called "download" and "upload"
	}
}

// Global instance
var barak *Barak

// Parse command-line options
func parseFlags() (optionsFile string) {
	flag.StringVar(&optionsFile, "options-file", "", "Read options from file instead of stdin")
	flag.Parse()
	return
}

func main() {
	fmt.Printf("New Barak Process (PID: %d)\n", os.Getpid())

	// Apply bare tendermint/* configuration.
	cfg.ApplyConfig(cfg.MapConfig(map[string]interface{}{"log_level": "info"}))

	// Read options
	optionsFile := parseFlags()
	options := ReadBarakOptions(optionsFile)

	// Init barak
	barak = NewBarakFromOptions(options)
	barak.StartRegisterRoutine()
	barak.WritePidFile() // This should be last, before TrapSignal().
	TrapSignal(func() {
		fmt.Println("Barak shutting down")
	})
}

//------------------------------------------------------------------------------
// RPC functions

func Status() (*ResponseStatus, error) {
	barak.mtx.Lock()
	pid := barak.pid
	nonce := barak.nonce
	validators := barak.validators
	barak.mtx.Unlock()

	return &ResponseStatus{
		Pid:        pid,
		Nonce:      nonce,
		Validators: validators,
	}, nil
}

func Run(authCommand AuthCommand) (interface{}, error) {
	command, err := parseValidateCommand(authCommand)
	if err != nil {
		return nil, err
	}
	log.Info(Fmt("Run() received command %v:\n%v", reflect.TypeOf(command), command))
	// Issue command
	switch c := command.(type) {
	case CommandStartProcess:
		return StartProcess(c.Wait, c.Label, c.ExecPath, c.Args, c.Input)
	case CommandStopProcess:
		return StopProcess(c.Label, c.Kill)
	case CommandListProcesses:
		return ListProcesses()
	case CommandOpenListener:
		return OpenListener(c.Addr)
	case CommandCloseListener:
		return CloseListener(c.Addr)
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
	if !validate([]byte(commandJSONStr), barak.ListValidators(), signatures) {
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
	barak.CheckIncrNonce(command.Nonce)
	return command.Command, nil
}

//------------------------------------------------------------------------------
// RPC base commands
// WARNING Not validated, do not export to routes.

func StartProcess(wait bool, label string, execPath string, args []string, input string) (*ResponseStartProcess, error) {
	// First, see if there already is a process labeled 'label'
	existing := barak.GetProcess(label)
	if existing != nil && existing.EndTime.IsZero() {
		return nil, fmt.Errorf("Process already exists: %v", label)
	}

	// Otherwise, create one.
	err := EnsureDir(barak.RootDir() + "/outputs")
	if err != nil {
		return nil, fmt.Errorf("Failed to create outputs dir: %v", err)
	}
	outPath := Fmt("%v/outputs/%v_%v.out", barak.RootDir(), label, time.Now().Format("2006_01_02_15_04_05_MST"))
	proc, err := pcm.Create(pcm.ProcessModeDaemon, label, execPath, args, input, outPath)
	if err != nil {
		return nil, err
	}
	barak.AddProcess(label, proc)

	if wait {
		<-proc.WaitCh
		output := pcm.ReadOutput(proc)
		fmt.Println("Read output", output)
		if proc.ExitState == nil {
			return &ResponseStartProcess{
				Success: true,
				Output:  output,
			}, nil
		} else {
			return &ResponseStartProcess{
				Success: proc.ExitState.Success(), // Would be always false?
				Output:  output,
			}, nil
		}
	} else {
		return &ResponseStartProcess{
			Success: true,
			Output:  "",
		}, nil
	}
}

func StopProcess(label string, kill bool) (*ResponseStopProcess, error) {
	err := barak.StopProcess(label, kill)
	return &ResponseStopProcess{}, err
}

func ListProcesses() (*ResponseListProcesses, error) {
	procs := barak.ListProcesses()
	return &ResponseListProcesses{
		Processes: procs,
	}, nil
}

func OpenListener(addr string) (*ResponseOpenListener, error) {
	listener := barak.OpenListener(addr)
	return &ResponseOpenListener{
		Addr: listener.Addr().String(),
	}, nil
}

func CloseListener(addr string) (*ResponseCloseListener, error) {
	barak.CloseListener(addr)
	return &ResponseCloseListener{}, nil
}

//--------------------------------------------------------------------------------

// Another barak instance registering its external
// address to a remote barak.
func RegisterHandler(w http.ResponseWriter, req *http.Request) {
	registry, err := os.OpenFile(barak.RootDir()+"/registry.log", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		http.Error(w, "Could not open registry file. Please contact the administrator", 500)
		return
	}
	// TODO: Also check the X-FORWARDED-FOR or whatever it's called.
	registry.Write([]byte(Fmt("++ %v\n", req.RemoteAddr)))
	registry.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write([]byte("Noted!"))
}

func ServeFileHandler(w http.ResponseWriter, req *http.Request) {
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
	if path[0] == '.' {
		// local paths must be explicitly local, e.g. "./xyz"
	} else if path[0] != '/' {
		// If not an absolute path, then is label
		proc := barak.GetProcess(path)
		if proc == nil {
			http.Error(w, Fmt("Unknown process label: %v", path), 400)
			return
		}
		path = proc.OutputPath
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
