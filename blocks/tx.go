package blocks

import (
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"io"
)

/*
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
	Signable
	GetSequence() uint
}

const (
	// Account transactions
	TxTypeSend = byte(0x01)
	TxTypeName = byte(0x02)

	// Validation transactions
	TxTypeBond    = byte(0x11)
	TxTypeUnbond  = byte(0x12)
	TxTypeTimeout = byte(0x13)
	TxTypeDupeout = byte(0x14)
)

func ReadTx(r io.Reader, n *int64, err *error) Tx {
	switch t := ReadByte(r, n, err); t {
	case TxTypeSend:
		return &SendTx{
			BaseTx: ReadBaseTx(r, n, err),
			Fee:    ReadUInt64(r, n, err),
			To:     ReadUInt64(r, n, err),
			Amount: ReadUInt64(r, n, err),
		}
	case TxTypeName:
		return &NameTx{
			BaseTx: ReadBaseTx(r, n, err),
			Fee:    ReadUInt64(r, n, err),
			Name:   ReadString(r, n, err),
			PubKey: ReadByteSlice(r, n, err),
		}
	case TxTypeBond:
		return &BondTx{
			BaseTx:   ReadBaseTx(r, n, err),
			Fee:      ReadUInt64(r, n, err),
			UnbondTo: ReadUInt64(r, n, err),
		}
	case TxTypeUnbond:
		return &UnbondTx{
			BaseTx: ReadBaseTx(r, n, err),
			Fee:    ReadUInt64(r, n, err),
		}
	case TxTypeTimeout:
		return &TimeoutTx{
			BaseTx:    ReadBaseTx(r, n, err),
			AccountId: ReadUInt64(r, n, err),
			Penalty:   ReadUInt64(r, n, err),
		}
	case TxTypeDupeout:
		return &DupeoutTx{
			BaseTx: ReadBaseTx(r, n, err),
			VoteA:  *ReadVote(r, n, err),
			VoteB:  *ReadVote(r, n, err),
		}
	default:
		*err = Errorf("Unknown Tx type %X", t)
		return nil
	}
}

//-----------------------------------------------------------------------------

type BaseTx struct {
	Sequence uint
	Signature
}

func ReadBaseTx(r io.Reader, n *int64, err *error) BaseTx {
	return BaseTx{
		Sequence:  ReadUVarInt(r, n, err),
		Signature: ReadSignature(r, n, err),
	}
}

func (tx BaseTx) WriteTo(w io.Writer) (n int64, err error) {
	WriteUVarInt(w, tx.Sequence, &n, &err)
	WriteBinary(w, tx.Signature, &n, &err)
	return
}

func (tx *BaseTx) GetSequence() uint {
	return tx.Sequence
}

func (tx *BaseTx) GetSignature() Signature {
	return tx.Signature
}

func (tx *BaseTx) SetSignature(sig Signature) {
	tx.Signature = sig
}

//-----------------------------------------------------------------------------

type SendTx struct {
	BaseTx
	Fee    uint64
	To     uint64
	Amount uint64
}

func (tx *SendTx) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, TxTypeSend, &n, &err)
	WriteBinary(w, tx.BaseTx, &n, &err)
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

func (tx *NameTx) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, TxTypeName, &n, &err)
	WriteBinary(w, tx.BaseTx, &n, &err)
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
}

func (tx *BondTx) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, TxTypeBond, &n, &err)
	WriteBinary(w, tx.BaseTx, &n, &err)
	WriteUInt64(w, tx.Fee, &n, &err)
	WriteUInt64(w, tx.UnbondTo, &n, &err)
	return
}

//-----------------------------------------------------------------------------

type UnbondTx struct {
	BaseTx
	Fee uint64
}

func (tx *UnbondTx) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, TxTypeUnbond, &n, &err)
	WriteBinary(w, tx.BaseTx, &n, &err)
	WriteUInt64(w, tx.Fee, &n, &err)
	return
}

//-----------------------------------------------------------------------------

type TimeoutTx struct {
	BaseTx
	AccountId uint64
	Penalty   uint64
}

func (tx *TimeoutTx) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, TxTypeTimeout, &n, &err)
	WriteBinary(w, tx.BaseTx, &n, &err)
	WriteUInt64(w, tx.AccountId, &n, &err)
	WriteUInt64(w, tx.Penalty, &n, &err)
	return
}

//-----------------------------------------------------------------------------

type DupeoutTx struct {
	BaseTx
	VoteA Vote
	VoteB Vote
}

func (tx *DupeoutTx) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, TxTypeDupeout, &n, &err)
	WriteBinary(w, tx.BaseTx, &n, &err)
	WriteBinary(w, &tx.VoteA, &n, &err)
	WriteBinary(w, &tx.VoteB, &n, &err)
	return
}
