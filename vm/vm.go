package vm

import (
	"errors"
	"fmt"
	"math/big"

	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/events"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/vm/sha3"
)

var (
	ErrUnknownAddress      = errors.New("Unknown address")
	ErrInsufficientBalance = errors.New("Insufficient balance")
	ErrInvalidJumpDest     = errors.New("Invalid jump dest")
	ErrInsufficientGas     = errors.New("Insuffient gas")
	ErrMemoryOutOfBounds   = errors.New("Memory out of bounds")
	ErrCodeOutOfBounds     = errors.New("Code out of bounds")
	ErrInputOutOfBounds    = errors.New("Input out of bounds")
	ErrCallStackOverflow   = errors.New("Call stack overflow")
	ErrCallStackUnderflow  = errors.New("Call stack underflow")
	ErrDataStackOverflow   = errors.New("Data stack overflow")
	ErrDataStackUnderflow  = errors.New("Data stack underflow")
	ErrInvalidContract     = errors.New("Invalid contract")
)

type Debug bool

const (
	dataStackCapacity       = 1024
	callStackCapacity       = 100         // TODO ensure usage.
	memoryCapacity          = 1024 * 1024 // 1 MB
	dbg               Debug = true
)

func (d Debug) Printf(s string, a ...interface{}) {
	if d {
		fmt.Printf(s, a...)
	}
}

type VM struct {
	appState AppState
	params   Params
	origin   Word256
	txid     []byte

	callDepth int

	evsw *events.EventSwitch
}

func NewVM(appState AppState, params Params, origin Word256, txid []byte) *VM {
	return &VM{
		appState:  appState,
		params:    params,
		origin:    origin,
		callDepth: 0,
		txid:      txid,
	}
}

// satisfies events.Eventable
func (vm *VM) SetEventSwitch(evsw *events.EventSwitch) {
	vm.evsw = evsw
}

// CONTRACT appState is aware of caller and callee, so we can just mutate them.
// value: To be transferred from caller to callee. Refunded upon error.
// gas:   Available gas. No refunds for gas.
func (vm *VM) Call(caller, callee *Account, code, input []byte, value uint64, gas *uint64) (output []byte, err error) {

	if len(code) == 0 {
		panic("Call() requires code")
	}

	if err = transfer(caller, callee, value); err != nil {
		return
	}

	vm.callDepth += 1
	output, err = vm.call(caller, callee, code, input, value, gas)
	vm.callDepth -= 1
	exception := ""
	if err != nil {
		exception = err.Error()
		err := transfer(callee, caller, value)
		if err != nil {
			panic("Could not return value to caller")
		}
	}
	// if callDepth is 0 the event is fired from ExecTx (along with the Input event)
	// otherwise, we fire from here.
	if vm.callDepth != 0 && vm.evsw != nil {
		vm.evsw.FireEvent(types.EventStringAccReceive(callee.Address.Prefix(20)), types.EventMsgCall{
			&types.CallData{caller.Address.Prefix(20), callee.Address.Prefix(20), input, value, *gas},
			vm.origin.Prefix(20),
			vm.txid,
			output,
			exception,
		})
	}
	return
}

