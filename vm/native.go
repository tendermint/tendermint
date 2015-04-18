package vm

import (
	"code.google.com/p/go.crypto/ripemd160"
	"crypto/sha256"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/vm/secp256k1"
	"github.com/tendermint/tendermint/vm/sha3"
)

var nativeContracts = make(map[Word256]NativeContract)

func init() {
	nativeContracts[Uint64ToWord256(1)] = ecrecoverFunc
	nativeContracts[Uint64ToWord256(2)] = sha256Func
	nativeContracts[Uint64ToWord256(3)] = ripemd160Func
	nativeContracts[Uint64ToWord256(4)] = identityFunc
}

//-----------------------------------------------------------------------------

type NativeContract func(input []byte, gas *uint64) (output []byte, err error)

func ecrecoverFunc(input []byte, gas *uint64) (output []byte, err error) {
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

func sha256Func(input []byte, gas *uint64) (output []byte, err error) {
	// Deduct gas
	gasRequired := uint64((len(input)+31)/32)*GasSha256Word + GasSha256Base
	if *gas < gasRequired {
		return nil, ErrInsufficientGas
	} else {
		*gas -= gasRequired
	}
	// Hash
	hasher := sha256.New()
	_, err = hasher.Write(input)
	if err != nil {
		panic(err)
	}
	return hasher.Sum(nil), nil
}

func ripemd160Func(input []byte, gas *uint64) (output []byte, err error) {
	// Deduct gas
	gasRequired := uint64((len(input)+31)/32)*GasRipemd160Word + GasRipemd160Base
	if *gas < gasRequired {
		return nil, ErrInsufficientGas
	} else {
		*gas -= gasRequired
	}
	// Hash
	hasher := ripemd160.New()
	_, err = hasher.Write(input)
	if err != nil {
		panic(err)
	}
	return LeftPadBytes(hasher.Sum(nil), 32), nil
}

func identityFunc(input []byte, gas *uint64) (output []byte, err error) {
	// Deduct gas
	gasRequired := uint64((len(input)+31)/32)*GasIdentityWord + GasIdentityBase
	if *gas < gasRequired {
		return nil, ErrInsufficientGas
	} else {
		*gas -= gasRequired
	}
	// Return identity
	return input, nil
}
