package types

import (
	"bytes"
	"fmt"
	"strings"
	"sync"

	"github.com/pkg/errors"

	cmn "github.com/tendermint/tendermint/libs/common"
)

// UNSTABLE
// XXX: duplicate of p2p.ID to avoid dependence between packages.
// Perhaps we can have a minimal types package containing this (and other things?)
// that both `types` and `p2p` import ?
type P2PID string

/*
	VoteSet helps collect signatures from validators at each height+round for a
	predefined vote type.

	We need VoteSet to be able to keep track of conflicting votes when validators
	double-sign.  Yet, we can't keep track of *all* the votes seen, as that could
	be a DoS attack vector.

	There are two storage areas for votes.
	1. voteSet.votes
	2. voteSet.votesByBlock

	`.votes` is the "canonical" list of votes.  It always has at least one vote,
	if a vote from a validator had been seen at all.  Usually it keeps track of
	the first vote seen, but when a 2/3 majority is found, votes for that get
	priority and are copied over from `.votesByBlock`.

	`.votesByBlock` keeps track of a list of votes for a particular block.  There
	are two ways a &blockVotes{} gets created in `.votesByBlock`.
	1. the first vote seen by a validator was for the particular block.
	2. a peer claims to have seen 2/3 majority for the particular block.

	Since the first vote from a validator will always get added in `.votesByBlock`
	, all votes in `.votes` will have a corresponding entry in `.votesByBlock`.

	When a &blockVotes{} in `.votesByBlock` reaches a 2/3 majority quorum, its
	votes are copied into `.votes`.

	All this is memory bounded because conflicting votes only get added if a peer
	told us to track that block, each peer only gets to tell us 1 such block, and,
	there's only a limited number of peers.

	NOTE: Assumes that the sum total of voting power does not exceed MaxUInt64.
*/
type VoteSet struct {
	chainID string
	height  int64
	round   int
	type_   SignedMsgType
	valSet  *ValidatorSet

	mtx           sync.Mutex
	votesBitArray *cmn.BitArray
	votes         []*Vote                // Primary votes to share
	sum           int64                  // Sum of voting power for seen votes, discounting conflicts
	maj23         *BlockID               // First 2/3 majority seen
	votesByBlock  map[string]*blockVotes // string(blockHash|blockParts) -> blockVotes
	peerMaj23s    map[P2PID]BlockID      // Maj23 for each peer
}

// Constructs a new VoteSet struct used to accumulate votes for given height/round.
func NewVoteSet(chainID string, height int64, round int, type_ SignedMsgType, valSet *ValidatorSet) *VoteSet {
	if height == 0 {
		cmn.PanicSanity("Cannot make VoteSet for height == 0, doesn't make sense.")
	}
	return &VoteSet{
		chainID:       chainID,
		height:        height,
		round:         round,
		type_:         type_,
		valSet:        valSet,
		votesBitArray: cmn.NewBitArray(valSet.Size()),
		votes:         make([]*Vote, valSet.Size()),
		sum:           0,
		maj23:         nil,
		votesByBlock:  make(map[string]*blockVotes, valSet.Size()),
		peerMaj23s:    make(map[P2PID]BlockID),
	}
}

func (voteSet *VoteSet) ChainID() string {
	return voteSet.chainID
}

func (voteSet *VoteSet) Height() int64 {
	if voteSet == nil {
		return 0
	}
	return voteSet.height
}

func (voteSet *VoteSet) Round() int {
	if voteSet == nil {
		return -1
	}
	return voteSet.round
}

func (voteSet *VoteSet) Type() byte {
	if voteSet == nil {
		return 0x00
	}
	return byte(voteSet.type_)
}

func (voteSet *VoteSet) Size() int {
	if voteSet == nil {
		return 0
	}
	return voteSet.valSet.Size()
}

