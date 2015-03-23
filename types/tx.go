package types

import (
	"errors"
	"io"

	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

var (
	ErrTxInvalidAddress       = errors.New("Error invalid address")
	ErrTxDuplicateAddress     = errors.New("Error duplicate address")
	ErrTxInvalidAmount        = errors.New("Error invalid amount")
	ErrTxInsufficientFunds    = errors.New("Error insufficient funds")
	ErrTxInsufficientGasPrice = errors.New("Error insufficient gas price")
	ErrTxUnknownPubKey        = errors.New("Error unknown pubkey")
	ErrTxInvalidPubKey        = errors.New("Error invalid pubkey")
	ErrTxInvalidSignature     = errors.New("Error invalid signature")
)

type ErrTxInvalidSequence struct {
	Got      uint64
	Expected uint64
}

func (e ErrTxInvalidSequence) Error() string {
	return Fmt("Error invalid sequence. Got %d, expected %d", e.Got, e.Expected)
}

/*
Tx (Transaction) is an atomic operation on the ledger state.

Account Txs:
 - SendTx         Send coins to address
 - CallTx         Send a msg to a contract that runs in the vm

Validation Txs:
 - BondTx         New validator posts a bond
 - UnbondTx       Validator leaves
 - DupeoutTx      Validator dupes out (equivocates)
*/
type Tx interface {
	WriteSignBytes(w io.Writer, n *int64, err *error)
}

// Types of Tx implementations
const (
	// Account transactions
	TxTypeSend = byte(0x01)
	TxTypeCall = byte(0x02)

	// Validation transactions
	TxTypeBond    = byte(0x11)
	TxTypeUnbond  = byte(0x12)
	TxTypeRebond  = byte(0x13)
	TxTypeDupeout = byte(0x14)
)

// for binary.readReflect
var _ = binary.RegisterInterface(
	struct{ Tx }{},
	binary.ConcreteType{&SendTx{}},
	binary.ConcreteType{&CallTx{}},
	binary.ConcreteType{&BondTx{}},
	binary.ConcreteType{&UnbondTx{}},
	binary.ConcreteType{&RebondTx{}},
	binary.ConcreteType{&DupeoutTx{}},
)

//-----------------------------------------------------------------------------

type TxInput struct {
	Address   []byte            // Hash of the PubKey
	Amount    uint64            // Must not exceed account balance
	Sequence  uint              // Must be 1 greater than the last committed TxInput
	Signature account.Signature // Depends on the PubKey type and the whole Tx
	PubKey    account.PubKey    // Must not be nil, may be PubKeyNil.
}

func (txIn *TxInput) ValidateBasic() error {
	if len(txIn.Address) != 20 {
		return ErrTxInvalidAddress
	}
	if txIn.Amount == 0 {
		return ErrTxInvalidAmount
	}
	return nil
}

func (txIn *TxInput) WriteSignBytes(w io.Writer, n *int64, err *error) {
	binary.WriteByteSlice(txIn.Address, w, n, err)
	binary.WriteUint64(txIn.Amount, w, n, err)
	binary.WriteUvarint(txIn.Sequence, w, n, err)
}

func (txIn *TxInput) String() string {
	return Fmt("TxInput{%X,%v,%v,%v,%v}", txIn.Address, txIn.Amount, txIn.Sequence, txIn.Signature, txIn.PubKey)
}

//-----------------------------------------------------------------------------

type TxOutput struct {
	Address []byte // Hash of the PubKey
	Amount  uint64 // The sum of all outputs must not exceed the inputs.
}

func (txOut *TxOutput) ValidateBasic() error {
	if len(txOut.Address) != 20 {
		return ErrTxInvalidAddress
	}
	if txOut.Amount == 0 {
		return ErrTxInvalidAmount
	}
	return nil
}

func (txOut *TxOutput) WriteSignBytes(w io.Writer, n *int64, err *error) {
	binary.WriteByteSlice(txOut.Address, w, n, err)
	binary.WriteUint64(txOut.Amount, w, n, err)
}

func (txOut *TxOutput) String() string {
	return Fmt("TxOutput{%X,%v}", txOut.Address, txOut.Amount)
}

//-----------------------------------------------------------------------------

type SendTx struct {
	Inputs  []*TxInput
	Outputs []*TxOutput
}

func (tx *SendTx) TypeByte() byte { return TxTypeSend }

func (tx *SendTx) WriteSignBytes(w io.Writer, n *int64, err *error) {
	binary.WriteUvarint(uint(len(tx.Inputs)), w, n, err)
	for _, in := range tx.Inputs {
		in.WriteSignBytes(w, n, err)
	}
	binary.WriteUvarint(uint(len(tx.Outputs)), w, n, err)
	for _, out := range tx.Outputs {
		out.WriteSignBytes(w, n, err)
	}
}

func (tx *SendTx) String() string {
	return Fmt("SendTx{%v -> %v}", tx.Inputs, tx.Outputs)
}

//-----------------------------------------------------------------------------

type CallTx struct {
	Input    *TxInput
	Address  []byte
	GasLimit uint64
	Fee      uint64
	Data     []byte
}

func (tx *CallTx) TypeByte() byte { return TxTypeCall }

func (tx *CallTx) WriteSignBytes(w io.Writer, n *int64, err *error) {
	tx.Input.WriteSignBytes(w, n, err)
	binary.WriteByteSlice(tx.Address, w, n, err)
	binary.WriteUint64(tx.GasLimit, w, n, err)
	binary.WriteUint64(tx.Fee, w, n, err)
	binary.WriteByteSlice(tx.Data, w, n, err)
}

func (tx *CallTx) String() string {
	return Fmt("CallTx{%v -> %x: %x}", tx.Input, tx.Address, tx.Data)
}

//-----------------------------------------------------------------------------

type BondTx struct {
	PubKey   account.PubKeyEd25519
	Inputs   []*TxInput
	UnbondTo []*TxOutput
}

func (tx *BondTx) TypeByte() byte { return TxTypeBond }

func (tx *BondTx) WriteSignBytes(w io.Writer, n *int64, err *error) {
	binary.WriteBinary(tx.PubKey, w, n, err)
	binary.WriteUvarint(uint(len(tx.Inputs)), w, n, err)
	for _, in := range tx.Inputs {
		in.WriteSignBytes(w, n, err)
	}
	binary.WriteUvarint(uint(len(tx.UnbondTo)), w, n, err)
	for _, out := range tx.UnbondTo {
		out.WriteSignBytes(w, n, err)
	}
}

func (tx *BondTx) String() string {
	return Fmt("BondTx{%v: %v -> %v}", tx.PubKey, tx.Inputs, tx.UnbondTo)
}

//-----------------------------------------------------------------------------

type UnbondTx struct {
	Address   []byte
	Height    uint
	Signature account.SignatureEd25519
}

func (tx *UnbondTx) TypeByte() byte { return TxTypeUnbond }

func (tx *UnbondTx) WriteSignBytes(w io.Writer, n *int64, err *error) {
	binary.WriteByteSlice(tx.Address, w, n, err)
	binary.WriteUvarint(tx.Height, w, n, err)
}

func (tx *UnbondTx) String() string {
	return Fmt("UnbondTx{%X,%v,%v}", tx.Address, tx.Height, tx.Signature)
}

//-----------------------------------------------------------------------------

type RebondTx struct {
	Address   []byte
	Height    uint
	Signature account.SignatureEd25519
}

func (tx *RebondTx) TypeByte() byte { return TxTypeRebond }

func (tx *RebondTx) WriteSignBytes(w io.Writer, n *int64, err *error) {
	binary.WriteByteSlice(tx.Address, w, n, err)
	binary.WriteUvarint(tx.Height, w, n, err)
}

func (tx *RebondTx) String() string {
	return Fmt("RebondTx{%X,%v,%v}", tx.Address, tx.Height, tx.Signature)
}

//-----------------------------------------------------------------------------

type DupeoutTx struct {
	Address []byte
	VoteA   Vote
	VoteB   Vote
}

func (tx *DupeoutTx) TypeByte() byte { return TxTypeDupeout }

func (tx *DupeoutTx) WriteSignBytes(w io.Writer, n *int64, err *error) {
	panic("DupeoutTx has no sign bytes")
}

func (tx *DupeoutTx) String() string {
	return Fmt("DupeoutTx{%X,%v,%v}", tx.Address, tx.VoteA, tx.VoteB)
}
