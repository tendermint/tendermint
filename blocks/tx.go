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

Consensus Txs:
3. Bond         New validator posts a bond
4. Unbond       Validator leaves
5. Timeout      Validator times out
6. Dupeout      Validator dupes out (signs twice)


*/

type Tx interface {
	Type() byte
	IsConsensus() bool
	Binary
}

const (
	// Account transactions
	TX_TYPE_SEND = byte(0x01)
	TX_TYPE_NAME = byte(0x02)

	// Consensus transactions
	TX_TYPE_BOND    = byte(0x11)
	TX_TYPE_UNBOND  = byte(0x12)
	TX_TYPE_TIMEOUT = byte(0x13)
	TX_TYPE_DUPEOUT = byte(0x14)
)

func ReadTx(r io.Reader, n *int64, err *error) Tx {
	switch t := ReadByte(r, n, err); t {
	case TX_TYPE_SEND:
		return &SendTx{
			Fee:       ReadUInt64(r, n, err),
			To:        ReadUInt64(r, n, err),
			Amount:    ReadUInt64(r, n, err),
			Signature: ReadSignature(r, n, err),
		}
	case TX_TYPE_NAME:
		return &NameTx{
			Fee:       ReadUInt64(r, n, err),
			Name:      ReadString(r, n, err),
			PubKey:    ReadByteSlice(r, n, err),
			Signature: ReadSignature(r, n, err),
		}
	case TX_TYPE_BOND:
		return &BondTx{
			Fee:       ReadUInt64(r, n, err),
			UnbondTo:  ReadUInt64(r, n, err),
			Amount:    ReadUInt64(r, n, err),
			Signature: ReadSignature(r, n, err),
		}
	case TX_TYPE_UNBOND:
		return &UnbondTx{
			Fee:       ReadUInt64(r, n, err),
			Amount:    ReadUInt64(r, n, err),
			Signature: ReadSignature(r, n, err),
		}
	case TX_TYPE_TIMEOUT:
		return &TimeoutTx{
			AccountId: ReadUInt64(r, n, err),
			Penalty:   ReadUInt64(r, n, err),
		}
	case TX_TYPE_DUPEOUT:
		return &DupeoutTx{
			VoteA: ReadBlockVote(r, n, err),
			VoteB: ReadBlockVote(r, n, err),
		}
	default:
		Panicf("Unknown Tx type %x", t)
		return nil
	}
}

//-----------------------------------------------------------------------------

type SendTx struct {
	Fee    uint64
	To     uint64
	Amount uint64
	Signature
}

func (self *SendTx) Type() byte {
	return TX_TYPE_SEND
}

func (self *SendTx) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, self.Type(), &n, &err)
	WriteUInt64(w, self.Fee, &n, &err)
	WriteUInt64(w, self.To, &n, &err)
	WriteUInt64(w, self.Amount, &n, &err)
	WriteBinary(w, self.Signature, &n, &err)
	return
}

//-----------------------------------------------------------------------------

type NameTx struct {
	Fee    uint64
	Name   string
	PubKey []byte
	Signature
}

func (self *NameTx) Type() byte {
	return TX_TYPE_NAME
}

func (self *NameTx) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, self.Type(), &n, &err)
	WriteUInt64(w, self.Fee, &n, &err)
	WriteString(w, self.Name, &n, &err)
	WriteByteSlice(w, self.PubKey, &n, &err)
	WriteBinary(w, self.Signature, &n, &err)
	return
}

//-----------------------------------------------------------------------------

type BondTx struct {
	Fee      uint64
	UnbondTo uint64
	Amount   uint64
	Signature
}

func (self *BondTx) Type() byte {
	return TX_TYPE_BOND
}

func (self *BondTx) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, self.Type(), &n, &err)
	WriteUInt64(w, self.Fee, &n, &err)
	WriteUInt64(w, self.UnbondTo, &n, &err)
	WriteUInt64(w, self.Amount, &n, &err)
	WriteBinary(w, self.Signature, &n, &err)
	return
}

//-----------------------------------------------------------------------------

type UnbondTx struct {
	Fee    uint64
	Amount uint64
	Signature
}

func (self *UnbondTx) Type() byte {
	return TX_TYPE_UNBOND
}

func (self *UnbondTx) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, self.Type(), &n, &err)
	WriteUInt64(w, self.Fee, &n, &err)
	WriteUInt64(w, self.Amount, &n, &err)
	WriteBinary(w, self.Signature, &n, &err)
	return
}

//-----------------------------------------------------------------------------

type TimeoutTx struct {
	AccountId uint64
	Penalty   uint64
}

func (self *TimeoutTx) Type() byte {
	return TX_TYPE_TIMEOUT
}

func (self *TimeoutTx) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, self.Type(), &n, &err)
	WriteUInt64(w, self.AccountId, &n, &err)
	WriteUInt64(w, self.Penalty, &n, &err)
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

func ReadBlockVote(r io.Reader, n *int64, err *error) BlockVote {
	return BlockVote{
		Height:    ReadUInt64(r, n, err),
		BlockHash: ReadByteSlice(r, n, err),
		Signature: ReadSignature(r, n, err),
	}
}

func (self BlockVote) WriteTo(w io.Writer) (n int64, err error) {
	WriteUInt64(w, self.Height, &n, &err)
	WriteByteSlice(w, self.BlockHash, &n, &err)
	WriteBinary(w, self.Signature, &n, &err)
	return
}

//-----------------------------------------------------------------------------

type DupeoutTx struct {
	VoteA BlockVote
	VoteB BlockVote
}

func (self *DupeoutTx) Type() byte {
	return TX_TYPE_DUPEOUT
}

func (self *DupeoutTx) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, self.Type(), &n, &err)
	WriteBinary(w, self.VoteA, &n, &err)
	WriteBinary(w, self.VoteB, &n, &err)
	return
}
