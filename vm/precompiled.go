package vm

import (
	"code.google.com/p/go.crypto/ripemd160"
	"crypto/sha256"
	"math/big"
)

type PrecompiledAccount struct {
	Gas func(l int) *big.Int
	fn  func(in []byte) []byte
}

func (self PrecompiledAccount) Call(in []byte) []byte {
	return self.fn(in)
}

var Precompiled = PrecompiledContracts()

// XXX Could set directly. Testing requires resetting and setting of pre compiled contracts.
func PrecompiledContracts() map[string]*PrecompiledAccount {
	return map[string]*PrecompiledAccount{
		// ECRECOVER
		/*
			string(LeftPadBytes([]byte{1}, 20)): &PrecompiledAccount{func(l int) *big.Int {
				return GasEcrecover
			}, ecrecoverFunc},
		*/

		// SHA256
		string(LeftPadBytes([]byte{2}, 20)): &PrecompiledAccount{func(l int) *big.Int {
			n := big.NewInt(int64(l+31) / 32)
			n.Mul(n, GasSha256Word)
			return n.Add(n, GasSha256Base)
		}, sha256Func},

		// RIPEMD160
		string(LeftPadBytes([]byte{3}, 20)): &PrecompiledAccount{func(l int) *big.Int {
			n := big.NewInt(int64(l+31) / 32)
			n.Mul(n, GasRipemdWord)
			return n.Add(n, GasRipemdBase)
		}, ripemd160Func},

		string(LeftPadBytes([]byte{4}, 20)): &PrecompiledAccount{func(l int) *big.Int {
			n := big.NewInt(int64(l+31) / 32)
			n.Mul(n, GasIdentityWord)

			return n.Add(n, GasIdentityBase)
		}, memCpy},
	}
}

func sha256Func(in []byte) []byte {
	hasher := sha256.New()
	n, err := hasher.Write(in)
	if err != nil {
		panic(err)
	}
	return hasher.Sum(nil)
}

func ripemd160Func(in []byte) []byte {
	hasher := ripemd160.New()
	n, err := hasher.Write(in)
	if err != nil {
		panic(err)
	}
	res := hasher.Sum(nil)
	return LeftPadBytes(res, 32)
}

func memCpy(in []byte) []byte {
	return in
}
