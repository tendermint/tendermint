package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/cmd/barak/types"
	. "github.com/tendermint/tendermint/common"
	pcm "github.com/tendermint/tendermint/process"
	"github.com/tendermint/tendermint/rpc/server"
)

type BarakOptions struct {
	Validators    []Validator
	ListenAddress string
	StartNonce    uint64
	Registries    []string
}

// Read options from a file, or stdin if optionsFile is ""
func ReadBarakOptions(optFile string) *BarakOptions {
	var optBytes []byte
	var err error
	if optFile != "" {
		optBytes, err = ioutil.ReadFile(optFile)
	} else {
		optBytes, err = ioutil.ReadAll(os.Stdin)
	}
	if err != nil {
		panic(Fmt("Error reading input: %v", err))
	}
	opt := binary.ReadJSON(&BarakOptions{}, optBytes, &err).(*BarakOptions)
	if err != nil {
		panic(Fmt("Error parsing input: %v", err))
	}
	return opt
}

func ensureRootDir() (rootDir string) {
	rootDir = os.Getenv("BRKROOT")
	if rootDir == "" {
		rootDir = os.Getenv("HOME") + "/.barak"
	}
	err := EnsureDir(rootDir)
	if err != nil {
		panic(Fmt("Error creating barak rootDir: %v", err))
	}
	return
}

func NewBarakFromOptions(opt *BarakOptions) *Barak {
	rootDir := ensureRootDir()
	barak := NewBarak(rootDir, opt.StartNonce, opt.Validators)
	for _, registry := range opt.Registries {
		barak.AddRegistry(registry)
	}
	barak.OpenListener(opt.ListenAddress)

	// Debug.
	fmt.Printf("Options: %v\n", opt)
	fmt.Printf("Barak: %v\n", barak)
	return barak
}

//--------------------------------------------------------------------------------

type Barak struct {
	mtx        sync.Mutex
	pid        int
	nonce      uint64
	processes  map[string]*pcm.Process
	validators []Validator
	listeners  []net.Listener
	rootDir    string
	registries []string
}

func NewBarak(rootDir string, nonce uint64, validators []Validator) *Barak {
	return &Barak{
		pid:        os.Getpid(),
		nonce:      nonce,
		processes:  make(map[string]*pcm.Process),
		validators: validators,
		listeners:  nil,
		rootDir:    rootDir,
		registries: nil,
	}
}

func (brk *Barak) RootDir() string {
	brk.mtx.Lock()
	defer brk.mtx.Unlock()
	return brk.rootDir
}

func (brk *Barak) ListProcesses() []*pcm.Process {
	brk.mtx.Lock()
	defer brk.mtx.Unlock()
	processes := []*pcm.Process{}
	for _, process := range brk.processes {
		processes = append(processes, process)
	}
	return processes
}

func (brk *Barak) GetProcess(label string) *pcm.Process {
	brk.mtx.Lock()
	defer brk.mtx.Unlock()
	return brk.processes[label]
}

func (brk *Barak) AddProcess(label string, process *pcm.Process) error {
	brk.mtx.Lock()
	defer brk.mtx.Unlock()
	existing := brk.processes[label]
	if existing != nil && existing.EndTime.IsZero() {
		return fmt.Errorf("Process already exists: %v", label)
	}
	brk.processes[label] = process
	return nil
}

func (brk *Barak) StopProcess(label string, kill bool) error {
	brk.mtx.Lock()
	proc := brk.processes[label]
	brk.mtx.Unlock()

	if proc == nil {
		return fmt.Errorf("Process does not exist: %v", label)
	}

	err := pcm.Stop(proc, kill)
	return err
}

func (brk *Barak) ListValidators() []Validator {
	brk.mtx.Lock()
	defer brk.mtx.Unlock()
	return brk.validators
}

func (brk *Barak) ListListeners() []net.Listener {
	brk.mtx.Lock()
	defer brk.mtx.Unlock()
	return brk.listeners
}

func (brk *Barak) OpenListener(addr string) (net.Listener, error) {
	brk.mtx.Lock()
	defer brk.mtx.Unlock()
	// Start rpc server.
	mux := http.NewServeMux()
	mux.HandleFunc("/download", ServeFileHandler)
	mux.HandleFunc("/register", RegisterHandler)
	// TODO: mux.HandleFunc("/upload", UploadFile)
	rpcserver.RegisterRPCFuncs(mux, Routes)
	listener, err := rpcserver.StartHTTPServer(addr, mux)
	if err != nil {
		return nil, err
	}
	brk.listeners = append(brk.listeners, listener)
	return listener, nil
}

func (brk *Barak) CloseListener(addr string) {
	brk.mtx.Lock()
	defer brk.mtx.Unlock()
	filtered := []net.Listener{}
	for _, listener := range brk.listeners {
		if listener.Addr().String() == addr {
			continue
		}
		filtered = append(filtered, listener)
	}
	brk.listeners = filtered
}

func (brk *Barak) GetRegistries() []string {
	brk.mtx.Lock()
	defer brk.mtx.Unlock()
	return brk.registries
}

func (brk *Barak) AddRegistry(registry string) {
	brk.mtx.Lock()
	defer brk.mtx.Unlock()
	brk.registries = append(brk.registries, registry)
}

func (brk *Barak) RemoveRegistry(registry string) {
	brk.mtx.Lock()
	defer brk.mtx.Unlock()
	filtered := []string{}
	for _, reg := range brk.registries {
		if registry == reg {
			continue
		}
		filtered = append(filtered, reg)
	}
	brk.registries = filtered
}

func (brk *Barak) StartRegisterRoutine() {
	// Register this barak with central listener
	go func() {
		// Workaround around issues when registries register on themselves upon startup.
		time.Sleep(3 * time.Second)
		for {
			// Every hour, register with the registries.
			for _, registry := range brk.registries {
				resp, err := http.Get(registry + "/register")
				if err != nil {
					fmt.Printf("Error registering to registry %v:\n  %v\n", registry, err)
				} else if resp.StatusCode != 200 {
					body, _ := ioutil.ReadAll(resp.Body)
					fmt.Printf("Error registering to registry %v:\n  %v\n", registry, string(body))
				} else {
					body, _ := ioutil.ReadAll(resp.Body)
					fmt.Printf("Successfully registered with registry %v\n  %v\n", registry, string(body))
				}
			}
			time.Sleep(1 * time.Hour)
		}
	}()
}

// Write pid to file.
func (brk *Barak) WritePidFile() {
	err := WriteFileAtomic(brk.rootDir+"/pidfile", []byte(Fmt("%v", brk.pid)))
	if err != nil {
		panic(Fmt("Error writing pidfile: %v", err))
	}
}

func (brk *Barak) CheckIncrNonce(newNonce uint64) error {
	brk.mtx.Lock()
	defer brk.mtx.Unlock()
	if brk.nonce+1 != newNonce {
		return errors.New("Replay error")
	}
	brk.nonce += 1
	return nil
}
