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
	Type() Byte
	Binary
}

const (
	ADJ_TYPE_BOND    = Byte(0x01)
	ADJ_TYPE_UNBOND  = Byte(0x02)
	ADJ_TYPE_TIMEOUT = Byte(0x03)
	ADJ_TYPE_DUPEOUT = Byte(0x04)
)

func ReadAdjustment(r io.Reader) Adjustment {
	switch t := ReadByte(r); t {
	case ADJ_TYPE_BOND:
		return &Bond{
			Fee:       ReadUInt64(r),
			UnbondTo:  ReadAccountId(r),
			Amount:    ReadUInt64(r),
			Signature: ReadSignature(r),
		}
	case ADJ_TYPE_UNBOND:
		return &Unbond{
			Fee:       ReadUInt64(r),
			Amount:    ReadUInt64(r),
			Signature: ReadSignature(r),
		}
	case ADJ_TYPE_TIMEOUT:
		return &Timeout{
			Account: ReadAccountId(r),
			Penalty: ReadUInt64(r),
		}
	case ADJ_TYPE_DUPEOUT:
		return &Dupeout{
			VoteA: ReadVote(r),
			VoteB: ReadVote(r),
		}
	default:
		Panicf("Unknown Adjustment type %x", t)
		return nil
	}
}

/* Bond < Adjustment */

type Bond struct {
	Fee      UInt64
	UnbondTo AccountId
	Amount   UInt64
	Signature
}

func (self *Bond) Type() Byte {
	return ADJ_TYPE_BOND
}

func (self *Bond) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteOnto(self.Type(), w, n, err)
	n, err = WriteOnto(self.Fee, w, n, err)
	n, err = WriteOnto(self.UnbondTo, w, n, err)
	n, err = WriteOnto(self.Amount, w, n, err)
	n, err = WriteOnto(self.Signature, w, n, err)
	return
}

/* Unbond < Adjustment */

type Unbond struct {
	Fee    UInt64
	Amount UInt64
	Signature
}

func (self *Unbond) Type() Byte {
	return ADJ_TYPE_UNBOND
}

func (self *Unbond) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteOnto(self.Type(), w, n, err)
	n, err = WriteOnto(self.Fee, w, n, err)
	n, err = WriteOnto(self.Amount, w, n, err)
	n, err = WriteOnto(self.Signature, w, n, err)
	return
}

/* Timeout < Adjustment */

type Timeout struct {
	Account AccountId
	Penalty UInt64
}

func (self *Timeout) Type() Byte {
	return ADJ_TYPE_TIMEOUT
}

func (self *Timeout) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteOnto(self.Type(), w, n, err)
	n, err = WriteOnto(self.Account, w, n, err)
	n, err = WriteOnto(self.Penalty, w, n, err)
	return
}

/* Dupeout < Adjustment */

type Dupeout struct {
	VoteA Vote
	VoteB Vote
}

func (self *Dupeout) Type() Byte {
	return ADJ_TYPE_DUPEOUT
}

func (self *Dupeout) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteOnto(self.Type(), w, n, err)
	n, err = WriteOnto(self.VoteA, w, n, err)
	n, err = WriteOnto(self.VoteB, w, n, err)
	return
}
