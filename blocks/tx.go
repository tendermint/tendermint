package blocks

import (
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"io"
)

/*

Tx wire format:

    |T|L...|MMM...|A...|SSS...|

    T  type of the tx (1 byte)
    L  length of M, varint encoded (1+ bytes)
    M  Tx bytes (L bytes)
    A  account number, varint encoded (1+ bytes)
    S  signature of all prior bytes (32 bytes)

Account Txs:
1. Send			Send coins to account
2. Name			Associate account with a name

Validation Txs:
3. Bond         New validator posts a bond
4. Unbond       Validator leaves
5. Timeout      Validator times out
6. Dupeout      Validator dupes out (signs twice)


*/

type Tx interface {
	Type() byte
	GetSequence() uint64
	GetSignature() *Signature
	//IsValidation() bool
	Binary
}

const (
	// Account transactions
	TX_TYPE_SEND = byte(0x01)
	TX_TYPE_NAME = byte(0x02)

	// Validation transactions
	TX_TYPE_BOND    = byte(0x11)
	TX_TYPE_UNBOND  = byte(0x12)
	TX_TYPE_TIMEOUT = byte(0x13)
	TX_TYPE_DUPEOUT = byte(0x14)
)

func ReadTx(r io.Reader, n *int64, err *error) Tx {
	switch t := ReadByte(r, n, err); t {
	case TX_TYPE_SEND:
		return &SendTx{
			BaseTx: ReadBaseTx(r, n, err),
			Fee:    ReadUInt64(r, n, err),
			To:     ReadUInt64(r, n, err),
			Amount: ReadUInt64(r, n, err),
		}
	case TX_TYPE_NAME:
		return &NameTx{
			BaseTx: ReadBaseTx(r, n, err),
			Fee:    ReadUInt64(r, n, err),
			Name:   ReadString(r, n, err),
			PubKey: ReadByteSlice(r, n, err),
		}
	case TX_TYPE_BOND:
		return &BondTx{
			BaseTx:   ReadBaseTx(r, n, err),
			Fee:      ReadUInt64(r, n, err),
			UnbondTo: ReadUInt64(r, n, err),
			Amount:   ReadUInt64(r, n, err),
		}
	case TX_TYPE_UNBOND:
		return &UnbondTx{
			BaseTx: ReadBaseTx(r, n, err),
			Fee:    ReadUInt64(r, n, err),
			Amount: ReadUInt64(r, n, err),
		}
	case TX_TYPE_TIMEOUT:
		return &TimeoutTx{
			BaseTx:    ReadBaseTx(r, n, err),
			AccountId: ReadUInt64(r, n, err),
			Penalty:   ReadUInt64(r, n, err),
		}
	case TX_TYPE_DUPEOUT:
		return &DupeoutTx{
			BaseTx: ReadBaseTx(r, n, err),
			VoteA:  *ReadBlockVote(r, n, err),
			VoteB:  *ReadBlockVote(r, n, err),
		}
	default:
		Panicf("Unknown Tx type %x", t)
		return nil
	}
}

//-----------------------------------------------------------------------------

type BaseTx struct {
	Sequence uint64
	Signature
}

func ReadBaseTx(r io.Reader, n *int64, err *error) BaseTx {
	return BaseTx{
		Sequence:  ReadUVarInt(r, n, err),
		Signature: ReadSignature(r, n, err),
	}
}

func (tx *BaseTx) GetSequence() uint64 {
	return tx.Sequence
}

func (tx *BaseTx) GetSignature() *Signature {
	return &tx.Signature
}

func (tx *BaseTx) WriteTo(w io.Writer) (n int64, err error) {
	WriteUVarInt(w, tx.Sequence, &n, &err)
	WriteBinary(w, tx.Signature, &n, &err)
	return
}

//-----------------------------------------------------------------------------

type SendTx struct {
	BaseTx
	Fee    uint64
	To     uint64
	Amount uint64
}

