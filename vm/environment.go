package vm

import (
	"errors"
)

const (
	defaultDataStackCapacity = 10
)

var (
	ErrCallStackOverflow  = errors.New("CallStackOverflow")
	ErrCallStackUnderflow = errors.New("CallStackUnderflow")
	ErrInsufficientGas    = errors.New("InsufficientGas")
)

type AppState interface {

	// Accounts
	GetAccount([]byte) *Account
	UpdateAccount(*Account)

	// Storage
	GetStorage([]byte, []byte)
	UpdateStorage([]byte, []byte)
	RemoveStorage([]byte)

	// Logs
	AddLog(*Log)
}

type VMCall struct {
	caller    *Account
	target    *Account
	code      []byte
	gasLimit  uint64
	gasUsed   uint64
	dataStack *Stack
	memory    *Memory
}

func (vmCall *VMCall) gasLeft() uint {
	return vmCall.gasLimit - vmCall.gasUsed
}

type VMParams struct {
	BlockHeight    uint
	BlockHash      []byte
	BlockTime      int64
	GasLimit       uint64
	GasPrice       uint64
	CallStackLimit uint
	Origin         []byte
}

//-----------------------------------------------------------------------------

type VMEnvironment struct {
	params    VMParams
	appState  AppState
	callStack []*VMCall
	lastCall  *VMCall
}

func NewVMEnvironment(appState AppState, params VMParams) *VMEnvironment {
	return &VMEnvironment{
		params:    params,
		appState:  appState,
		callStack: make([]*VMCall, 0, params.CallStackLimit),
		lastCall:  nil,
	}
}

// XXX I think this is all wrong.
// Begin a new transaction (root call)
func (env *VMEnvironment) SetupTransaction(caller, target *Account, gasLimit, value uint64, input []byte) error {
	// TODO charge gas for transaction
	var gasUsed uint64 = 0
	return env.setupCall(caller, target, gasUsed, gasLimit, value, input)
}

// XXX I think this is all wrong.
// env.lastCall.target (env.callStack[-1]) is calling target.
func (env *VMEnvironment) SetupCall(target *Account, gasLimit, value uint64, input []byte) error {

	// Gas check
	if env.lastCall.gasLeft() < gasLimit {
		return ErrInsufficientGas
	}

	// Depth check
	if len(env.callStack) == env.params.CallStackLimit {
		return ErrCallStackOverflow
	}

	var gasUsed uint64 = 0
	var caller = env.lastCall.target
	return env.setupCall(caller, target, gasUsed, gasLimit, value, input)
}

// XXX I think this is all wrong.
func (env *VMEnvironment) setupCall(caller, target *Account, gasUsed, gasLimit uint64, input []byte) error {

	// Incr nonces
	caller.IncrNonce()

	// TODO Charge TX and data gas

	// Value transfer
	if value != 0 {
		// TODO Charge for gas
		err := caller.SubBalance(value)
		if err != nil {
			return err
		}

		err = target.AddBalance(value)
		if err != nil {
			return err
		}
	}

	// Create new VMCall
	vmCall := &VMCall{
		caller:    caller,
		target:    target,
		code:      target.Code(),
		gasLimit:  gasLimit,
		gasUsed:   gasUsed,
		dataStack: NewStack(defaultDataStackCapacity),
		memory:    NewMemory(),
	}
	env.callStack = append(env.callStack, vmCall)
	env.lastCall = vmCall

	return nil
}

func (env *VMEnvironment) CallStackDepth() int {
	return len(env.callStack)
}