// Just like Call() but does not transfer 'value' or modify the callDepth.
func (vm *VM) call(caller, callee *Account, code, input []byte, value uint64, gas *uint64) (output []byte, err error) {
	dbg.Printf("(%d) (%X) %X (code=%d) gas: %v (d) %X\n", vm.callDepth, caller.Address[:4], callee.Address, len(callee.Code), *gas, input)

	var (
		pc     uint64 = 0
		stack         = NewStack(dataStackCapacity, gas, &err)
		memory        = make([]byte, memoryCapacity)
		ok            = false // convenience
	)

	for {
		// If there is an error, return
		if err != nil {
			return nil, err
		}

		var op = codeGetOp(code, pc)
		dbg.Printf("(pc) %-3d (op) %-14s (st) %-4d ", pc, op.String(), stack.Len())

		switch op {

		case STOP: // 0x00
			return nil, nil

		case ADD: // 0x01
			//x, y := stack.Pop64(), stack.Pop64()
			//stack.Push64(x + y)
			x, y := stack.Pop(), stack.Pop()
			xb := new(big.Int).SetBytes(flip(x[:]))
			yb := new(big.Int).SetBytes(flip(y[:]))
			sum := new(big.Int).Add(xb, yb)
			stack.Push(RightPadWord256(flip(sum.Bytes())))
			dbg.Printf(" %v + %v = %v\n", xb, yb, sum)

		case MUL: // 0x02
			//x, y := stack.Pop64(), stack.Pop64()
			//stack.Push64(x * y)
			x, y := stack.Pop(), stack.Pop()
			xb := new(big.Int).SetBytes(flip(x[:]))
			yb := new(big.Int).SetBytes(flip(y[:]))
			prod := new(big.Int).Mul(xb, yb)
			stack.Push(RightPadWord256(flip(prod.Bytes())))
			dbg.Printf(" %v * %v = %v\n", xb, yb, prod)

		case SUB: // 0x03
			//x, y := stack.Pop64(), stack.Pop64()
			//stack.Push64(x - y)
			x, y := stack.Pop(), stack.Pop()
			xb := new(big.Int).SetBytes(flip(x[:]))
			yb := new(big.Int).SetBytes(flip(y[:]))
			diff := new(big.Int).Sub(xb, yb)
			stack.Push(RightPadWord256(flip(diff.Bytes())))
			dbg.Printf(" %v - %v = %v\n", xb, yb, diff)

		case DIV: // 0x04
			//x, y := stack.Pop64(), stack.Pop64()
			//stack.Push64(x / y)
			x, y := stack.Pop(), stack.Pop()
			if y.IsZero() { // TODO
				stack.Push(Zero256)
				dbg.Printf(" %x / %x = %v (TODO)\n", x, y, 0)
			} else {
				xb := new(big.Int).SetBytes(flip(x[:]))
				yb := new(big.Int).SetBytes(flip(y[:]))
				div := new(big.Int).Div(xb, yb)
				stack.Push(RightPadWord256(flip(div.Bytes())))
				dbg.Printf(" %v / %v = %v\n", xb, yb, div)
			}

		case SDIV: // 0x05
			// TODO ... big?
			x, y := int64(stack.Pop64()), int64(stack.Pop64())
			if y == 0 { // TODO
				stack.Push(Zero256)
				dbg.Printf(" %v / %v = %v (TODO)\n", x, y, 0)
			} else {
				stack.Push64(uint64(x / y))
				dbg.Printf(" %v / %v = %v\n", x, y, x/y)
			}

		case MOD: // 0x06
			//x, y := stack.Pop64(), stack.Pop64()
			x, y := stack.Pop(), stack.Pop()
			if y.IsZero() { // TODO
				stack.Push(Zero256)
				dbg.Printf(" %v %% %v = %v (TODO)\n", x, y, 0)
			} else {
				xb := new(big.Int).SetBytes(flip(x[:]))
				yb := new(big.Int).SetBytes(flip(y[:]))
				mod := new(big.Int).Mod(xb, yb)
				stack.Push(RightPadWord256(flip(mod.Bytes())))
				dbg.Printf(" %v %% %v = %v\n", xb, yb, mod)
			}

		case SMOD: // 0x07
			// TODO ... big?
			x, y := int64(stack.Pop64()), int64(stack.Pop64())
			if y == 0 { // TODO
				stack.Push(Zero256)
				dbg.Printf(" %v %% %v = %v (TODO)\n", x, y, 0)
			} else {
				stack.Push64(uint64(x % y))
				dbg.Printf(" %v %% %v = %v\n", x, y, x%y)
			}

		case ADDMOD: // 0x08
			// TODO ... big?
			x, y, z := stack.Pop64(), stack.Pop64(), stack.Pop64()
			if z == 0 { // TODO
				stack.Push(Zero256)
				dbg.Printf(" (%v + %v) %% %v = %v (TODO)\n", x, y, z, 0)
			} else {
				stack.Push64((x + y) % z)
				dbg.Printf(" (%v + %v) %% %v = %v\n", x, y, z, (x+y)%z)
			}

		case MULMOD: // 0x09
			// TODO ... big?
			x, y, z := stack.Pop64(), stack.Pop64(), stack.Pop64()
			if z == 0 { // TODO
				stack.Push(Zero256)
				dbg.Printf(" (%v + %v) %% %v = %v (TODO)\n", x, y, z, 0)
			} else {
				stack.Push64((x * y) % z)
				dbg.Printf(" (%v + %v) %% %v = %v\n", x, y, z, (x*y)%z)
			}

		case EXP: // 0x0A
			//x, y := stack.Pop64(), stack.Pop64()
			//stack.Push64(ExpUint64(x, y))
			x, y := stack.Pop(), stack.Pop()
			xb := new(big.Int).SetBytes(flip(x[:]))
			yb := new(big.Int).SetBytes(flip(y[:]))
			pow := new(big.Int).Exp(xb, yb, big.NewInt(0))
			stack.Push(RightPadWord256(flip(pow.Bytes())))
			dbg.Printf(" %v ** %v = %v\n", xb, yb, pow)

		case SIGNEXTEND: // 0x0B
			x, y := stack.Pop64(), stack.Pop64()
			res := (y << uint(x)) >> x
			stack.Push64(res)
			dbg.Printf(" (%v << %v) >> %v = %v\n", y, x, x, res)

		case LT: // 0x10
			x, y := stack.Pop64(), stack.Pop64()
			if x < y {
				stack.Push64(1)
			} else {
				stack.Push(Zero256)
			}
			dbg.Printf(" %v < %v = %v\n", x, y, x < y)

		case GT: // 0x11
			x, y := stack.Pop64(), stack.Pop64()
			if x > y {
				stack.Push64(1)
			} else {
				stack.Push(Zero256)
			}
			dbg.Printf(" %v > %v = %v\n", x, y, x > y)

		case SLT: // 0x12
			x, y := int64(stack.Pop64()), int64(stack.Pop64())
			if x < y {
				stack.Push64(1)
			} else {
				stack.Push(Zero256)
			}
			dbg.Printf(" %v < %v = %v\n", x, y, x < y)

		case SGT: // 0x13
			x, y := int64(stack.Pop64()), int64(stack.Pop64())
			if x > y {
				stack.Push64(1)
			} else {
				stack.Push(Zero256)
			}
			dbg.Printf(" %v > %v = %v\n", x, y, x > y)

		case EQ: // 0x14
			x, y := stack.Pop64(), stack.Pop64()
			if x == y {
				stack.Push64(1)
			} else {
				stack.Push(Zero256)
			}
			dbg.Printf(" %v == %v = %v\n", x, y, x == y)

		case ISZERO: // 0x15
			x := stack.Pop64()
			if x == 0 {
				stack.Push64(1)
			} else {
				stack.Push(Zero256)
			}
			dbg.Printf(" %v == 0 = %v\n", x, x == 0)

		case AND: // 0x16
			x, y := stack.Pop64(), stack.Pop64()
			stack.Push64(x & y)
			dbg.Printf(" %v & %v = %v\n", x, y, x&y)

		case OR: // 0x17
			x, y := stack.Pop64(), stack.Pop64()
			stack.Push64(x | y)
			dbg.Printf(" %v | %v = %v\n", x, y, x|y)

		case XOR: // 0x18
			x, y := stack.Pop64(), stack.Pop64()
			stack.Push64(x ^ y)
			dbg.Printf(" %v ^ %v = %v\n", x, y, x^y)

		case NOT: // 0x19
			x := stack.Pop64()
			stack.Push64(^x)
			dbg.Printf(" !%v = %v\n", x, ^x)

		case BYTE: // 0x1A
			idx, val := stack.Pop64(), stack.Pop()
			res := byte(0)
			if idx < 32 {
				res = val[idx]
			}
			stack.Push64(uint64(res))
			dbg.Printf(" => 0x%X\n", res)

		case SHA3: // 0x20
			if ok = useGas(gas, GasSha3); !ok {
				return nil, firstErr(err, ErrInsufficientGas)
			}
			offset, size := stack.Pop64(), stack.Pop64()
			data, ok := subslice(memory, offset, size, false)
			if !ok {
				return nil, firstErr(err, ErrMemoryOutOfBounds)
			}
			data = sha3.Sha3(data)
			stack.PushBytes(flip(data))
			dbg.Printf(" => (%v) %X\n", size, data)

		case ADDRESS: // 0x30
			stack.Push(callee.Address)
			dbg.Printf(" => %X\n", callee.Address)

		case BALANCE: // 0x31
			addr := stack.Pop()
			if ok = useGas(gas, GasGetAccount); !ok {
				return nil, firstErr(err, ErrInsufficientGas)
			}
			acc := vm.appState.GetAccount(addr) // TODO ensure that 20byte lengths are supported.
			if acc == nil {
				return nil, firstErr(err, ErrUnknownAddress)
			}
			balance := acc.Balance
			stack.Push64(balance)
			dbg.Printf(" => %v (%X)\n", balance, addr)

		case ORIGIN: // 0x32
			stack.Push(vm.origin)
			dbg.Printf(" => %X\n", vm.origin)

		case CALLER: // 0x33
			stack.Push(caller.Address)
			dbg.Printf(" => %X\n", caller.Address)

		case CALLVALUE: // 0x34
			stack.Push64(value)
			dbg.Printf(" => %v\n", value)

		case CALLDATALOAD: // 0x35
			offset := stack.Pop64()
			data, ok := subslice(input, offset, 32, true)
			if !ok {
				return nil, firstErr(err, ErrInputOutOfBounds)
			}
			stack.Push(RightPadWord256(data))
			dbg.Printf(" => 0x%X\n", data)

		case CALLDATASIZE: // 0x36
			stack.Push64(uint64(len(input)))
			dbg.Printf(" => %d\n", len(input))

		case CALLDATACOPY: // 0x37
			memOff := stack.Pop64()
			inputOff := stack.Pop64()
			length := stack.Pop64()
			data, ok := subslice(input, inputOff, length, false)
			if !ok {
				return nil, firstErr(err, ErrInputOutOfBounds)
			}
			dest, ok := subslice(memory, memOff, length, false)
			if !ok {
				return nil, firstErr(err, ErrMemoryOutOfBounds)
			}
			copy(dest, data)
			dbg.Printf(" => [%v, %v, %v] %X\n", memOff, inputOff, length, data)

		case CODESIZE: // 0x38
			l := uint64(len(code))
			stack.Push64(l)
			dbg.Printf(" => %d\n", l)

		case CODECOPY: // 0x39
			memOff := stack.Pop64()
			codeOff := stack.Pop64()
			length := stack.Pop64()
			data, ok := subslice(code, codeOff, length, false)
			if !ok {
				return nil, firstErr(err, ErrCodeOutOfBounds)
			}
			dest, ok := subslice(memory, memOff, length, false)
			if !ok {
				return nil, firstErr(err, ErrMemoryOutOfBounds)
			}
			copy(dest, data)
			dbg.Printf(" => [%v, %v, %v] %X\n", memOff, codeOff, length, data)

		case GASPRICE_DEPRECATED: // 0x3A
			stack.Push(Zero256)
			dbg.Printf(" => %X (GASPRICE IS DEPRECATED)\n")

		case EXTCODESIZE: // 0x3B
			addr := stack.Pop()
			if ok = useGas(gas, GasGetAccount); !ok {
				return nil, firstErr(err, ErrInsufficientGas)
			}
			acc := vm.appState.GetAccount(addr)
			if acc == nil {
				return nil, firstErr(err, ErrUnknownAddress)
			}
			code := acc.Code
			l := uint64(len(code))
			stack.Push64(l)
			dbg.Printf(" => %d\n", l)

		case EXTCODECOPY: // 0x3C
			addr := stack.Pop()
			if ok = useGas(gas, GasGetAccount); !ok {
				return nil, firstErr(err, ErrInsufficientGas)
			}
			acc := vm.appState.GetAccount(addr)
			if acc == nil {
				return nil, firstErr(err, ErrUnknownAddress)
			}
			code := acc.Code
			memOff := stack.Pop64()
			codeOff := stack.Pop64()
			length := stack.Pop64()
			data, ok := subslice(code, codeOff, length, false)
			if !ok {
				return nil, firstErr(err, ErrCodeOutOfBounds)
			}
			dest, ok := subslice(memory, memOff, length, false)
			if !ok {
				return nil, firstErr(err, ErrMemoryOutOfBounds)
			}
			copy(dest, data)
			dbg.Printf(" => [%v, %v, %v] %X\n", memOff, codeOff, length, data)

		case BLOCKHASH: // 0x40
			stack.Push(Zero256)
			dbg.Printf(" => 0x%X (NOT SUPPORTED)\n", stack.Peek().Bytes())

		case COINBASE: // 0x41
			stack.Push(Zero256)
			dbg.Printf(" => 0x%X (NOT SUPPORTED)\n", stack.Peek().Bytes())

		case TIMESTAMP: // 0x42
			time := vm.params.BlockTime
			stack.Push64(uint64(time))
			dbg.Printf(" => 0x%X\n", time)

		case BLOCKHEIGHT: // 0x43
			number := uint64(vm.params.BlockHeight)
			stack.Push64(number)
			dbg.Printf(" => 0x%X\n", number)

		case GASLIMIT: // 0x45
			stack.Push64(vm.params.GasLimit)
			dbg.Printf(" => %v\n", vm.params.GasLimit)

		case POP: // 0x50
			stack.Pop()
			dbg.Printf(" => %v\n", vm.params.GasLimit)

		case MLOAD: // 0x51
			offset := stack.Pop64()
			data, ok := subslice(memory, offset, 32, true)
			if !ok {
				return nil, firstErr(err, ErrMemoryOutOfBounds)
			}
			stack.Push(RightPadWord256(data))
			dbg.Printf(" => 0x%X\n", data)

		case MSTORE: // 0x52
			offset, data := stack.Pop64(), stack.Pop()
			dest, ok := subslice(memory, offset, 32, false)
			if !ok {
				return nil, firstErr(err, ErrMemoryOutOfBounds)
			}
			copy(dest, flip(data[:]))
			dbg.Printf(" => 0x%X\n", data)

		case MSTORE8: // 0x53
			offset, val := stack.Pop64(), byte(stack.Pop64()&0xFF)
			if len(memory) <= int(offset) {
				return nil, firstErr(err, ErrMemoryOutOfBounds)
			}
			memory[offset] = val
			dbg.Printf(" => [%v] 0x%X\n", offset, val)

		case SLOAD: // 0x54
			loc := stack.Pop()
			data := vm.appState.GetStorage(callee.Address, loc)
			stack.Push(data)
			dbg.Printf(" {0x%X : 0x%X}\n", loc, data)

		case SSTORE: // 0x55
			loc, data := stack.Pop(), stack.Pop()
			vm.appState.SetStorage(callee.Address, loc, data)
			useGas(gas, GasStorageUpdate)
			dbg.Printf(" {0x%X : 0x%X}\n", loc, data)

		case JUMP: // 0x56
			err = jump(code, stack.Pop64(), &pc)
			continue

		case JUMPI: // 0x57
			pos, cond := stack.Pop64(), stack.Pop64()
			if cond >= 1 {
				err = jump(code, pos, &pc)
				continue
			}
			dbg.Printf(" ~> false\n")

		case PC: // 0x58
			stack.Push64(pc)

		case MSIZE: // 0x59
			stack.Push64(uint64(len(memory)))

		case GAS: // 0x5A
			stack.Push64(*gas)
			dbg.Printf(" => %X\n", *gas)

		case JUMPDEST: // 0x5B
			dbg.Printf("\n")
			// Do nothing

		case PUSH1, PUSH2, PUSH3, PUSH4, PUSH5, PUSH6, PUSH7, PUSH8, PUSH9, PUSH10, PUSH11, PUSH12, PUSH13, PUSH14, PUSH15, PUSH16, PUSH17, PUSH18, PUSH19, PUSH20, PUSH21, PUSH22, PUSH23, PUSH24, PUSH25, PUSH26, PUSH27, PUSH28, PUSH29, PUSH30, PUSH31, PUSH32:
			a := uint64(op - PUSH1 + 1)
			codeSegment, ok := subslice(code, pc+1, a, true)
			if !ok {
				return nil, firstErr(err, ErrCodeOutOfBounds)
			}
			res := RightPadWord256(codeSegment)
			stack.Push(res)
			pc += a
			dbg.Printf(" => 0x%X\n", res)

		case DUP1, DUP2, DUP3, DUP4, DUP5, DUP6, DUP7, DUP8, DUP9, DUP10, DUP11, DUP12, DUP13, DUP14, DUP15, DUP16:
			n := int(op - DUP1 + 1)
			stack.Dup(n)
			dbg.Printf(" => [%d] 0x%X\n", n, stack.Peek().Bytes())

		case SWAP1, SWAP2, SWAP3, SWAP4, SWAP5, SWAP6, SWAP7, SWAP8, SWAP9, SWAP10, SWAP11, SWAP12, SWAP13, SWAP14, SWAP15, SWAP16:
			n := int(op - SWAP1 + 2)
			stack.Swap(n)
			dbg.Printf(" => [%d]\n", n)

		case LOG0, LOG1, LOG2, LOG3, LOG4:
			n := int(op - LOG0)
			topics := make([]Word256, n)
			offset, size := stack.Pop64(), stack.Pop64()
			for i := 0; i < n; i++ {
				topics[i] = stack.Pop()
			}
			data, ok := subslice(memory, offset, size, false)
			if !ok {
				return nil, firstErr(err, ErrMemoryOutOfBounds)
			}
			log := &Log{
				callee.Address,
				topics,
				data,
				vm.params.BlockHeight,
			}
			vm.appState.AddLog(log)
			dbg.Printf(" => %v\n", log)

		case CREATE: // 0xF0
			contractValue := stack.Pop64()
			offset, size := stack.Pop64(), stack.Pop64()
			input, ok := subslice(memory, offset, size, false)
			if !ok {
				return nil, firstErr(err, ErrMemoryOutOfBounds)
			}

			// Check balance
			if callee.Balance < contractValue {
				return nil, firstErr(err, ErrInsufficientBalance)
			}

			// TODO charge for gas to create account _ the code length * GasCreateByte

			newAccount := vm.appState.CreateAccount(callee)
			// Run the input to get the contract code.
			ret, err_ := vm.Call(callee, newAccount, input, input, contractValue, gas)
			if err_ != nil {
				stack.Push(Zero256)
			} else {
				newAccount.Code = ret // Set the code
				stack.Push(newAccount.Address)
			}

		case CALL, CALLCODE: // 0xF1, 0xF2
			gasLimit := stack.Pop64()
			addr, value := stack.Pop(), stack.Pop64()
			inOffset, inSize := stack.Pop64(), stack.Pop64()   // inputs
			retOffset, retSize := stack.Pop64(), stack.Pop64() // outputs
			dbg.Printf(" => %X\n", addr)

			// Get the arguments from the memory
			args, ok := subslice(memory, inOffset, inSize, false)
			if !ok {
				return nil, firstErr(err, ErrMemoryOutOfBounds)
			}

			// Ensure that gasLimit is reasonable
			if *gas < gasLimit {
				return nil, firstErr(err, ErrInsufficientGas)
			} else {
				*gas -= gasLimit
				// NOTE: we will return any used gas later.
			}

			// Begin execution
			var ret []byte
			var err error
			if nativeContract := nativeContracts[addr]; nativeContract != nil {
				// Native contract
				ret, err = nativeContract(args, &gasLimit)
			} else {
				// EVM contract
				if ok = useGas(gas, GasGetAccount); !ok {
					return nil, firstErr(err, ErrInsufficientGas)
				}
				// :(
				addr = RightPadWord256(flip(addr.Prefix(20)))
				acc := vm.appState.GetAccount(addr)
				if acc == nil {
					return nil, firstErr(err, ErrUnknownAddress)
				}
				if op == CALLCODE {
					ret, err = vm.Call(callee, callee, acc.Code, args, value, gas)
				} else {
					ret, err = vm.Call(callee, acc, acc.Code, args, value, gas)
				}
			}

			// Push result
			if err != nil {
				stack.Push(Zero256)
			} else {
				stack.Push(One256)
				dest, ok := subslice(memory, retOffset, retSize, false)
				if !ok {
					return nil, firstErr(err, ErrMemoryOutOfBounds)
				}
				copy(dest, ret)
			}

			// Handle remaining gas.
			*gas += gasLimit

			dbg.Printf("resume %X (%v)\n", callee.Address, gas)

		case RETURN: // 0xF3
			offset, size := stack.Pop64(), stack.Pop64()
			ret, ok := subslice(memory, offset, size, false)
			if !ok {
				return nil, firstErr(err, ErrMemoryOutOfBounds)
			}
			dbg.Printf(" => [%v, %v] (%d) 0x%X\n", offset, size, len(ret), ret)
			return ret, nil

		case SUICIDE: // 0xFF
			addr := stack.Pop()
			if ok = useGas(gas, GasGetAccount); !ok {
				return nil, firstErr(err, ErrInsufficientGas)
			}
			// TODO if the receiver is , then make it the fee.
			receiver := vm.appState.GetAccount(addr)
			if receiver == nil {
				return nil, firstErr(err, ErrUnknownAddress)
			}
			balance := callee.Balance
			receiver.Balance += balance
			vm.appState.UpdateAccount(receiver)
			vm.appState.RemoveAccount(callee)
			dbg.Printf(" => (%X) %v\n", addr[:4], balance)
			fallthrough

		default:
			dbg.Printf("(pc) %-3v Invalid opcode %X\n", pc, op)
			panic(fmt.Errorf("Invalid opcode %X", op))
		}

		pc++

	}
}

