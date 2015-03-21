package vm

import ()

const (
	defaultDataStackCapacity = 10
)

var (
	Zero = Word{0}
	One  = Word{1}
)

type Word [32]byte

func (w Word) String() string  { return string(w[:]) }
func (w Word) Copy() Word      { return w }
func (w Word) Bytes() []byte   { return w[:] } // copied.
func (w Word) Address() []byte { return w[:20] }
func (w Word) IsZero() bool {
	accum := byte(0)
	for _, byt := range w {
		accum |= byt
	}
	return accum == 0
}

//-----------------------------------------------------------------------------

type Account interface {
	GetAddress() Word
	GetBalance() uint64
	GetCode() []byte
	GetNonce() uint64
	GetStorageRoot() Word

	SetAddress(Word)
	SetBalance(uint64)
	SetCode([]byte)
	SetNonce(uint64)
	SetStorageRoot(Word)
}

type Log struct {
	Address Word
	Topics  []Word
	Data    []byte
	Height  uint64
}

type AppState interface {

	// Accounts
	GetAccount(addr Word) (Account, error)
	UpdateAccount(Account) error
	DeleteAccount(Account) error
	CreateAccount(Account) (Account, error)

	// Storage
	GetStorage(Word, Word) (Word, error)
	SetStorage(Word, Word, Word) (bool, error) // Setting to Zero is deleting.

	// Logs
	AddLog(*Log)
}

type Params struct {
	BlockHeight uint64
	BlockHash   Word
	BlockTime   int64
	GasLimit    uint64
}
