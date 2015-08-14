package vm

import (
	"crypto/sha256"
	"github.com/tendermint/tendermint/Godeps/_workspace/src/code.google.com/p/go.crypto/ripemd160"
	. "github.com/tendermint/tendermint/common"
)

var registeredNativeContracts = make(map[Word256]NativeContract)

func RegisteredNativeContract(addr Word256) bool {
	_, ok := registeredNativeContracts[addr]
	return ok
}

func RegisterNativeContract(addr Word256, fn NativeContract) bool {
	_, exists := registeredNativeContracts[addr]
	if exists {
		return false
	}
	registeredNativeContracts[addr] = fn
	return true
}

func init() {
	registerNativeContracts()
	registerSNativeContracts()
}

func registerNativeContracts() {
	// registeredNativeContracts[Int64ToWord256(1)] = ecrecoverFunc
	registeredNativeContracts[Int64ToWord256(2)] = sha256Func
	registeredNativeContracts[Int64ToWord256(3)] = ripemd160Func
	registeredNativeContracts[Int64ToWord256(4)] = identityFunc
}

//-----------------------------------------------------------------------------

type NativeContract func(appState AppState, caller *Account, input []byte, gas *int64) (output []byte, err error)

/* Removed due to C dependency
func ecrecoverFunc(appState AppState, caller *Account, input []byte, gas *int64) (output []byte, err error) {
	// Deduct gas
	gasRequired := GasEcRecover
	if *gas < gasRequired {
		return nil, ErrInsufficientGas
	} else {
		*gas -= gasRequired
	}
	// Recover
	hash := input[:32]
	v := byte(input[32] - 27) // ignore input[33:64], v is small.
	sig := append(input[64:], v)

	recovered, err := secp256k1.RecoverPubkey(hash, sig)
	if err != nil {
		return nil, err
	}
	hashed := sha3.Sha3(recovered[1:])
	return LeftPadBytes(hashed, 32), nil
}
*/

func sha256Func(appState AppState, caller *Account, input []byte, gas *int64) (output []byte, err error) {
	// Deduct gas
	gasRequired := int64((len(input)+31)/32)*GasSha256Word + GasSha256Base
	if *gas < gasRequired {
		return nil, ErrInsufficientGas
	} else {
		*gas -= gasRequired
	}
	// Hash
	hasher := sha256.New()
	// CONTRACT: this does not err
	hasher.Write(input)
	return hasher.Sum(nil), nil
}

func ripemd160Func(appState AppState, caller *Account, input []byte, gas *int64) (output []byte, err error) {
	// Deduct gas
	gasRequired := int64((len(input)+31)/32)*GasRipemd160Word + GasRipemd160Base
	if *gas < gasRequired {
		return nil, ErrInsufficientGas
	} else {
		*gas -= gasRequired
	}
	// Hash
	hasher := ripemd160.New()
	// CONTRACT: this does not err
	hasher.Write(input)
	return LeftPadBytes(hasher.Sum(nil), 32), nil
}

func identityFunc(appState AppState, caller *Account, input []byte, gas *int64) (output []byte, err error) {
	// Deduct gas
	gasRequired := int64((len(input)+31)/32)*GasIdentityWord + GasIdentityBase
	if *gas < gasRequired {
		return nil, ErrInsufficientGas
	} else {
		*gas -= gasRequired
	}
	// Return identity
	return input, nil
}
