package blocks

import (
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"io"
)

/* Adjustment

1. Bond         New validator posts a bond
2. Unbond       Validator leaves
3. Timeout      Validator times out
4. Dupeout      Validator dupes out (signs twice)

TODO: signing a bad checkpoint (block)
*/
type Adjustment interface {
	Type() byte
	Binary
}

const (
	ADJ_TYPE_BOND    = byte(0x01)
	ADJ_TYPE_UNBOND  = byte(0x02)
	ADJ_TYPE_TIMEOUT = byte(0x03)
	ADJ_TYPE_DUPEOUT = byte(0x04)
)

func ReadAdjustment(r io.Reader, n *int64, err *error) Adjustment {
	switch t := ReadByte(r, n, err); t {
	case ADJ_TYPE_BOND:
		return &Bond{
			Fee:       ReadUInt64(r, n, err),
			UnbondTo:  ReadUInt64(r, n, err),
			Amount:    ReadUInt64(r, n, err),
			Signature: ReadSignature(r, n, err),
		}
	case ADJ_TYPE_UNBOND:
		return &Unbond{
			Fee:       ReadUInt64(r, n, err),
			Amount:    ReadUInt64(r, n, err),
			Signature: ReadSignature(r, n, err),
		}
	case ADJ_TYPE_TIMEOUT:
		return &Timeout{
			AccountId: ReadUInt64(r, n, err),
			Penalty:   ReadUInt64(r, n, err),
		}
	case ADJ_TYPE_DUPEOUT:
		return &Dupeout{
			VoteA: ReadBlockVote(r, n, err),
			VoteB: ReadBlockVote(r, n, err),
		}
	default:
		Panicf("Unknown Adjustment type %x", t)
		return nil
	}
}

//-----------------------------------------------------------------------------

/* Bond < Adjustment */
type Bond struct {
	Fee      uint64
	UnbondTo uint64
	Amount   uint64
	Signature
}

func (self *Bond) Type() byte {
	return ADJ_TYPE_BOND
}

func (self *Bond) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, self.Type(), &n, &err)
	WriteUInt64(w, self.Fee, &n, &err)
	WriteUInt64(w, self.UnbondTo, &n, &err)
	WriteUInt64(w, self.Amount, &n, &err)
	WriteBinary(w, self.Signature, &n, &err)
	return
}

//-----------------------------------------------------------------------------

/* Unbond < Adjustment */
type Unbond struct {
	Fee    uint64
	Amount uint64
	Signature
}

func (self *Unbond) Type() byte {
	return ADJ_TYPE_UNBOND
}

func (self *Unbond) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, self.Type(), &n, &err)
	WriteUInt64(w, self.Fee, &n, &err)
	WriteUInt64(w, self.Amount, &n, &err)
	WriteBinary(w, self.Signature, &n, &err)
	return
}

//-----------------------------------------------------------------------------

/* Timeout < Adjustment */
type Timeout struct {
	AccountId uint64
	Penalty   uint64
}

func (self *Timeout) Type() byte {
	return ADJ_TYPE_TIMEOUT
}

func (self *Timeout) WriteTo(w io.Writer) (n int64, err error) {
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

/* Dupeout < Adjustment */
type Dupeout struct {
	VoteA BlockVote
	VoteB BlockVote
}

func (self *Dupeout) Type() byte {
	return ADJ_TYPE_DUPEOUT
}

func (self *Dupeout) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, self.Type(), &n, &err)
	WriteBinary(w, self.VoteA, &n, &err)
	WriteBinary(w, self.VoteB, &n, &err)
	return
}
