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
	WriteSignBytes(chainID string, w io.Writer, n *int64, err *error)
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
	binary.ConcreteType{&SendTx{}, TxTypeSend},
	binary.ConcreteType{&CallTx{}, TxTypeCall},
	binary.ConcreteType{&BondTx{}, TxTypeBond},
	binary.ConcreteType{&UnbondTx{}, TxTypeUnbond},
	binary.ConcreteType{&RebondTx{}, TxTypeRebond},
	binary.ConcreteType{&DupeoutTx{}, TxTypeDupeout},
)

//-----------------------------------------------------------------------------

type TxInput struct {
	Address   []byte            `json:"address"`   // Hash of the PubKey
	Amount    uint64            `json:"amount"`    // Must not exceed account balance
	Sequence  uint              `json:"sequence"`  // Must be 1 greater than the last committed TxInput
	Signature account.Signature `json:"signature"` // Depends on the PubKey type and the whole Tx
	PubKey    account.PubKey    `json:"pub_key"`   // Must not be nil, may be nil
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
	binary.WriteTo([]byte(Fmt(`{"address":"%X","amount":%v,"sequence":%v}`, txIn.Address, txIn.Amount, txIn.Sequence)), w, n, err)
}

func (txIn *TxInput) String() string {
	return Fmt("TxInput{%X,%v,%v,%v,%v}", txIn.Address, txIn.Amount, txIn.Sequence, txIn.Signature, txIn.PubKey)
}

//-----------------------------------------------------------------------------

type TxOutput struct {
	Address []byte `json:"address"` // Hash of the PubKey
	Amount  uint64 `json:"amount"`  // The sum of all outputs must not exceed the inputs.
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
	binary.WriteTo([]byte(Fmt(`{"address":"%X","amount":%v}`, txOut.Address, txOut.Amount)), w, n, err)
}

func (txOut *TxOutput) String() string {
	return Fmt("TxOutput{%X,%v}", txOut.Address, txOut.Amount)
}

//-----------------------------------------------------------------------------

type SendTx struct {
	Inputs  []*TxInput  `json:"inputs"`
	Outputs []*TxOutput `json:"outputs"`
}

func (tx *SendTx) WriteSignBytes(chainID string, w io.Writer, n *int64, err *error) {
	// We hex encode the chain_id so we don't deal with escaping issues.
	binary.WriteTo([]byte(Fmt(`{"chain_id":"%X"`, chainID)), w, n, err)
	binary.WriteTo([]byte(Fmt(`,"tx":[%v,{"inputs":[`, TxTypeSend)), w, n, err)
	for i, in := range tx.Inputs {
		in.WriteSignBytes(w, n, err)
		if i != len(tx.Inputs)-1 {
			binary.WriteTo([]byte(","), w, n, err)
		}
	}
	binary.WriteTo([]byte(`],"outputs":[`), w, n, err)
	for i, out := range tx.Outputs {
		out.WriteSignBytes(w, n, err)
		if i != len(tx.Outputs)-1 {
			binary.WriteTo([]byte(","), w, n, err)
		}
	}
	binary.WriteTo([]byte(`]}]}`), w, n, err)
}

func (tx *SendTx) String() string {
	return Fmt("SendTx{%v -> %v}", tx.Inputs, tx.Outputs)
}

//-----------------------------------------------------------------------------

type CallTx struct {
	Input    *TxInput `json:"input"`
	Address  []byte   `json:"address"`
	GasLimit uint64   `json:"gas_limit"`
	Fee      uint64   `json:"fee"`
	Data     []byte   `json:"data"`
}

func (tx *CallTx) WriteSignBytes(chainID string, w io.Writer, n *int64, err *error) {
	// We hex encode the chain_id so we don't deal with escaping issues.
	binary.WriteTo([]byte(Fmt(`{"chain_id":"%X"`, chainID)), w, n, err)
	binary.WriteTo([]byte(Fmt(`,"tx":[%v,{"address":"%X","data":"%X"`, TxTypeCall, tx.Address, tx.Data)), w, n, err)
	binary.WriteTo([]byte(Fmt(`,"fee":%v,"gas_limit":%v,"input":`, tx.Fee, tx.GasLimit)), w, n, err)
	tx.Input.WriteSignBytes(w, n, err)
	binary.WriteTo([]byte(`}]}`), w, n, err)
}