// Returns added=true if vote is valid and new.
// Otherwise returns err=ErrVote[
//		UnexpectedStep | InvalidIndex | InvalidAddress |
//		InvalidSignature | InvalidBlockHash | ConflictingVotes ]
// Duplicate votes return added=false, err=nil.
// Conflicting votes return added=*, err=ErrVoteConflictingVotes.
// NOTE: vote should not be mutated after adding.
// NOTE: VoteSet must not be nil
// NOTE: Vote must not be nil
func (voteSet *VoteSet) AddVote(vote *Vote) (added bool, err error) {
	if voteSet == nil {
		cmn.PanicSanity("AddVote() on nil VoteSet")
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()

	return voteSet.addVote(vote)
}

// NOTE: Validates as much as possible before attempting to verify the signature.
func (voteSet *VoteSet) addVote(vote *Vote) (added bool, err error) {
	if vote == nil {
		return false, ErrVoteNil
	}
	valIndex := vote.ValidatorIndex
	valAddr := vote.ValidatorAddress
	blockKey := vote.BlockID.Key()

	// Ensure that validator index was set
	if valIndex < 0 {
		return false, errors.Wrap(ErrVoteInvalidValidatorIndex, "Index < 0")
	} else if len(valAddr) == 0 {
		return false, errors.Wrap(ErrVoteInvalidValidatorAddress, "Empty address")
	}

	// Make sure the step matches.
	if (vote.Height != voteSet.height) ||
		(vote.Round != voteSet.round) ||
		(vote.Type != voteSet.type_) {
		return false, errors.Wrapf(ErrVoteUnexpectedStep, "Expected %d/%d/%d, but got %d/%d/%d",
			voteSet.height, voteSet.round, voteSet.type_,
			vote.Height, vote.Round, vote.Type)
	}

	// Ensure that signer is a validator.
	lookupAddr, val := voteSet.valSet.GetByIndex(valIndex)
	if val == nil {
		return false, errors.Wrapf(ErrVoteInvalidValidatorIndex,
			"Cannot find validator %d in valSet of size %d", valIndex, voteSet.valSet.Size())
	}

	// Ensure that the signer has the right address.
	if !bytes.Equal(valAddr, lookupAddr) {
		return false, errors.Wrapf(ErrVoteInvalidValidatorAddress,
			"vote.ValidatorAddress (%X) does not match address (%X) for vote.ValidatorIndex (%d)\nEnsure the genesis file is correct across all validators.",
			valAddr, lookupAddr, valIndex)
	}

	// If we already know of this vote, return false.
	if existing, ok := voteSet.getVote(valIndex, blockKey); ok {
		if bytes.Equal(existing.Signature, vote.Signature) {
			return false, nil // duplicate
		}
		return false, errors.Wrapf(ErrVoteNonDeterministicSignature, "Existing vote: %v; New vote: %v", existing, vote)
	}

	// Check signature.
	if err := vote.Verify(voteSet.chainID, val.PubKey); err != nil {
		return false, errors.Wrapf(err, "Failed to verify vote with ChainID %s and PubKey %s", voteSet.chainID, val.PubKey)
	}

	// Add vote and get conflicting vote if any.
	added, conflicting := voteSet.addVerifiedVote(vote, blockKey, val.VotingPower)
	if conflicting != nil {
		return added, NewConflictingVoteError(val, conflicting, vote)
	}
	if !added {
		cmn.PanicSanity("Expected to add non-conflicting vote")
	}
	return added, nil
}

// Returns (vote, true) if vote exists for valIndex and blockKey.
func (voteSet *VoteSet) getVote(valIndex int, blockKey string) (vote *Vote, ok bool) {
	if existing := voteSet.votes[valIndex]; existing != nil && existing.BlockID.Key() == blockKey {
		return existing, true
	}
	if existing := voteSet.votesByBlock[blockKey].getByIndex(valIndex); existing != nil {
		return existing, true
	}
	return nil, false
}

// Assumes signature is valid.
// If conflicting vote exists, returns it.
func (voteSet *VoteSet) addVerifiedVote(vote *Vote, blockKey string, votingPower int64) (added bool, conflicting *Vote) {
	valIndex := vote.ValidatorIndex

	// Already exists in voteSet.votes?
	if existing := voteSet.votes[valIndex]; existing != nil {
		if existing.BlockID.Equals(vote.BlockID) {
			cmn.PanicSanity("addVerifiedVote does not expect duplicate votes")
		} else {
			conflicting = existing
		}
		// Replace vote if blockKey matches voteSet.maj23.
		if voteSet.maj23 != nil && voteSet.maj23.Key() == blockKey {
			voteSet.votes[valIndex] = vote
			voteSet.votesBitArray.SetIndex(valIndex, true)
		}
		// Otherwise don't add it to voteSet.votes
	} else {
		// Add to voteSet.votes and incr .sum
		voteSet.votes[valIndex] = vote
		voteSet.votesBitArray.SetIndex(valIndex, true)
		voteSet.sum += votingPower
	}

	votesByBlock, ok := voteSet.votesByBlock[blockKey]
	if ok {
		if conflicting != nil && !votesByBlock.peerMaj23 {
			// There's a conflict and no peer claims that this block is special.
			return false, conflicting
		}
		// We'll add the vote in a bit.
	} else {
		// .votesByBlock doesn't exist...
		if conflicting != nil {
			// ... and there's a conflicting vote.
			// We're not even tracking this blockKey, so just forget it.
			return false, conflicting
		}
		// ... and there's no conflicting vote.
		// Start tracking this blockKey
		votesByBlock = newBlockVotes(false, voteSet.valSet.Size())
		voteSet.votesByBlock[blockKey] = votesByBlock
		// We'll add the vote in a bit.
	}

	// Before adding to votesByBlock, see if we'll exceed quorum
	origSum := votesByBlock.sum
	quorum := voteSet.valSet.TotalVotingPower()*2/3 + 1

	// Add vote to votesByBlock
	votesByBlock.addVerifiedVote(vote, votingPower)

	// If we just crossed the quorum threshold and have 2/3 majority...
	if origSum < quorum && quorum <= votesByBlock.sum {
		// Only consider the first quorum reached
		if voteSet.maj23 == nil {
			maj23BlockID := vote.BlockID
			voteSet.maj23 = &maj23BlockID
			// And also copy votes over to voteSet.votes
			for i, vote := range votesByBlock.votes {
				if vote != nil {
					voteSet.votes[i] = vote
				}
			}
		}
	}

	return true, conflicting
}

// If a peer claims that it has 2/3 majority for given blockKey, call this.
// NOTE: if there are too many peers, or too much peer churn,
// this can cause memory issues.
// TODO: implement ability to remove peers too
// NOTE: VoteSet must not be nil
func (voteSet *VoteSet) SetPeerMaj23(peerID P2PID, blockID BlockID) error {
	if voteSet == nil {
		cmn.PanicSanity("SetPeerMaj23() on nil VoteSet")
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()

	blockKey := blockID.Key()

	// Make sure peer hasn't already told us something.
	if existing, ok := voteSet.peerMaj23s[peerID]; ok {
		if existing.Equals(blockID) {
			return nil // Nothing to do
		}
		return fmt.Errorf("SetPeerMaj23: Received conflicting blockID from peer %v. Got %v, expected %v",
			peerID, blockID, existing)
	}
	voteSet.peerMaj23s[peerID] = blockID

	// Create .votesByBlock entry if needed.
	votesByBlock, ok := voteSet.votesByBlock[blockKey]
	if ok {
		if votesByBlock.peerMaj23 {
			return nil // Nothing to do
		}
		votesByBlock.peerMaj23 = true
		// No need to copy votes, already there.
	} else {
		votesByBlock = newBlockVotes(true, voteSet.valSet.Size())
		voteSet.votesByBlock[blockKey] = votesByBlock
		// No need to copy votes, no votes to copy over.
	}
	return nil
}

func (voteSet *VoteSet) BitArray() *cmn.BitArray {
	if voteSet == nil {
		return nil
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.votesBitArray.Copy()
}

func (voteSet *VoteSet) BitArrayByBlockID(blockID BlockID) *cmn.BitArray {
	if voteSet == nil {
		return nil
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	votesByBlock, ok := voteSet.votesByBlock[blockID.Key()]
	if ok {
		return votesByBlock.bitArray.Copy()
	}
	return nil
}

// NOTE: if validator has conflicting votes, returns "canonical" vote
func (voteSet *VoteSet) GetByIndex(valIndex int) *Vote {
	if voteSet == nil {
		return nil
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.votes[valIndex]
}

func (voteSet *VoteSet) GetByAddress(address []byte) *Vote {
	if voteSet == nil {
		return nil
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	valIndex, val := voteSet.valSet.GetByAddress(address)
	if val == nil {
		cmn.PanicSanity("GetByAddress(address) returned nil")
	}
	return voteSet.votes[valIndex]
}

func (voteSet *VoteSet) HasTwoThirdsMajority() bool {
	if voteSet == nil {
		return false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.maj23 != nil
}

func (voteSet *VoteSet) IsCommit() bool {
	if voteSet == nil {
		return false
	}
	if voteSet.type_ != PrecommitType {
		return false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.maj23 != nil
}

func (voteSet *VoteSet) HasTwoThirdsAny() bool {
	if voteSet == nil {
		return false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.sum > voteSet.valSet.TotalVotingPower()*2/3
}

func (voteSet *VoteSet) HasAll() bool {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.sum == voteSet.valSet.TotalVotingPower()
}

// If there was a +2/3 majority for blockID, return blockID and true.
// Else, return the empty BlockID{} and false.
func (voteSet *VoteSet) TwoThirdsMajority() (blockID BlockID, ok bool) {
	if voteSet == nil {
		return BlockID{}, false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	if voteSet.maj23 != nil {
		return *voteSet.maj23, true
	}
	return BlockID{}, false
}

//--------------------------------------------------------------------------------
// Strings and JSON

func (voteSet *VoteSet) String() string {
	if voteSet == nil {
		return "nil-VoteSet"
	}
	return voteSet.StringIndented("")
}

func (voteSet *VoteSet) StringIndented(indent string) string {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	voteStrings := make([]string, len(voteSet.votes))
	for i, vote := range voteSet.votes {
		if vote == nil {
			voteStrings[i] = "nil-Vote"
		} else {
			voteStrings[i] = vote.String()
		}
	}
	return fmt.Sprintf(`VoteSet{
%s  H:%v R:%v T:%v
%s  %v
%s  %v
%s  %v
%s}`,
		indent, voteSet.height, voteSet.round, voteSet.type_,
		indent, strings.Join(voteStrings, "\n"+indent+"  "),
		indent, voteSet.votesBitArray,
		indent, voteSet.peerMaj23s,
		indent)
}

// Marshal the VoteSet to JSON. Same as String(), just in JSON,
// and without the height/round/type_ (since its already included in the votes).
func (voteSet *VoteSet) MarshalJSON() ([]byte, error) {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return cdc.MarshalJSON(VoteSetJSON{
		voteSet.voteStrings(),
		voteSet.bitArrayString(),
		voteSet.peerMaj23s,
	})
}

// More human readable JSON of the vote set
// NOTE: insufficient for unmarshalling from (compressed votes)
// TODO: make the peerMaj23s nicer to read (eg just the block hash)
type VoteSetJSON struct {
	Votes         []string          `json:"votes"`
	VotesBitArray string            `json:"votes_bit_array"`
	PeerMaj23s    map[P2PID]BlockID `json:"peer_maj_23s"`
}

// Return the bit-array of votes including
// the fraction of power that has voted like:
// "BA{29:xx__x__x_x___x__x_______xxx__} 856/1304 = 0.66"
func (voteSet *VoteSet) BitArrayString() string {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.bitArrayString()
}

func (voteSet *VoteSet) bitArrayString() string {
	bAString := voteSet.votesBitArray.String()
	voted, total, fracVoted := voteSet.sumTotalFrac()
	return fmt.Sprintf("%s %d/%d = %.2f", bAString, voted, total, fracVoted)
}

// Returns a list of votes compressed to more readable strings.
func (voteSet *VoteSet) VoteStrings() []string {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.voteStrings()
}

func (voteSet *VoteSet) voteStrings() []string {
	voteStrings := make([]string, len(voteSet.votes))
	for i, vote := range voteSet.votes {
		if vote == nil {
			voteStrings[i] = "nil-Vote"
		} else {
			voteStrings[i] = vote.String()
		}
	}
	return voteStrings
}

func (voteSet *VoteSet) StringShort() string {
	if voteSet == nil {
		return "nil-VoteSet"
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	_, _, frac := voteSet.sumTotalFrac()
	return fmt.Sprintf(`VoteSet{H:%v R:%v T:%v +2/3:%v(%v) %v %v}`,
		voteSet.height, voteSet.round, voteSet.type_, voteSet.maj23, frac, voteSet.votesBitArray, voteSet.peerMaj23s)
}

// return the power voted, the total, and the fraction
func (voteSet *VoteSet) sumTotalFrac() (int64, int64, float64) {
	voted, total := voteSet.sum, voteSet.valSet.TotalVotingPower()
	fracVoted := float64(voted) / float64(total)
	return voted, total, fracVoted
}

//--------------------------------------------------------------------------------
// Commit

func (voteSet *VoteSet) MakeCommit() *Commit {
	if voteSet.type_ != PrecommitType {
		cmn.PanicSanity("Cannot MakeCommit() unless VoteSet.Type is PrecommitType")
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()

	// Make sure we have a 2/3 majority
	if voteSet.maj23 == nil {
		cmn.PanicSanity("Cannot MakeCommit() unless a blockhash has +2/3")
	}

	// For every validator, get the precommit
	votesCopy := make([]*Vote, len(voteSet.votes))
	copy(votesCopy, voteSet.votes)
	return &Commit{
		BlockID:    *voteSet.maj23,
		Precommits: votesCopy,
	}
}

//--------------------------------------------------------------------------------

/*
	Votes for a particular block
	There are two ways a *blockVotes gets created for a blockKey.
	1. first (non-conflicting) vote of a validator w/ blockKey (peerMaj23=false)
	2. A peer claims to have a 2/3 majority w/ blockKey (peerMaj23=true)
*/
type blockVotes struct {
	peerMaj23 bool          // peer claims to have maj23
	bitArray  *cmn.BitArray // valIndex -> hasVote?
	votes     []*Vote       // valIndex -> *Vote
	sum       int64         // vote sum
}

func newBlockVotes(peerMaj23 bool, numValidators int) *blockVotes {
	return &blockVotes{
		peerMaj23: peerMaj23,
		bitArray:  cmn.NewBitArray(numValidators),
		votes:     make([]*Vote, numValidators),
		sum:       0,
	}
}

func (vs *blockVotes) addVerifiedVote(vote *Vote, votingPower int64) {
	valIndex := vote.ValidatorIndex
	if existing := vs.votes[valIndex]; existing == nil {
		vs.bitArray.SetIndex(valIndex, true)
		vs.votes[valIndex] = vote
		vs.sum += votingPower
	}
}

func (vs *blockVotes) getByIndex(index int) *Vote {
	if vs == nil {
		return nil
	}
	return vs.votes[index]
}

//--------------------------------------------------------------------------------

// Common interface between *consensus.VoteSet and types.Commit
type VoteSetReader interface {
	Height() int64
	Round() int
	Type() byte
	Size() int
	BitArray() *cmn.BitArray
	GetByIndex(int) *Vote
	IsCommit() bool
}
