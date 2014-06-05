package blocks

import (
    . "github.com/tendermint/tendermint/binary"
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
    Type()      Byte
    Binary
}

const (
    ADJ_TYPE_BOND =      Byte(0x01)
    ADJ_TYPE_UNBOND =    Byte(0x02)
    ADJ_TYPE_TIMEOUT =   Byte(0x03)
    ADJ_TYPE_GUILTOUT =  Byte(0x04)
)

func ReadAdjustment(r io.Reader) Adjustment {
    return nil
}


/* Bond < Adjustment */

type Bond struct {
    Signature
    Fee             UInt64
    UnbondTo        AccountId
    Amount          UInt64
}

func (self *Bond) Type() Byte {
    return ADJ_TYPE_BOND
}

func (self *Bond) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int64
    n_, err = self.Type().WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Signature.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Fee.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.UnbondTo.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Amount.WriteTo(w)
    n += n_; return
}


/* Unbond < Adjustment */

type Unbond struct {
    Signature
    Fee             UInt64
    Amount          UInt64
}

func (self *Unbond) Type() Byte {
    return ADJ_TYPE_UNBOND
}

func (self *Unbond) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int64
    n_, err = self.Type().WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Signature.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Fee.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Amount.WriteTo(w)
    n += n_; return
}


/* Timeout < Adjustment */

type Timeout struct {
    Account         AccountId
    Penalty         UInt64
}

func (self *Timeout) Type() Byte {
    return ADJ_TYPE_TIMEOUT
}

func (self *Timeout) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int64
    n_, err = self.Type().WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Account.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Penalty.WriteTo(w)
    n += n_; return
}


/* Dupeout < Adjustment */

type Dupeout struct {
    VoteA           Vote
    VoteB           Vote
}

func (self *Dupeout) Type() Byte {
    return ADJ_TYPE_DUPEOUT
}

func (self *Dupeout) WriteTo(w io.Writer) (n int64, err error) {
    var n_ int64
    n_, err = self.Type().WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Account.WriteTo(w)
    n += n_; if err != nil { return n, err }
    n_, err = self.Penalty.WriteTo(w)
    n += n_; return
}
