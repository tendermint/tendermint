package vm

import (
	"fmt"

	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/vm/sha3"
)

type Vm struct {
	VMEnvironment
}

func NewVM(appState AppState, params VMParams) *Vm {
	vmEnv := NewVMEnvironment(appState, params)
	return &VM{
		VMEnvironment: vmEnv,
	}
}

// feeLimit: the maximum the caller is willing to pay for fees.
// gasLimit: the maximum gas that will be run.
func (vm *Vm) RunTransaction(caller, target *Account, feeLimit, gasLimit, value uint64, input []byte) (output []byte, err error) {

	if len(target.Code) == 0 {
		panic("RunTransaction() requires target with code")
	}

	// Check the gasLimit vs feeLimit
	// TODO

	// Check caller's account balance vs feeLimit and value
	if caller.Balance < (feeLimit + value) {
		return nil, ErrInsufficientAccountBalance
	}

	// Deduct balance from caller.
	caller.Balance -= (feeLimit + value)

	vm.SetupTransaction(caller, target, gasLimit, value, input)

	fmt.Printf("(%d) (%X) %X (code=%d) gas: %v (d) %X\n", vm.CallStackDepth(), caller.Address[:4], target.Address, len(target.Code), gasLimit, input)

	/*
		if p := Precompiled[string(me.Address())]; p != nil {
			return vm.RunPrecompiled(p, callData, context)
		}
	*/

	//-----------------------------------

	// By the time we're here, the related VMCall context is already appended onto VMEnvironment.callStack
	var (
		code          = target.Code
		pc     uint64 = 0
		gas    uint64 = call.gasLimit
		err    error  = nil
		stack         = NewStack(defaultDataStackCapacity, &gas, &err)
		memory        = NewMemory(&gas, &err)

		// volatile, convenience
		ok = false

		// TODO review this code.
		jump = func(from, to uint64) error {
			dest := CodeGetOp(code, to)
			if dest != JUMPDEST {
				return ErrInvalidJumpDest
			}
			pc = to
			fmt.Printf(" ~> %v\n", to)
			return nil
		}
	)

	for {
		var op = CodeGetOp(code, pc)
		fmt.Printf("(pc) %-3d -o- %-14s (m) %-4d (s) %-4d ", pc, op.String(), mem.Len(), stack.Len())

		switch op {

		case ADD: // 0x01
			x, y := stack.Pop64(), stack.Pop64()
			stack.Push64(x + y)
			fmt.Printf(" %v + %v = %v\n", x, y, x+y)

		case MUL: // 0x02
			x, y := stack.Pop64(), stack.Pop64()
			stack.Push64(x * y)
			fmt.Printf(" %v * %v = %v\n", x, y, x*y)

		case SUB: // 0x03
			x, y := stack.Pop64(), stack.Pop64()
			stack.Push64(x - y)
			fmt.Printf(" %v - %v = %v\n", x, y, x-y)

		case DIV: // 0x04
			x, y := stack.Pop64(), stack.Pop64()
			if y == 0 { // TODO
				stack.Push64(0)
				fmt.Printf(" %v / %v = %v (TODO)\n", x, y, 0)
			} else {
				stack.Push64(x / y)
				fmt.Printf(" %v / %v = %v\n", x, y, x/y)
			}

		case SDIV: // 0x05
			x, y := int64(stack.Pop64()), int64(stack.Pop64())
			if y == 0 { // TODO
				stack.Push64(0)
				fmt.Printf(" %v / %v = %v (TODO)\n", x, y, 0)
			} else {
				stack.Push64(uint64(x / y))
				fmt.Printf(" %v / %v = %v\n", x, y, x/y)
			}

		case MOD: // 0x06
			x, y := stack.Pop64(), stack.Pop64()
			if y == 0 { // TODO
				stack.Push64(0)
				fmt.Printf(" %v %% %v = %v (TODO)\n", x, y, 0)
			} else {
				stack.Push64(x % y)
				fmt.Printf(" %v %% %v = %v\n", x, y, x%y)
			}

		case SMOD: // 0x07
			x, y := int64(stack.Pop64()), int64(stack.Pop64())
			if y == 0 { // TODO
				stack.Push64(0)
				fmt.Printf(" %v %% %v = %v (TODO)\n", x, y, 0)
			} else {
				stack.Push64(uint64(x % y))
				fmt.Printf(" %v %% %v = %v\n", x, y, x%y)
			}

		case ADDMOD: // 0x08
			x, y, z := stack.Pop64(), stack.Pop64(), stack.Pop64()
			if z == 0 { // TODO
				stack.Push64(0)
				fmt.Printf(" (%v + %v) %% %v = %v (TODO)\n", x, y, z, 0)
			} else {
				stack.Push64(x % y)
				fmt.Printf(" (%v + %v) %% %v = %v\n", x, y, z, (x+y)%z)
			}

		case MULMOD: // 0x09
			x, y, z := stack.Pop64(), stack.Pop64(), stack.Pop64()
			if z == 0 { // TODO
				stack.Push64(0)
				fmt.Printf(" (%v + %v) %% %v = %v (TODO)\n", x, y, z, 0)
			} else {
				stack.Push64(x % y)
				fmt.Printf(" (%v + %v) %% %v = %v\n", x, y, z, (x*y)%z)
			}

		case EXP: // 0x0A
			x, y := stack.Pop64(), stack.Pop64()
			stack.Push64(ExpUint64(x, y))
			fmt.Printf(" %v ** %v = %v\n", x, y, uint64(math.Pow(float64(x), float64(y))))

		case SIGNEXTEND: // 0x0B
			x, y := stack.Pop64(), stack.Pop64()
			res := (y << uint(x)) >> x
			stack.Push64(res)
			fmt.Printf(" (%v << %v) >> %v = %v\n", y, x, x, res)

		case LT: // 0x10
			x, y := stack.Pop64(), stack.Pop64()
			if x < y {
				stack.Push64(1)
			} else {
				stack.Push64(0)
			}
			fmt.Printf(" %v < %v = %v\n", x, y, x < y)

		case GT: // 0x11
			x, y := stack.Pop64(), stack.Pop64()
			if x > y {
				stack.Push64(1)
			} else {
				stack.Push64(0)
			}
			fmt.Printf(" %v > %v = %v\n", x, y, x > y)

		case SLT: // 0x12
			x, y := int64(stack.Pop64()), int64(stack.Pop64())
			if x < y {
				stack.Push64(1)
			} else {
				stack.Push64(0)
			}
			fmt.Printf(" %v < %v = %v\n", x, y, x < y)

		case SGT: // 0x13
			x, y := int64(stack.Pop64()), int64(stack.Pop64())
			if x > y {
				stack.Push64(1)
			} else {
				stack.Push64(0)
			}
			fmt.Printf(" %v > %v = %v\n", x, y, x > y)

		case EQ: // 0x14
			x, y := stack.Pop64(), stack.Pop64()
			if x > y {
				stack.Push64(1)
			} else {
				stack.Push64(0)
			}
			fmt.Printf(" %v == %v = %v\n", x, y, x == y)

		case ISZERO: // 0x15
			x := stack.Pop64()
			if x == 0 {
				stack.Push64(1)
			} else {
				stack.Push64(0)
			}
			fmt.Printf(" %v == 0 = %v\n", x, x == 0)

		case AND: // 0x16
			x, y := stack.Pop64(), stack.Pop64()
			stack.Push64(x & y)
			fmt.Printf(" %v & %v = %v\n", x, y, x&y)

		case OR: // 0x17
			x, y := stack.Pop64(), stack.Pop64()
			stack.Push64(x | y)
			fmt.Printf(" %v | %v = %v\n", x, y, x|y)

		case XOR: // 0x18
			x, y := stack.Pop64(), stack.Pop64()
			stack.Push64(x ^ y)
			fmt.Printf(" %v ^ %v = %v\n", x, y, x^y)

		case NOT: // 0x19
			x := stack.Pop64()
			stack.Push64(^x)
			fmt.Printf(" !%v = %v\n", x, ^x)

		case BYTE: // 0x1A
			idx, val := stack.Pop64(), stack.Pop()
			res := 0
			if idx < 32 {
				res = Uint642Bytes(val[idx/8])[idx%8]
			}
			stack.Push64(res)
			fmt.Printf(" => 0x%X", res)

		case SHA3: // 0x20
			if gas, ok = useGas(gas, GasSha3); !ok {
				return ErrInsufficientGas
			}
			offset, size := stack.Pop64(), stack.Pop64()
			data := sha3.Sha3(memory.Get(offset, size))
			stack.PushBytes(data)
			fmt.Printf(" => (%v) %X", size, data)

		case ADDRESS: // 0x30
			stack.PushBytes(RightPadBytes(context.Address(), 32))
			fmt.Printf(" => %X", RightPadBytes(context.Address(), 32))

		case BALANCE: // 0x31
			addr := stack.PopBytes()
			if gas, ok = useGas(gas, GasGetAccount); !ok {
				return ErrInsufficientGas
			}
			account := vm.GetAccount(addr) // TODO ensure that 20byte lengths are supported.
			balance := account.Balance
			stack.Push64(balance)
			fmt.Printf(" => %v (%X)", balance, addr)

		case ORIGIN: // 0x32
			origin := vm.Origin()
			stack.PushBytes(origin)
			fmt.Printf(" => %X", origin)

		case CALLER: // 0x33
			caller := vm.lastCall.caller
			stack.PushBytes(caller.Address)
			fmt.Printf(" => %X", caller.Address)

		case CALLVALUE: // 0x34
			stack.Push64(value)
			fmt.Printf(" => %v", value)

		case CALLDATALOAD: // 0x35
			offset := stack.Pop64()
			data, _ := subslice(input, offset, 32)
			stack.PushBytes(RightPadBytes(data), 32)
			fmt.Printf(" => 0x%X", data)

		case CALLDATASIZE: // 0x36
			stack.Push64(uint64(len(callData)))
			fmt.Printf(" => %d", len(callData))

		case CALLDATACOPY: // 0x37
			memOff := stack.Pop64()
			inputOff := stack.Pop64()
			length := stack.Pop64()
			data, ok := subslice(input, inputOff, length)
			if ok {
				memory.Set(memOff, length, data)
			}
			fmt.Printf(" => [%v, %v, %v] %X", memOff, inputOff, length, data)

		case CODESIZE: // 0x38
			l := uint64(len(code))
			stack.Push64(l)
			fmt.Printf(" => %d", l)

		case CODECOPY: // 0x39
			memOff := stack.Pop64()
			codeOff := stack.Pop64()
			length := stack.Pop64()
			data, ok := subslice(code, codeOff, length)
			if ok {
				memory.Set(memOff, length, data)
			}
			fmt.Printf(" => [%v, %v, %v] %X", memOff, codeOff, length, data)

		case GASPRICE: // 0x3A
			stack.Push64(vm.params.GasPrice)
			fmt.Printf(" => %X", vm.params.GasPrice)

		case EXTCODESIZE: // 0x3B
			addr := stack.PopBytes()[:20]
			account := vm.GetAccount(addr)
			code := account.Code
			l := uint64(len(code))
			stack.Push64(l)
			fmt.Printf(" => %d", l)

		case EXTCODECOPY: // 0x3C
			addr := stack.PopBytes()[:20]
			account := vm.GetAccount(addr)
			code := account.Code
			memOff := stack.Pop64()
			codeOff := stack.Pop64()
			length := stack.Pop64()
			data, ok := subslice(code, codeOff, length)
			if ok {
				memory.Set(memOff, length, data)
			}
			fmt.Printf(" => [%v, %v, %v] %X", memOff, codeOff, length, data)

		case BLOCKHASH: // 0x40
			/*
				num := stack.pop()
				n := new(big.Int).Sub(vm.env.BlockHeight(), Big257)
				if num.Cmp(n) > 0 && num.Cmp(vm.env.BlockHeight()) < 0 {
					stack.push(Bytes2Big(vm.env.GetBlockHash(num.Uint64())))
				} else {
					stack.push(Big0)
				}
			*/
			stack.Push([4]Word{0, 0, 0, 0})
			fmt.Printf(" => 0x%X (NOT SUPPORTED)", stack.Peek().Bytes())

		case COINBASE: // 0x41
			stack.Push([4]Word{0, 0, 0, 0})
			fmt.Printf(" => 0x%X (NOT SUPPORTED)", stack.Peek().Bytes())

		case TIMESTAMP: // 0x42
			time := vm.params.BlockTime
			stack.Push64(uint64(time))
			fmt.Printf(" => 0x%X", time)

		case BLOCKHEIGHT: // 0x43
			number := uint64(vm.params.BlockHeight)
			stack.Push64(number)
			fmt.Printf(" => 0x%X", number)

		case GASLIMIT: // 0x45
			stack.Push64(vm.params.GasLimit)
			fmt.Printf(" => %v", vm.params.GasLimit)

		case POP: // 0x50
			stack.Pop()
			fmt.Printf(" => %v", vm.params.GasLimit)

		case MLOAD: // 0x51
			offset := stack.Pop64()
			data, _ := subslice(input, offset, 32)
			stack.PushBytes(RightPadBytes(data), 32)
			fmt.Printf(" => 0x%X", data)

			offset := stack.pop()
			val := Bytes2Big(mem.Get(offset.Int64(), 32))
			stack.push(val)

			fmt.Printf(" => 0x%X", val.Bytes())
		case MSTORE: // Store the value at stack top-1 in to memory at location stack top
			// pop value of the stack
			mStart, val := stack.pop(), stack.pop()
			mem.Set(mStart.Uint64(), 32, Big2Bytes(val, 256))

			fmt.Printf(" => 0x%X", val)
		case MSTORE8:
			off, val := stack.pop(), stack.pop()

			mem.store[off.Int64()] = byte(val.Int64() & 0xff)

			fmt.Printf(" => [%v] 0x%X", off, val)
		case SLOAD:
			loc := stack.pop()
			val := Bytes2Big(state.GetState(context.Address(), loc.Bytes()))
			stack.push(val)

			fmt.Printf(" {0x%X : 0x%X}", loc.Bytes(), val.Bytes())
		case SSTORE:
			loc, val := stack.pop(), stack.pop()
			state.SetState(context.Address(), loc.Bytes(), val)

			fmt.Printf(" {0x%X : 0x%X}", loc.Bytes(), val.Bytes())
		case JUMP:
			jump(pc, stack.pop())

			continue
		case JUMPI:
			pos, cond := stack.pop(), stack.pop()

			if cond.Cmp(BigTrue) >= 0 {
				jump(pc, pos)

				continue
			}

			fmt.Printf(" ~> false")

		case JUMPDEST:
		case PC:
			stack.push(Big(int64(pc)))
		case MSIZE:
			stack.push(Big(int64(mem.Len())))
		case GAS:
			stack.push(context.Gas)

			fmt.Printf(" => %X", context.Gas)

		case PUSH1, PUSH2, PUSH3, PUSH4, PUSH5, PUSH6, PUSH7, PUSH8, PUSH9, PUSH10, PUSH11, PUSH12, PUSH13, PUSH14, PUSH15, PUSH16, PUSH17, PUSH18, PUSH19, PUSH20, PUSH21, PUSH22, PUSH23, PUSH24, PUSH25, PUSH26, PUSH27, PUSH28, PUSH29, PUSH30, PUSH31, PUSH32:
			a := uint64(op - PUSH1 + 1)
			byts := context.GetRangeValue(pc+1, a)
			// push value to stack
			stack.push(Bytes2Big(byts))
			pc += a

			fmt.Printf(" => 0x%X", byts)

		case DUP1, DUP2, DUP3, DUP4, DUP5, DUP6, DUP7, DUP8, DUP9, DUP10, DUP11, DUP12, DUP13, DUP14, DUP15, DUP16:
			n := int(op - DUP1 + 1)
			stack.dup(n)

			fmt.Printf(" => [%d] 0x%X", n, stack.peek().Bytes())
		case SWAP1, SWAP2, SWAP3, SWAP4, SWAP5, SWAP6, SWAP7, SWAP8, SWAP9, SWAP10, SWAP11, SWAP12, SWAP13, SWAP14, SWAP15, SWAP16:
			n := int(op - SWAP1 + 2)
			stack.swap(n)

			fmt.Printf(" => [%d]", n)
		case LOG0, LOG1, LOG2, LOG3, LOG4:
			n := int(op - LOG0)
			topics := make([][]byte, n)
			mStart, mSize := stack.pop(), stack.pop()
			for i := 0; i < n; i++ {
				topics[i] = LeftPadBytes(stack.pop().Bytes(), 32)
			}

			data := mem.Get(mStart.Int64(), mSize.Int64())
			log := &Log{context.Address(), topics, data, vm.env.BlockHeight().Uint64()}
			vm.env.AddLog(log)

			fmt.Printf(" => %v", log)

			// 0x60 range
		case CREATE:

			var (
				value        = stack.pop()
				offset, size = stack.pop(), stack.pop()
				input        = mem.Get(offset.Int64(), size.Int64())
				gas          = new(big.Int).Set(context.Gas)
				addr         []byte
			)
			vm.Endl()

			context.UseGas(context.Gas)
			ret, suberr, ref := Create(vm, context, nil, input, gas, price, value)
			if suberr != nil {
				stack.push(BigFalse)

				fmt.Printf(" (*) 0x0 %v", suberr)
			} else {

				// gas < len(ret) * CreateDataGas == NO_CODE
				dataGas := Big(int64(len(ret)))
				dataGas.Mul(dataGas, GasCreateByte)
				if context.UseGas(dataGas) {
					ref.SetCode(ret)
				}
				addr = ref.Address()

				stack.push(Bytes2Big(addr))

			}

		case CALL, CALLCODE:
			gas := stack.pop()
			// pop gas and value of the stack.
			addr, value := stack.pop(), stack.pop()
			value = U256(value)
			// pop input size and offset
			inOffset, inSize := stack.pop(), stack.pop()
			// pop return size and offset
			retOffset, retSize := stack.pop(), stack.pop()

			address := addr.Bytes()
			fmt.Printf(" => %X", address).Endl()

			// Get the arguments from the memory
			args := mem.Get(inOffset.Int64(), inSize.Int64())

			if len(value.Bytes()) > 0 {
				gas.Add(gas, GasStipend)
			}

			var (
				ret []byte
				err error
			)
			if op == CALLCODE {
				ret, err = CallCode(env, context, address, args, gas, price, value)
			} else {
				ret, err = Call(env, context, address, args, gas, price, value)
			}

			if err != nil {
				stack.push(BigFalse)

				fmt.Printf("%v").Endl()
			} else {
				stack.push(BigTrue)

				mem.Set(retOffset.Uint64(), retSize.Uint64(), ret)
			}
			fmt.Printf("resume %X (%v)", context.Address(), context.Gas)

		case RETURN:
			offset, size := stack.pop(), stack.pop()
			ret := mem.Get(offset.Int64(), size.Int64())
			fmt.Printf(" => [%v, %v] (%d) 0x%X", offset, size, len(ret), ret).Endl()
			return context.Return(ret), nil

		case SUICIDE:
			receiver := state.GetOrNewStateObject(stack.pop().Bytes())
			balance := state.GetBalance(context.Address())

			fmt.Printf(" => (%X) %v", receiver.Address()[:4], balance)

			receiver.AddBalance(balance)

			state.Delete(context.Address())

			fallthrough
		case STOP: // Stop the context
			vm.Endl()

			return context.Return(nil), nil
		default:
			fmt.Printf("(pc) %-3v Invalid opcode %X\n", pc, op).Endl()

			panic(fmt.Errorf("Invalid opcode %X", op))
		}

		pc++

		vm.Endl()
	}
}

/*
func (vm *Vm) RunPrecompiled(p *PrecompiledAccount, callData []byte, context *Context) (ret []byte, err error) {
	gas := p.Gas(len(callData))
	if context.UseGas(gas) {
		ret = p.Call(callData)
		fmt.Printf("NATIVE_FUNC => %X", ret)
		vm.Endl()

		return context.Return(ret), nil
	} else {
		fmt.Printf("NATIVE_FUNC => failed").Endl()

		tmp := new(big.Int).Set(context.Gas)

		panic(OOG(gas, tmp).Error())
	}
}
*/

func subslice(data []byte, offset, length uint64) ([]byte, bool) {
	size := uint64(len(data))
	if size < offset {
		return nil, false
	} else if size < offset+length {
		return data[offset:], false
	} else {
		return data[offset : offset+length], true
	}
}

func useGas(gasLeft, gasToUse uint64) (uint64, bool) {
	if gasLeft > gasToUse {
		return gasLeft - gasToUse, true
	} else {
		return gasLeft, false
	}
}
