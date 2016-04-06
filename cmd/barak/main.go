package main

// A note on the origin of the name.
// http://en.wikipedia.org/wiki/Barak
// TODO: Nonrepudiable command log

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"time"

	. "github.com/eris-ltd/tendermint/cmd/barak/types"
	. "github.com/eris-ltd/tendermint/common"
	cfg "github.com/eris-ltd/tendermint/config"
	pcm "github.com/eris-ltd/tendermint/process"
	"github.com/eris-ltd/tendermint/rpc/server"
	"github.com/eris-ltd/tendermint/wire"
)

const BarakVersion = "0.0.1"

var Routes map[string]*rpcserver.RPCFunc

func init() {
	Routes = map[string]*rpcserver.RPCFunc{
		"status": rpcserver.NewRPCFunc(Status, []string{}),
		"run":    rpcserver.NewRPCFunc(Run, []string{"auth_command"}),
		// NOTE: also, two special non-JSONRPC routes called "download" and "upload"
	}
}

// Global instance
var barak_ *Barak

// Parse command-line options
func parseFlags() (optionsFile string) {
	flag.StringVar(&optionsFile, "config", "", "Read config from file instead of stdin")
	flag.Parse()
	return
}

func main() {
	fmt.Printf("New Barak Process (PID: %d)\n", os.Getpid())

	// Apply bare tendermint/* configuration.
	config := cfg.NewMapConfig(nil)
	config.Set("log_level", "info")
	cfg.ApplyConfig(config)

	// Read options
	optionsFile := parseFlags()
	options := ReadBarakOptions(optionsFile)

	// Init barak
	barak_ = NewBarakFromOptions(options)
	barak_.StartRegisterRoutine()
	barak_.WritePidFile() // This should be last, before TrapSignal().
	TrapSignal(func() {
		fmt.Println("Barak shutting down")
	})
}

//------------------------------------------------------------------------------
// RPC functions

func Status() (*ResponseStatus, error) {
	barak_.mtx.Lock()
	pid := barak_.pid
	nonce := barak_.nonce
	validators := barak_.validators
	barak_.mtx.Unlock()

	return &ResponseStatus{
		Version:    BarakVersion,
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
	log.Notice(Fmt("Run() received command %v:%v", reflect.TypeOf(command), command))
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
	case CommandQuit:
		return Quit()
	default:
		return nil, errors.New("Invalid endpoint for command")
	}
}

func parseValidateCommandStr(authCommandStr string) (Command, error) {
	var err error
	authCommand := wire.ReadJSON(AuthCommand{}, []byte(authCommandStr), &err).(AuthCommand)
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
	if !validate([]byte(commandJSONStr), barak_.ListValidators(), signatures) {
		fmt.Printf("Failed validation attempt")
		return nil, errors.New("Validation error")
	}
	// Parse command
	var err error
	command := wire.ReadJSON(NoncedCommand{}, []byte(commandJSONStr), &err).(NoncedCommand)
	if err != nil {
		fmt.Printf("Failed to parse command")
		return nil, errors.New("Command parse error")
	}
	// Prevent replays
	barak_.CheckIncrNonce(command.Nonce)
	return command.Command, nil
}

//------------------------------------------------------------------------------
// RPC base commands
// WARNING Not validated, do not export to routes.

func StartProcess(wait bool, label string, execPath string, args []string, input string) (*ResponseStartProcess, error) {
	// First, see if there already is a process labeled 'label'
	existing := barak_.GetProcess(label)
	if existing != nil && existing.EndTime.IsZero() {
		return nil, fmt.Errorf("Process already exists: %v", label)
	}

	// Otherwise, create one.
	err := EnsureDir(barak_.RootDir() + "/outputs")
	if err != nil {
		return nil, fmt.Errorf("Failed to create outputs dir: %v", err)
	}
	inFile := bytes.NewReader([]byte(input))
	outPath := Fmt("%v/outputs/%v_%v.out", barak_.RootDir(), label, time.Now().Format("2006_01_02_15_04_05_MST"))
	outFile, err := OpenAutoFile(outPath)
	if err != nil {
		return nil, err
	}
	proc, err := pcm.Create(label, execPath, args, inFile, outFile)
	if err != nil {
		return nil, err
	}
	barak_.AddProcess(label, proc)

	if wait {
		<-proc.WaitCh

		// read output from outPath
		outputBytes, err := ioutil.ReadFile(outPath)
		if err != nil {
			fmt.Sprintf("ERROR READING OUTPUT: %v", err)
		}
		output := string(outputBytes)

		// fmt.Println("Read output", output)
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
	err := barak_.StopProcess(label, kill)
	return &ResponseStopProcess{}, err
}

func ListProcesses() (*ResponseListProcesses, error) {
	procs := barak_.ListProcesses()
	return &ResponseListProcesses{
		Processes: procs,
	}, nil
}

func OpenListener(addr string) (*ResponseOpenListener, error) {
	listener, err := barak_.OpenListener(addr)
	if err != nil {
		return nil, err
	}
	return &ResponseOpenListener{
		Addr: listener.Addr().String(),
	}, nil
}

func CloseListener(addr string) (*ResponseCloseListener, error) {
	barak_.CloseListener(addr)
	return &ResponseCloseListener{}, nil
}

func Quit() (*ResponseQuit, error) {
	fmt.Println("Barak shutting down due to Quit()")
	go func() {
		time.Sleep(time.Second)
		os.Exit(0)
	}()
	return &ResponseQuit{}, nil
}

//--------------------------------------------------------------------------------

// Another barak instance registering its external
// address to a remote barak.
func RegisterHandler(w http.ResponseWriter, req *http.Request) {
	registry, err := os.OpenFile(barak_.RootDir()+"/registry.log", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
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
		proc := barak_.GetProcess(path)
		if proc == nil {
			http.Error(w, Fmt("Unknown process label: %v", path), 400)
			return
		}
		path = proc.OutputFile.(*os.File).Name()
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
