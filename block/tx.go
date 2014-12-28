package block

import (
	"errors"
	"io"
	"reflect"

	. "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

var (
	ErrTxInvalidAddress    = errors.New("Error invalid address")
	ErrTxDuplicateAddress  = errors.New("Error duplicate address")
	ErrTxInvalidAmount     = errors.New("Error invalid amount")
	ErrTxInsufficientFunds = errors.New("Error insufficient funds")
	ErrTxUnknownPubKey     = errors.New("Error unknown pubkey")
	ErrTxInvalidPubKey     = errors.New("Error invalid pubkey")
	ErrTxRedeclaredPubKey  = errors.New("Error redeclared pubkey")
	ErrTxInvalidSignature  = errors.New("Error invalid signature")
	ErrTxInvalidSequence   = errors.New("Error invalid sequence")
)

/*
Tx (Transaction) is an atomic operation on the ledger state.

Account Txs:
 - SendTx         Send coins to address

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

	// Validation transactions
	TxTypeBond    = byte(0x11)
	TxTypeUnbond  = byte(0x12)
	TxTypeRebond  = byte(0x13)
	TxTypeDupeout = byte(0x14)
)

//-------------------------------------
// for binary.readReflect

func TxDecoder(r io.Reader, n *int64, err *error) interface{} {
	switch t := ReadByte(r, n, err); t {
	case TxTypeSend:
		return ReadBinary(&SendTx{}, r, n, err)
	case TxTypeBond:
		return ReadBinary(&BondTx{}, r, n, err)
	case TxTypeUnbond:
		return ReadBinary(&UnbondTx{}, r, n, err)
	case TxTypeRebond:
		return ReadBinary(&RebondTx{}, r, n, err)
	case TxTypeDupeout:
		return ReadBinary(&DupeoutTx{}, r, n, err)
	default:
		*err = Errorf("Unknown Tx type %X", t)
		return nil
	}
}

var _ = RegisterType(&TypeInfo{
	Type:    reflect.TypeOf((*Tx)(nil)).Elem(),
	Decoder: TxDecoder,
})

//-----------------------------------------------------------------------------

type TxInput struct {
	Address   []byte    // Hash of the PubKey
	Amount    uint64    // Must not exceed account balance
	Sequence  uint      // Must be 1 greater than the last committed TxInput
	Signature Signature // Depends on the PubKey type and the whole Tx
	PubKey    PubKey    // Optional, may be nil
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
	WriteByteSlice(txIn.Address, w, n, err)
	WriteUint64(txIn.Amount, w, n, err)
	WriteUvarint(txIn.Sequence, w, n, err)
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
	WriteByteSlice(txOut.Address, w, n, err)
	WriteUint64(txOut.Amount, w, n, err)
}

//-----------------------------------------------------------------------------

type SendTx struct {
	Inputs  []*TxInput
	Outputs []*TxOutput
}

func (tx *SendTx) TypeByte() byte { return TxTypeSend }

func (tx *SendTx) WriteSignBytes(w io.Writer, n *int64, err *error) {
	WriteUvarint(uint(len(tx.Inputs)), w, n, err)
	for _, in := range tx.Inputs {
		in.WriteSignBytes(w, n, err)
	}
	WriteUvarint(uint(len(tx.Outputs)), w, n, err)
	for _, out := range tx.Outputs {
		out.WriteSignBytes(w, n, err)
	}
}

//-----------------------------------------------------------------------------

type BondTx struct {
	PubKey   PubKeyEd25519
	Inputs   []*TxInput
	UnbondTo []*TxOutput
}

func (tx *BondTx) TypeByte() byte { return TxTypeBond }

func (tx *BondTx) WriteSignBytes(w io.Writer, n *int64, err *error) {
	WriteBinary(tx.PubKey, w, n, err)
	WriteUvarint(uint(len(tx.Inputs)), w, n, err)
	for _, in := range tx.Inputs {
		in.WriteSignBytes(w, n, err)
	}
	WriteUvarint(uint(len(tx.UnbondTo)), w, n, err)
	for _, out := range tx.UnbondTo {
		out.WriteSignBytes(w, n, err)
	}
}

//-----------------------------------------------------------------------------

type UnbondTx struct {
	Address   []byte
	Height    uint
	Signature SignatureEd25519
}

func (tx *UnbondTx) TypeByte() byte { return TxTypeUnbond }

func (tx *UnbondTx) WriteSignBytes(w io.Writer, n *int64, err *error) {
	WriteByteSlice(tx.Address, w, n, err)
	WriteUvarint(tx.Height, w, n, err)
}

//-----------------------------------------------------------------------------

type RebondTx struct {
	Address   []byte
	Height    uint
	Signature SignatureEd25519
}

func (tx *RebondTx) TypeByte() byte { return TxTypeRebond }

func (tx *RebondTx) WriteSignBytes(w io.Writer, n *int64, err *error) {
	WriteByteSlice(tx.Address, w, n, err)
	WriteUvarint(tx.Height, w, n, err)
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