func (tx *CallTx) String() string {
	return Fmt("CallTx{%v -> %x: %x}", tx.Input, tx.Address, tx.Data)
}

//-----------------------------------------------------------------------------

type BondTx struct {
	PubKey    account.PubKeyEd25519    `json:"pub_key"`
	Signature account.SignatureEd25519 `json:"signature"`
	Inputs    []*TxInput               `json:"inputs"`
	UnbondTo  []*TxOutput              `json:"unbond_to"`
}

func (tx *BondTx) WriteSignBytes(chainID string, w io.Writer, n *int64, err *error) {
	// We hex encode the chain_id so we don't deal with escaping issues.
	binary.WriteTo([]byte(Fmt(`{"chain_id":"%X"`, chainID)), w, n, err)
	binary.WriteTo([]byte(Fmt(`,"tx":[%v,{"inputs":[`, TxTypeBond)), w, n, err)
	for i, in := range tx.Inputs {
		in.WriteSignBytes(w, n, err)
		if i != len(tx.Inputs)-1 {
			binary.WriteTo([]byte(","), w, n, err)
		}
	}
	binary.WriteTo([]byte(Fmt(`],"pub_key":`)), w, n, err)
	binary.WriteTo(binary.JSONBytes(tx.PubKey), w, n, err)
	binary.WriteTo([]byte(`,"unbond_to":[`), w, n, err)
	for i, out := range tx.UnbondTo {
		out.WriteSignBytes(w, n, err)
		if i != len(tx.UnbondTo)-1 {
			binary.WriteTo([]byte(","), w, n, err)
		}
	}
	binary.WriteTo([]byte(`]}]}`), w, n, err)
}

func (tx *BondTx) String() string {
	return Fmt("BondTx{%v: %v -> %v}", tx.PubKey, tx.Inputs, tx.UnbondTo)
}

//-----------------------------------------------------------------------------

type UnbondTx struct {
	Address   []byte                   `json:"address"`
	Height    uint                     `json:"height"`
	Signature account.SignatureEd25519 `json:"signature"`
}

func (tx *UnbondTx) WriteSignBytes(chainID string, w io.Writer, n *int64, err *error) {
	// We hex encode the chain_id so we don't deal with escaping issues.
	binary.WriteTo([]byte(Fmt(`{"chain_id":"%X"`, chainID)), w, n, err)
	binary.WriteTo([]byte(Fmt(`,"tx":[%v,{"address":"%X","height":%v}]}`, TxTypeUnbond, tx.Address, tx.Height)), w, n, err)
}

func (tx *UnbondTx) String() string {
	return Fmt("UnbondTx{%X,%v,%v}", tx.Address, tx.Height, tx.Signature)
}

//-----------------------------------------------------------------------------

type RebondTx struct {
	Address   []byte                   `json:"address"`
	Height    uint                     `json:"height"`
	Signature account.SignatureEd25519 `json:"signature"`
}

func (tx *RebondTx) WriteSignBytes(chainID string, w io.Writer, n *int64, err *error) {
	// We hex encode the chain_id so we don't deal with escaping issues.
	binary.WriteTo([]byte(Fmt(`{"chain_id":"%X"`, chainID)), w, n, err)
	binary.WriteTo([]byte(Fmt(`,"tx":[%v,{"address":"%X","height":%v}]}`, TxTypeRebond, tx.Address, tx.Height)), w, n, err)
}

func (tx *RebondTx) String() string {
	return Fmt("RebondTx{%X,%v,%v}", tx.Address, tx.Height, tx.Signature)
}

//-----------------------------------------------------------------------------

type DupeoutTx struct {
	Address []byte `json:"address"`
	VoteA   Vote   `json:"vote_a"`
	VoteB   Vote   `json:"vote_b"`
}

func (tx *DupeoutTx) WriteSignBytes(chainID string, w io.Writer, n *int64, err *error) {
	panic("DupeoutTx has no sign bytes")
}

func (tx *DupeoutTx) String() string {
	return Fmt("DupeoutTx{%X,%v,%v}", tx.Address, tx.VoteA, tx.VoteB)
}

//-----------------------------------------------------------------------------

func TxId(chainID string, tx Tx) []byte {
	signBytes := account.SignBytes(chainID, tx)
	return binary.BinaryRipemd160(signBytes)
}
