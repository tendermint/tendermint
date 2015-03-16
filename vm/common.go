package vm

import (
	"math/big"
)

var (
	GasStorageGet        = Big(50)
	GasStorageAdd        = Big(20000)
	GasStorageMod        = Big(5000)
	GasLogBase           = Big(375)
	GasLogTopic          = Big(375)
	GasLogByte           = Big(8)
	GasCreate            = Big(32000)
	GasCreateByte        = Big(200)
	GasCall              = Big(40)
	GasCallValueTransfer = Big(9000)
	GasStipend           = Big(2300)
	GasCallNewAccount    = Big(25000)
	GasReturn            = Big(0)
	GasStop              = Big(0)
	GasJumpDest          = Big(1)

	RefundStorage = Big(15000)
	RefundSuicide = Big(24000)

	GasMemWord           = Big(3)
	GasQuadCoeffDenom    = Big(512)
	GasContractByte      = Big(200)
	GasTransaction       = Big(21000)
	GasTxDataNonzeroByte = Big(68)
	GasTxDataZeroByte    = Big(4)
	GasTx                = Big(21000)
	GasExp               = Big(10)
	GasExpByte           = Big(10)

	GasSha3Base     = Big(30)
	GasSha3Word     = Big(6)
	GasSha256Base   = Big(60)
	GasSha256Word   = Big(12)
	GasRipemdBase   = Big(600)
	GasRipemdWord   = Big(12)
	GasEcrecover    = Big(3000)
	GasIdentityBase = Big(15)
	GasIdentityWord = Big(3)
	GasCopyWord     = Big(3)

	Pow256 = BigPow(2, 256)

	LogTyPretty byte = 0x1
	LogTyDiff   byte = 0x2
)

const MaxCallDepth = 1025

func calcMemSize(off, l *big.Int) *big.Int {
	if l.Cmp(Big0) == 0 {
		return Big0
	}

	return new(big.Int).Add(off, l)
}

// Mainly used for print variables and passing to Print*
func toValue(val *big.Int) interface{} {
	// Let's assume a string on right padded zero's
	b := val.Bytes()
	if b[0] != 0 && b[len(b)-1] == 0x0 && b[len(b)-2] == 0x0 {
		return string(b)
	}

	return val
}
