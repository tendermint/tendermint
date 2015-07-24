package vm

const (
	GasSha3          int64 = 1
	GasGetAccount    int64 = 1
	GasStorageUpdate int64 = 1

	GasBaseOp  int64 = 0 // TODO: make this 1
	GasStackOp int64 = 1

	GasEcRecover     int64 = 1
	GasSha256Word    int64 = 1
	GasSha256Base    int64 = 1
	GasRipemd160Word int64 = 1
	GasRipemd160Base int64 = 1
	GasIdentityWord  int64 = 1
	GasIdentityBase  int64 = 1
)
