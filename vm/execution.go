package vm

import (
	"math/big"
	"time"

	"github.com/tendermint/tendermint/state"
)

type Execution struct {
	env               Environment
	address, input    []byte
	Gas, price, value *big.Int
}

func NewExecution(env Environment, address, input []byte, gas, gasPrice, value *big.Int) *Execution {
	return &Execution{env: env, address: address, input: input, Gas: gas, price: gasPrice, value: value}
}

func (self *Execution) Addr() []byte {
	return self.address
}

func (self *Execution) Call(codeAddr []byte, caller ContextRef) ([]byte, error) {
	// Retrieve the executing code
	code := self.env.State().GetCode(codeAddr)

	return self.exec(code, codeAddr, caller)
}

func (self *Execution) exec(code, contextAddr []byte, caller ContextRef) (ret []byte, err error) {
	env := self.env
	evm := NewVm(env)
	if env.Depth() == MaxCallDepth {
		caller.ReturnGas(self.Gas, self.price)

		return nil, DepthError{}
	}

	vsnapshot := env.State().Copy()
	if len(self.address) == 0 {
		// Generate a new address
		nonce := env.State().GetNonce(caller.Address())
		self.address = crypto.CreateAddress(caller.Address(), nonce)
		env.State().SetNonce(caller.Address(), nonce+1)
	}

	from, to := env.State().GetStateObject(caller.Address()), env.State().GetOrNewStateObject(self.address)
	err = env.Transfer(from, to, self.value)
	if err != nil {
		env.State().Set(vsnapshot)

		caller.ReturnGas(self.Gas, self.price)

		return nil, ValueTransferErr("insufficient funds to transfer value. Req %v, has %v", self.value, from.Balance())
	}

	snapshot := env.State().Copy()
	start := time.Now()
	ret, err = evm.Run(to, caller, code, self.value, self.Gas, self.price, self.input)
	chainlogger.Debugf("vm took %v\n", time.Since(start))
	if err != nil {
		env.State().Set(snapshot)
	}

	return
}

func (self *Execution) Create(caller ContextRef) (ret []byte, err error, account *state.StateObject) {
	ret, err = self.exec(self.input, nil, caller)
	account = self.env.State().GetStateObject(self.address)

	return
}