func subslice(data []byte, offset, length uint64, flip_ bool) (ret []byte, ok bool) {
	size := uint64(len(data))
	if size < offset {
		return nil, false
	} else if size < offset+length {
		ret, ok = data[offset:], false
	} else {
		ret, ok = data[offset:offset+length], true
	}
	if flip_ {
		ret = flip(ret)
	}
	return
}

func codeGetOp(code []byte, n uint64) OpCode {
	if uint64(len(code)) <= n {
		return OpCode(0) // stop
	} else {
		return OpCode(code[n])
	}
}

func jump(code []byte, to uint64, pc *uint64) (err error) {
	dest := codeGetOp(code, to)
	if dest != JUMPDEST {
		dbg.Printf(" ~> %v invalid jump dest %v\n", to, dest)
		return ErrInvalidJumpDest
	}
	dbg.Printf(" ~> %v\n", to)
	*pc = to
	return nil
}

func firstErr(errA, errB error) error {
	if errA != nil {
		return errA
	} else {
		return errB
	}
}

func useGas(gas *uint64, gasToUse uint64) bool {
	if *gas > gasToUse {
		*gas -= gasToUse
		return true
	} else {
		return false
	}
}

func transfer(from, to *Account, amount uint64) error {
	if from.Balance < amount {
		return ErrInsufficientBalance
	} else {
		from.Balance -= amount
		to.Balance += amount
		return nil
	}
}

func flip(in []byte) []byte {
	l2 := len(in) / 2
	flipped := make([]byte, len(in))
	// copy the middle bit (if its even it will get overwritten)
	if len(in) != 0 {
		flipped[l2] = in[l2]
	}
	for i := 0; i < l2; i++ {
		flipped[i] = in[len(in)-1-i]
		flipped[len(in)-1-i] = in[i]
	}
	return flipped
}

func flipWord(in Word256) Word256 {
	word := Word256{}
	// copy the middle bit (if its even it will get overwritten)
	for i := 0; i < 16; i++ {
		word[i] = in[len(in)-1-i]
		word[len(in)-1-i] = in[i]
	}
	return word
}
