package blocks

import (
    "github.com/tendermint/tendermint/merkle"
    "io"
)

/* Validation */

type BlockValidation struct {
    Votes       merkle.Tree
    Adjustments merkle.Tree
}

/* Votes */

type Votes struct {
    Tree            merkle.Tree
}

func NewVotesFromHash(hash []byte) *Votes {
    return nil
}

func (self *Votes) GetVote(validator AccountId) *Vote {
    return nil
}

func (self *Votes) PutVote(vote *Vote) bool {
    return false
}

func (self *Votes) Verify() bool {
    return false
}

func (self *Votes) WriteTo(w io.Writer) (n int64, err error) {
    return 0, nil
}

/*

The canonical representation of a Vote for signing:

    |L|NNN...|h...|HHH...|

    L  length of network name (1 byte)
    N  name of network (max 255 bytes)
    h  height of block, varint encoded (1+ bytes)
    H  blockhash voted for height h

The wire format of a vote is usually simply a Signature.
The network name, height, and blockhash are omitted because
they are implied from context.  When it is not, e.g. evidence
for double-signing, the wire format is:

    |h...|HHH...|A...|SSS...|

*/

type Vote struct {
    Signature

    Height          uint32
    BlockHash       []byte
}

/* |h...|HHH...|A...|SSS...| */
func ReadVote(buf []byte, start int) (*Vote, int) {
    return nil, 0
}

/* |L|NNN...|h...|HHH...| */
func (self *Vote) WriteTo(w io.Writer) (n int64, err error) {
    return 0, nil
}

/* Adjustments */