func (tx *SendTx) Type() byte {
	return TX_TYPE_SEND
}

func (tx *SendTx) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, tx.Type(), &n, &err)
	WriteBinary(w, &tx.BaseTx, &n, &err)
	WriteUInt64(w, tx.Fee, &n, &err)
	WriteUInt64(w, tx.To, &n, &err)
	WriteUInt64(w, tx.Amount, &n, &err)
	return
}

//-----------------------------------------------------------------------------

type NameTx struct {
	BaseTx
	Fee    uint64
	Name   string
	PubKey []byte
}

func (tx *NameTx) Type() byte {
	return TX_TYPE_NAME
}

func (tx *NameTx) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, tx.Type(), &n, &err)
	WriteBinary(w, &tx.BaseTx, &n, &err)
	WriteUInt64(w, tx.Fee, &n, &err)
	WriteString(w, tx.Name, &n, &err)
	WriteByteSlice(w, tx.PubKey, &n, &err)
	return
}

//-----------------------------------------------------------------------------

type BondTx struct {
	BaseTx
	Fee      uint64
	UnbondTo uint64
	Amount   uint64
}

func (tx *BondTx) Type() byte {
	return TX_TYPE_BOND
}

func (tx *BondTx) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, tx.Type(), &n, &err)
	WriteBinary(w, &tx.BaseTx, &n, &err)
	WriteUInt64(w, tx.Fee, &n, &err)
	WriteUInt64(w, tx.UnbondTo, &n, &err)
	WriteUInt64(w, tx.Amount, &n, &err)
	return
}

//-----------------------------------------------------------------------------

type UnbondTx struct {
	BaseTx
	Fee    uint64
	Amount uint64
}

func (tx *UnbondTx) Type() byte {
	return TX_TYPE_UNBOND
}

func (tx *UnbondTx) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, tx.Type(), &n, &err)
	WriteBinary(w, &tx.BaseTx, &n, &err)
	WriteUInt64(w, tx.Fee, &n, &err)
	WriteUInt64(w, tx.Amount, &n, &err)
	return
}

//-----------------------------------------------------------------------------

type TimeoutTx struct {
	BaseTx
	AccountId uint64
	Penalty   uint64
}

func (tx *TimeoutTx) Type() byte {
	return TX_TYPE_TIMEOUT
}

func (tx *TimeoutTx) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, tx.Type(), &n, &err)
	WriteBinary(w, &tx.BaseTx, &n, &err)
	WriteUInt64(w, tx.AccountId, &n, &err)
	WriteUInt64(w, tx.Penalty, &n, &err)
	return
}

//-----------------------------------------------------------------------------

/*
The full vote structure is only needed when presented as evidence.
Typically only the signature is passed around, as the hash & height are implied.
*/
type BlockVote struct {
	Height    uint64
	BlockHash []byte
	Signature
}

func ReadBlockVote(r io.Reader, n *int64, err *error) *BlockVote {
	return &BlockVote{
		Height:    ReadUInt64(r, n, err),
		BlockHash: ReadByteSlice(r, n, err),
		Signature: ReadSignature(r, n, err),
	}
}

func (tx BlockVote) WriteTo(w io.Writer) (n int64, err error) {
	WriteUInt64(w, tx.Height, &n, &err)
	WriteByteSlice(w, tx.BlockHash, &n, &err)
	WriteBinary(w, tx.Signature, &n, &err)
	return
}

//-----------------------------------------------------------------------------

type DupeoutTx struct {
	BaseTx
	VoteA BlockVote
	VoteB BlockVote
}

func (tx *DupeoutTx) Type() byte {
	return TX_TYPE_DUPEOUT
}

func (tx *DupeoutTx) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, tx.Type(), &n, &err)
	WriteBinary(w, &tx.BaseTx, &n, &err)
	WriteBinary(w, tx.VoteA, &n, &err)
	WriteBinary(w, tx.VoteB, &n, &err)
	return
}
