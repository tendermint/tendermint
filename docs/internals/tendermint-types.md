

# types
`import "github.com/tendermint/tendermint/types"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)

## <a name="pkg-overview">Overview</a>



## <a name="pkg-index">Index</a>
* [Constants](#pkg-constants)
* [Variables](#pkg-variables)
* [func AddListenerForEvent(evsw EventSwitch, id, event string, cb func(data TMEventData))](#AddListenerForEvent)
* [func EventStringBond() string](#EventStringBond)
* [func EventStringCompleteProposal() string](#EventStringCompleteProposal)
* [func EventStringDupeout() string](#EventStringDupeout)
* [func EventStringFork() string](#EventStringFork)
* [func EventStringLock() string](#EventStringLock)
* [func EventStringNewBlock() string](#EventStringNewBlock)
* [func EventStringNewBlockHeader() string](#EventStringNewBlockHeader)
* [func EventStringNewRound() string](#EventStringNewRound)
* [func EventStringNewRoundStep() string](#EventStringNewRoundStep)
* [func EventStringPolka() string](#EventStringPolka)
* [func EventStringRebond() string](#EventStringRebond)
* [func EventStringRelock() string](#EventStringRelock)
* [func EventStringTimeoutPropose() string](#EventStringTimeoutPropose)
* [func EventStringTimeoutWait() string](#EventStringTimeoutWait)
* [func EventStringTx(tx Tx) string](#EventStringTx)
* [func EventStringUnbond() string](#EventStringUnbond)
* [func EventStringUnlock() string](#EventStringUnlock)
* [func EventStringVote() string](#EventStringVote)
* [func FireEventCompleteProposal(fireable events.Fireable, rs EventDataRoundState)](#FireEventCompleteProposal)
* [func FireEventLock(fireable events.Fireable, rs EventDataRoundState)](#FireEventLock)
* [func FireEventNewBlock(fireable events.Fireable, block EventDataNewBlock)](#FireEventNewBlock)
* [func FireEventNewBlockHeader(fireable events.Fireable, header EventDataNewBlockHeader)](#FireEventNewBlockHeader)
* [func FireEventNewRound(fireable events.Fireable, rs EventDataRoundState)](#FireEventNewRound)
* [func FireEventNewRoundStep(fireable events.Fireable, rs EventDataRoundState)](#FireEventNewRoundStep)
* [func FireEventPolka(fireable events.Fireable, rs EventDataRoundState)](#FireEventPolka)
* [func FireEventRelock(fireable events.Fireable, rs EventDataRoundState)](#FireEventRelock)
* [func FireEventTimeoutPropose(fireable events.Fireable, rs EventDataRoundState)](#FireEventTimeoutPropose)
* [func FireEventTimeoutWait(fireable events.Fireable, rs EventDataRoundState)](#FireEventTimeoutWait)
* [func FireEventTx(fireable events.Fireable, tx EventDataTx)](#FireEventTx)
* [func FireEventUnlock(fireable events.Fireable, rs EventDataRoundState)](#FireEventUnlock)
* [func FireEventVote(fireable events.Fireable, vote EventDataVote)](#FireEventVote)
* [func HashSignBytes(chainID string, o Signable) []byte](#HashSignBytes)
* [func IsVoteTypeValid(type_ byte) bool](#IsVoteTypeValid)
* [func SignBytes(chainID string, o Signable) []byte](#SignBytes)
* [type Block](#Block)
  * [func MakeBlock(height int, chainID string, txs []Tx, commit *Commit, prevBlockID BlockID, valHash, appHash []byte, partSize int) (*Block, *PartSet)](#MakeBlock)
  * [func (b *Block) FillHeader()](#Block.FillHeader)
  * [func (b *Block) Hash() []byte](#Block.Hash)
  * [func (b *Block) HashesTo(hash []byte) bool](#Block.HashesTo)
  * [func (b *Block) MakePartSet(partSize int) *PartSet](#Block.MakePartSet)
  * [func (b *Block) String() string](#Block.String)
  * [func (b *Block) StringIndented(indent string) string](#Block.StringIndented)
  * [func (b *Block) StringShort() string](#Block.StringShort)
  * [func (b *Block) ValidateBasic(chainID string, lastBlockHeight int, lastBlockID BlockID, lastBlockTime time.Time, appHash []byte) error](#Block.ValidateBasic)
* [type BlockID](#BlockID)
  * [func (blockID BlockID) Equals(other BlockID) bool](#BlockID.Equals)
  * [func (blockID BlockID) IsZero() bool](#BlockID.IsZero)
  * [func (blockID BlockID) Key() string](#BlockID.Key)
  * [func (blockID BlockID) String() string](#BlockID.String)
  * [func (blockID BlockID) WriteSignBytes(w io.Writer, n *int, err *error)](#BlockID.WriteSignBytes)
* [type BlockMeta](#BlockMeta)
  * [func NewBlockMeta(block *Block, blockParts *PartSet) *BlockMeta](#NewBlockMeta)
* [type CanonicalJSONBlockID](#CanonicalJSONBlockID)
  * [func CanonicalBlockID(blockID BlockID) CanonicalJSONBlockID](#CanonicalBlockID)
* [type CanonicalJSONOnceProposal](#CanonicalJSONOnceProposal)
* [type CanonicalJSONOnceVote](#CanonicalJSONOnceVote)
* [type CanonicalJSONPartSetHeader](#CanonicalJSONPartSetHeader)
  * [func CanonicalPartSetHeader(psh PartSetHeader) CanonicalJSONPartSetHeader](#CanonicalPartSetHeader)
* [type CanonicalJSONProposal](#CanonicalJSONProposal)
  * [func CanonicalProposal(proposal *Proposal) CanonicalJSONProposal](#CanonicalProposal)
* [type CanonicalJSONVote](#CanonicalJSONVote)
  * [func CanonicalVote(vote *Vote) CanonicalJSONVote](#CanonicalVote)
* [type Commit](#Commit)
  * [func (commit *Commit) BitArray() *BitArray](#Commit.BitArray)
  * [func (commit *Commit) FirstPrecommit() *Vote](#Commit.FirstPrecommit)
  * [func (commit *Commit) GetByIndex(index int) *Vote](#Commit.GetByIndex)
  * [func (commit *Commit) Hash() []byte](#Commit.Hash)
  * [func (commit *Commit) Height() int](#Commit.Height)
  * [func (commit *Commit) IsCommit() bool](#Commit.IsCommit)
  * [func (commit *Commit) Round() int](#Commit.Round)
  * [func (commit *Commit) Size() int](#Commit.Size)
  * [func (commit *Commit) StringIndented(indent string) string](#Commit.StringIndented)
  * [func (commit *Commit) Type() byte](#Commit.Type)
  * [func (commit *Commit) ValidateBasic() error](#Commit.ValidateBasic)
* [type Data](#Data)
  * [func (data *Data) Hash() []byte](#Data.Hash)
  * [func (data *Data) StringIndented(indent string) string](#Data.StringIndented)
* [type DefaultSigner](#DefaultSigner)
  * [func NewDefaultSigner(priv crypto.PrivKey) *DefaultSigner](#NewDefaultSigner)
  * [func (ds *DefaultSigner) Sign(msg []byte) crypto.Signature](#DefaultSigner.Sign)
* [type ErrVoteConflictingVotes](#ErrVoteConflictingVotes)
  * [func (err *ErrVoteConflictingVotes) Error() string](#ErrVoteConflictingVotes.Error)
* [type EventCache](#EventCache)
  * [func NewEventCache(evsw EventSwitch) EventCache](#NewEventCache)
* [type EventDataNewBlock](#EventDataNewBlock)
  * [func (_ EventDataNewBlock) AssertIsTMEventData()](#EventDataNewBlock.AssertIsTMEventData)
* [type EventDataNewBlockHeader](#EventDataNewBlockHeader)
  * [func (_ EventDataNewBlockHeader) AssertIsTMEventData()](#EventDataNewBlockHeader.AssertIsTMEventData)
* [type EventDataRoundState](#EventDataRoundState)
  * [func (_ EventDataRoundState) AssertIsTMEventData()](#EventDataRoundState.AssertIsTMEventData)
* [type EventDataTx](#EventDataTx)
  * [func (_ EventDataTx) AssertIsTMEventData()](#EventDataTx.AssertIsTMEventData)
* [type EventDataVote](#EventDataVote)
  * [func (_ EventDataVote) AssertIsTMEventData()](#EventDataVote.AssertIsTMEventData)
* [type EventSwitch](#EventSwitch)
  * [func NewEventSwitch() EventSwitch](#NewEventSwitch)
* [type Eventable](#Eventable)
* [type Fireable](#Fireable)
* [type GenesisDoc](#GenesisDoc)
  * [func GenesisDocFromJSON(jsonBlob []byte) (genDoc *GenesisDoc, err error)](#GenesisDocFromJSON)
  * [func (genDoc *GenesisDoc) SaveAs(file string) error](#GenesisDoc.SaveAs)
* [type GenesisValidator](#GenesisValidator)
* [type Header](#Header)
  * [func (h *Header) Hash() []byte](#Header.Hash)
  * [func (h *Header) StringIndented(indent string) string](#Header.StringIndented)
* [type Part](#Part)
  * [func (part *Part) Hash() []byte](#Part.Hash)
  * [func (part *Part) String() string](#Part.String)
  * [func (part *Part) StringIndented(indent string) string](#Part.StringIndented)
* [type PartSet](#PartSet)
  * [func NewPartSetFromData(data []byte, partSize int) *PartSet](#NewPartSetFromData)
  * [func NewPartSetFromHeader(header PartSetHeader) *PartSet](#NewPartSetFromHeader)
  * [func (ps *PartSet) AddPart(part *Part, verify bool) (bool, error)](#PartSet.AddPart)
  * [func (ps *PartSet) BitArray() *BitArray](#PartSet.BitArray)
  * [func (ps *PartSet) Count() int](#PartSet.Count)
  * [func (ps *PartSet) GetPart(index int) *Part](#PartSet.GetPart)
  * [func (ps *PartSet) GetReader() io.Reader](#PartSet.GetReader)
  * [func (ps *PartSet) HasHeader(header PartSetHeader) bool](#PartSet.HasHeader)
  * [func (ps *PartSet) Hash() []byte](#PartSet.Hash)
  * [func (ps *PartSet) HashesTo(hash []byte) bool](#PartSet.HashesTo)
  * [func (ps *PartSet) Header() PartSetHeader](#PartSet.Header)
  * [func (ps *PartSet) IsComplete() bool](#PartSet.IsComplete)
  * [func (ps *PartSet) StringShort() string](#PartSet.StringShort)
  * [func (ps *PartSet) Total() int](#PartSet.Total)
* [type PartSetHeader](#PartSetHeader)
  * [func (psh PartSetHeader) Equals(other PartSetHeader) bool](#PartSetHeader.Equals)
  * [func (psh PartSetHeader) IsZero() bool](#PartSetHeader.IsZero)
  * [func (psh PartSetHeader) String() string](#PartSetHeader.String)
  * [func (psh PartSetHeader) WriteSignBytes(w io.Writer, n *int, err *error)](#PartSetHeader.WriteSignBytes)
* [type PartSetReader](#PartSetReader)
  * [func NewPartSetReader(parts []*Part) *PartSetReader](#NewPartSetReader)
  * [func (psr *PartSetReader) Read(p []byte) (n int, err error)](#PartSetReader.Read)
* [type PrivValidator](#PrivValidator)
  * [func GenPrivValidator() *PrivValidator](#GenPrivValidator)
  * [func LoadOrGenPrivValidator(filePath string) *PrivValidator](#LoadOrGenPrivValidator)
  * [func LoadPrivValidator(filePath string) *PrivValidator](#LoadPrivValidator)
  * [func (privVal *PrivValidator) GetAddress() []byte](#PrivValidator.GetAddress)
  * [func (privVal *PrivValidator) Reset()](#PrivValidator.Reset)
  * [func (privVal *PrivValidator) Save()](#PrivValidator.Save)
  * [func (privVal *PrivValidator) SetFile(filePath string)](#PrivValidator.SetFile)
  * [func (privVal *PrivValidator) SetSigner(s Signer)](#PrivValidator.SetSigner)
  * [func (privVal *PrivValidator) SignProposal(chainID string, proposal *Proposal) error](#PrivValidator.SignProposal)
  * [func (privVal *PrivValidator) SignVote(chainID string, vote *Vote) error](#PrivValidator.SignVote)
  * [func (privVal *PrivValidator) String() string](#PrivValidator.String)
* [type PrivValidatorsByAddress](#PrivValidatorsByAddress)
  * [func (pvs PrivValidatorsByAddress) Len() int](#PrivValidatorsByAddress.Len)
  * [func (pvs PrivValidatorsByAddress) Less(i, j int) bool](#PrivValidatorsByAddress.Less)
  * [func (pvs PrivValidatorsByAddress) Swap(i, j int)](#PrivValidatorsByAddress.Swap)
* [type Proposal](#Proposal)
  * [func NewProposal(height int, round int, blockPartsHeader PartSetHeader, polRound int, polBlockID BlockID) *Proposal](#NewProposal)
  * [func (p *Proposal) String() string](#Proposal.String)
  * [func (p *Proposal) WriteSignBytes(chainID string, w io.Writer, n *int, err *error)](#Proposal.WriteSignBytes)
* [type Signable](#Signable)
* [type Signer](#Signer)
* [type TMEventData](#TMEventData)
* [type Tx](#Tx)
  * [func (tx Tx) Hash() []byte](#Tx.Hash)
* [type Txs](#Txs)
  * [func (txs Txs) Hash() []byte](#Txs.Hash)
* [type Validator](#Validator)
  * [func NewValidator(pubKey crypto.PubKey, votingPower int64) *Validator](#NewValidator)
  * [func RandValidator(randPower bool, minPower int64) (*Validator, *PrivValidator)](#RandValidator)
  * [func (v *Validator) CompareAccum(other *Validator) *Validator](#Validator.CompareAccum)
  * [func (v *Validator) Copy() *Validator](#Validator.Copy)
  * [func (v *Validator) Hash() []byte](#Validator.Hash)
  * [func (v *Validator) String() string](#Validator.String)
* [type ValidatorSet](#ValidatorSet)
  * [func NewValidatorSet(vals []*Validator) *ValidatorSet](#NewValidatorSet)
  * [func RandValidatorSet(numValidators int, votingPower int64) (*ValidatorSet, []*PrivValidator)](#RandValidatorSet)
  * [func (valSet *ValidatorSet) Add(val *Validator) (added bool)](#ValidatorSet.Add)
  * [func (valSet *ValidatorSet) Copy() *ValidatorSet](#ValidatorSet.Copy)
  * [func (valSet *ValidatorSet) GetByAddress(address []byte) (index int, val *Validator)](#ValidatorSet.GetByAddress)
  * [func (valSet *ValidatorSet) GetByIndex(index int) (address []byte, val *Validator)](#ValidatorSet.GetByIndex)
  * [func (valSet *ValidatorSet) HasAddress(address []byte) bool](#ValidatorSet.HasAddress)
  * [func (valSet *ValidatorSet) Hash() []byte](#ValidatorSet.Hash)
  * [func (valSet *ValidatorSet) IncrementAccum(times int)](#ValidatorSet.IncrementAccum)
  * [func (valSet *ValidatorSet) Iterate(fn func(index int, val *Validator) bool)](#ValidatorSet.Iterate)
  * [func (valSet *ValidatorSet) Proposer() (proposer *Validator)](#ValidatorSet.Proposer)
  * [func (valSet *ValidatorSet) Remove(address []byte) (val *Validator, removed bool)](#ValidatorSet.Remove)
  * [func (valSet *ValidatorSet) Size() int](#ValidatorSet.Size)
  * [func (valSet *ValidatorSet) String() string](#ValidatorSet.String)
  * [func (valSet *ValidatorSet) StringIndented(indent string) string](#ValidatorSet.StringIndented)
  * [func (valSet *ValidatorSet) TotalVotingPower() int64](#ValidatorSet.TotalVotingPower)
  * [func (valSet *ValidatorSet) Update(val *Validator) (updated bool)](#ValidatorSet.Update)
  * [func (valSet *ValidatorSet) VerifyCommit(chainID string, blockID BlockID, height int, commit *Commit) error](#ValidatorSet.VerifyCommit)
  * [func (valSet *ValidatorSet) VerifyCommitAny(chainID string, blockID BlockID, height int, commit *Commit) error](#ValidatorSet.VerifyCommitAny)
* [type ValidatorsByAddress](#ValidatorsByAddress)
  * [func (vs ValidatorsByAddress) Len() int](#ValidatorsByAddress.Len)
  * [func (vs ValidatorsByAddress) Less(i, j int) bool](#ValidatorsByAddress.Less)
  * [func (vs ValidatorsByAddress) Swap(i, j int)](#ValidatorsByAddress.Swap)
* [type Vote](#Vote)
  * [func (vote *Vote) Copy() *Vote](#Vote.Copy)
  * [func (vote *Vote) String() string](#Vote.String)
  * [func (vote *Vote) WriteSignBytes(chainID string, w io.Writer, n *int, err *error)](#Vote.WriteSignBytes)
* [type VoteSet](#VoteSet)
  * [func NewVoteSet(chainID string, height int, round int, type_ byte, valSet *ValidatorSet) *VoteSet](#NewVoteSet)
  * [func (voteSet *VoteSet) AddVote(vote *Vote) (added bool, err error)](#VoteSet.AddVote)
  * [func (voteSet *VoteSet) BitArray() *BitArray](#VoteSet.BitArray)
  * [func (voteSet *VoteSet) BitArrayByBlockID(blockID BlockID) *BitArray](#VoteSet.BitArrayByBlockID)
  * [func (voteSet *VoteSet) ChainID() string](#VoteSet.ChainID)
  * [func (voteSet *VoteSet) GetByAddress(address []byte) *Vote](#VoteSet.GetByAddress)
  * [func (voteSet *VoteSet) GetByIndex(valIndex int) *Vote](#VoteSet.GetByIndex)
  * [func (voteSet *VoteSet) HasAll() bool](#VoteSet.HasAll)
  * [func (voteSet *VoteSet) HasTwoThirdsAny() bool](#VoteSet.HasTwoThirdsAny)
  * [func (voteSet *VoteSet) HasTwoThirdsMajority() bool](#VoteSet.HasTwoThirdsMajority)
  * [func (voteSet *VoteSet) Height() int](#VoteSet.Height)
  * [func (voteSet *VoteSet) IsCommit() bool](#VoteSet.IsCommit)
  * [func (voteSet *VoteSet) MakeCommit() *Commit](#VoteSet.MakeCommit)
  * [func (voteSet *VoteSet) Round() int](#VoteSet.Round)
  * [func (voteSet *VoteSet) SetPeerMaj23(peerID string, blockID BlockID)](#VoteSet.SetPeerMaj23)
  * [func (voteSet *VoteSet) Size() int](#VoteSet.Size)
  * [func (voteSet *VoteSet) String() string](#VoteSet.String)
  * [func (voteSet *VoteSet) StringIndented(indent string) string](#VoteSet.StringIndented)
  * [func (voteSet *VoteSet) StringShort() string](#VoteSet.StringShort)
  * [func (voteSet *VoteSet) TwoThirdsMajority() (blockID BlockID, ok bool)](#VoteSet.TwoThirdsMajority)
  * [func (voteSet *VoteSet) Type() byte](#VoteSet.Type)
* [type VoteSetReader](#VoteSetReader)


#### <a name="pkg-files">Package files</a>
[block.go](/src/github.com/tendermint/tendermint/types/block.go) [block_meta.go](/src/github.com/tendermint/tendermint/types/block_meta.go) [canonical_json.go](/src/github.com/tendermint/tendermint/types/canonical_json.go) [events.go](/src/github.com/tendermint/tendermint/types/events.go) [genesis.go](/src/github.com/tendermint/tendermint/types/genesis.go) [keys.go](/src/github.com/tendermint/tendermint/types/keys.go) [log.go](/src/github.com/tendermint/tendermint/types/log.go) [part_set.go](/src/github.com/tendermint/tendermint/types/part_set.go) [priv_validator.go](/src/github.com/tendermint/tendermint/types/priv_validator.go) [proposal.go](/src/github.com/tendermint/tendermint/types/proposal.go) [protobuf.go](/src/github.com/tendermint/tendermint/types/protobuf.go) [signable.go](/src/github.com/tendermint/tendermint/types/signable.go) [tx.go](/src/github.com/tendermint/tendermint/types/tx.go) [validator.go](/src/github.com/tendermint/tendermint/types/validator.go) [validator_set.go](/src/github.com/tendermint/tendermint/types/validator_set.go) [vote.go](/src/github.com/tendermint/tendermint/types/vote.go) [vote_set.go](/src/github.com/tendermint/tendermint/types/vote_set.go) 


## <a name="pkg-constants">Constants</a>
``` go
const (
    EventDataTypeNewBlock       = byte(0x01)
    EventDataTypeFork           = byte(0x02)
    EventDataTypeTx             = byte(0x03)
    EventDataTypeNewBlockHeader = byte(0x04)

    EventDataTypeRoundState = byte(0x11)
    EventDataTypeVote       = byte(0x12)
)
```
``` go
const (
    VoteTypePrevote   = byte(0x01)
    VoteTypePrecommit = byte(0x02)
)
```
Types of votes
TODO Make a new type "VoteType"

``` go
const MaxBlockSize = 22020096 // 21MB TODO make it configurable

```

## <a name="pkg-variables">Variables</a>
``` go
var (
    PeerStateKey     = "ConsensusReactor.peerState"
    PeerMempoolChKey = "MempoolReactor.peerMempoolCh"
)
```
``` go
var (
    ErrPartSetUnexpectedIndex = errors.New("Error part set unexpected index")
    ErrPartSetInvalidProof    = errors.New("Error part set invalid proof")
)
```
``` go
var (
    ErrInvalidBlockPartSignature = errors.New("Error invalid block part signature")
    ErrInvalidBlockPartHash      = errors.New("Error invalid block part hash")
)
```
``` go
var (
    ErrVoteUnexpectedStep          = errors.New("Unexpected step")
    ErrVoteInvalidValidatorIndex   = errors.New("Invalid round vote validator index")
    ErrVoteInvalidValidatorAddress = errors.New("Invalid round vote validator address")
    ErrVoteInvalidSignature        = errors.New("Invalid round vote signature")
    ErrVoteInvalidBlockHash        = errors.New("Invalid block hash")
)
```
``` go
var GenDocKey = []byte("GenDocKey")
```
``` go
var TM2PB = tm2pb{}
```
Convert tendermint types to protobuf types

``` go
var ValidatorCodec = validatorCodec{}
```


## <a name="AddListenerForEvent">func</a> [AddListenerForEvent](/src/target/events.go?s=4038:4125#L128)
``` go
func AddListenerForEvent(evsw EventSwitch, id, event string, cb func(data TMEventData))
```


## <a name="EventStringBond">func</a> [EventStringBond](/src/target/events.go?s=279:308#L4)
``` go
func EventStringBond() string
```
Reserved



## <a name="EventStringCompleteProposal">func</a> [EventStringCompleteProposal](/src/target/events.go?s=946:987#L16)
``` go
func EventStringCompleteProposal() string
```


## <a name="EventStringDupeout">func</a> [EventStringDupeout](/src/target/events.go?s=436:468#L7)
``` go
func EventStringDupeout() string
```


## <a name="EventStringFork">func</a> [EventStringFork](/src/target/events.go?s=490:519#L8)
``` go
func EventStringFork() string
```


## <a name="EventStringLock">func</a> [EventStringLock](/src/target/events.go?s=1141:1170#L19)
``` go
func EventStringLock() string
```


## <a name="EventStringNewBlock">func</a> [EventStringNewBlock](/src/target/events.go?s=610:643#L11)
``` go
func EventStringNewBlock() string
```


## <a name="EventStringNewBlockHeader">func</a> [EventStringNewBlockHeader](/src/target/events.go?s=674:713#L12)
``` go
func EventStringNewBlockHeader() string
```


## <a name="EventStringNewRound">func</a> [EventStringNewRound](/src/target/events.go?s=744:777#L13)
``` go
func EventStringNewRound() string
```


## <a name="EventStringNewRoundStep">func</a> [EventStringNewRoundStep](/src/target/events.go?s=808:845#L14)
``` go
func EventStringNewRoundStep() string
```


## <a name="EventStringPolka">func</a> [EventStringPolka](/src/target/events.go?s=1018:1048#L17)
``` go
func EventStringPolka() string
```


## <a name="EventStringRebond">func</a> [EventStringRebond](/src/target/events.go?s=383:414#L6)
``` go
func EventStringRebond() string
```


## <a name="EventStringRelock">func</a> [EventStringRelock](/src/target/events.go?s=1201:1232#L20)
``` go
func EventStringRelock() string
```


## <a name="EventStringTimeoutPropose">func</a> [EventStringTimeoutPropose](/src/target/events.go?s=876:915#L15)
``` go
func EventStringTimeoutPropose() string
```


## <a name="EventStringTimeoutWait">func</a> [EventStringTimeoutWait](/src/target/events.go?s=1263:1299#L21)
``` go
func EventStringTimeoutWait() string
```


## <a name="EventStringTx">func</a> [EventStringTx](/src/target/events.go?s=541:573#L9)
``` go
func EventStringTx(tx Tx) string
```


## <a name="EventStringUnbond">func</a> [EventStringUnbond](/src/target/events.go?s=330:361#L5)
``` go
func EventStringUnbond() string
```


## <a name="EventStringUnlock">func</a> [EventStringUnlock](/src/target/events.go?s=1079:1110#L18)
``` go
func EventStringUnlock() string
```


## <a name="EventStringVote">func</a> [EventStringVote](/src/target/events.go?s=1330:1359#L22)
``` go
func EventStringVote() string
```


## <a name="FireEventCompleteProposal">func</a> [FireEventCompleteProposal](/src/target/events.go?s=5333:5413#L171)
``` go
func FireEventCompleteProposal(fireable events.Fireable, rs EventDataRoundState)
```


## <a name="FireEventLock">func</a> [FireEventLock](/src/target/events.go?s=5839:5907#L187)
``` go
func FireEventLock(fireable events.Fireable, rs EventDataRoundState)
```


## <a name="FireEventNewBlock">func</a> [FireEventNewBlock](/src/target/events.go?s=4262:4335#L137)
``` go
func FireEventNewBlock(fireable events.Fireable, block EventDataNewBlock)
```


## <a name="FireEventNewBlockHeader">func</a> [FireEventNewBlockHeader](/src/target/events.go?s=4392:4478#L141)
``` go
func FireEventNewBlockHeader(fireable events.Fireable, header EventDataNewBlockHeader)
```


## <a name="FireEventNewRound">func</a> [FireEventNewRound](/src/target/events.go?s=5207:5279#L167)
``` go
func FireEventNewRound(fireable events.Fireable, rs EventDataRoundState)
```


## <a name="FireEventNewRoundStep">func</a> [FireEventNewRoundStep](/src/target/events.go?s=4803:4879#L155)
``` go
func FireEventNewRoundStep(fireable events.Fireable, rs EventDataRoundState)
```


## <a name="FireEventPolka">func</a> [FireEventPolka](/src/target/events.go?s=5475:5544#L175)
``` go
func FireEventPolka(fireable events.Fireable, rs EventDataRoundState)
```


## <a name="FireEventRelock">func</a> [FireEventRelock](/src/target/events.go?s=5717:5787#L183)
``` go
func FireEventRelock(fireable events.Fireable, rs EventDataRoundState)
```


## <a name="FireEventTimeoutPropose">func</a> [FireEventTimeoutPropose](/src/target/events.go?s=4937:5015#L159)
``` go
func FireEventTimeoutPropose(fireable events.Fireable, rs EventDataRoundState)
```


## <a name="FireEventTimeoutWait">func</a> [FireEventTimeoutWait](/src/target/events.go?s=5075:5150#L163)
``` go
func FireEventTimeoutWait(fireable events.Fireable, rs EventDataRoundState)
```


## <a name="FireEventTx">func</a> [FireEventTx](/src/target/events.go?s=4658:4716#L149)
``` go
func FireEventTx(fireable events.Fireable, tx EventDataTx)
```


## <a name="FireEventUnlock">func</a> [FireEventUnlock](/src/target/events.go?s=5595:5665#L179)
``` go
func FireEventUnlock(fireable events.Fireable, rs EventDataRoundState)
```


## <a name="FireEventVote">func</a> [FireEventVote](/src/target/events.go?s=4542:4606#L145)
``` go
func FireEventVote(fireable events.Fireable, vote EventDataVote)
```


## <a name="HashSignBytes">func</a> [HashSignBytes](/src/target/signable.go?s=699:752#L18)
``` go
func HashSignBytes(chainID string, o Signable) []byte
```
HashSignBytes is a convenience method for getting the hash of the bytes of a signable



## <a name="IsVoteTypeValid">func</a> [IsVoteTypeValid](/src/target/vote.go?s=820:857#L27)
``` go
func IsVoteTypeValid(type_ byte) bool
```


## <a name="SignBytes">func</a> [SignBytes](/src/target/signable.go?s=399:448#L8)
``` go
func SignBytes(chainID string, o Signable) []byte
```
SignBytes is a convenience method for getting the bytes to sign of a Signable.




## <a name="Block">type</a> [Block](/src/target/block.go?s=249:365#L8)
``` go
type Block struct {
    *Header    `json:"header"`
    *Data      `json:"data"`
    LastCommit *Commit `json:"last_commit"`
}
```






### <a name="MakeBlock">func</a> [MakeBlock](/src/target/block.go?s=384:532#L15)
``` go
func MakeBlock(height int, chainID string, txs []Tx, commit *Commit,
    prevBlockID BlockID, valHash, appHash []byte, partSize int) (*Block, *PartSet)
```
TODO: version





### <a name="Block.FillHeader">func</a> (\*Block) [FillHeader](/src/target/block.go?s=2551:2579#L76)
``` go
func (b *Block) FillHeader()
```



### <a name="Block.Hash">func</a> (\*Block) [Hash](/src/target/block.go?s=2816:2845#L87)
``` go
func (b *Block) Hash() []byte
```
Computes and returns the block hash.
If the block is incomplete, block hash is nil for safety.




### <a name="Block.HashesTo">func</a> (\*Block) [HashesTo](/src/target/block.go?s=3203:3245#L103)
``` go
func (b *Block) HashesTo(hash []byte) bool
```
Convenience.
A nil block never hashes to anything.
Nothing hashes to a nil hash.




### <a name="Block.MakePartSet">func</a> (\*Block) [MakePartSet](/src/target/block.go?s=2999:3049#L96)
``` go
func (b *Block) MakePartSet(partSize int) *PartSet
```



### <a name="Block.String">func</a> (\*Block) [String](/src/target/block.go?s=3359:3390#L113)
``` go
func (b *Block) String() string
```



### <a name="Block.StringIndented">func</a> (\*Block) [StringIndented](/src/target/block.go?s=3425:3477#L117)
``` go
func (b *Block) StringIndented(indent string) string
```



### <a name="Block.StringShort">func</a> (\*Block) [StringShort](/src/target/block.go?s=3746:3782#L132)
``` go
func (b *Block) StringShort() string
```



### <a name="Block.ValidateBasic">func</a> (\*Block) [ValidateBasic](/src/target/block.go?s=1010:1145#L37)
``` go
func (b *Block) ValidateBasic(chainID string, lastBlockHeight int, lastBlockID BlockID,
    lastBlockTime time.Time, appHash []byte) error
```
Basic validation that doesn't involve state data.




## <a name="BlockID">type</a> [BlockID](/src/target/block.go?s=10033:10139#L380)
``` go
type BlockID struct {
    Hash        []byte        `json:"hash"`
    PartsHeader PartSetHeader `json:"parts"`
}
```









### <a name="BlockID.Equals">func</a> (BlockID) [Equals](/src/target/block.go?s=10246:10295#L389)
``` go
func (blockID BlockID) Equals(other BlockID) bool
```



### <a name="BlockID.IsZero">func</a> (BlockID) [IsZero](/src/target/block.go?s=10141:10177#L385)
``` go
func (blockID BlockID) IsZero() bool
```



### <a name="BlockID.Key">func</a> (BlockID) [Key](/src/target/block.go?s=10398:10433#L394)
``` go
func (blockID BlockID) Key() string
```



### <a name="BlockID.String">func</a> (BlockID) [String](/src/target/block.go?s=10726:10764#L407)
``` go
func (blockID BlockID) String() string
```



### <a name="BlockID.WriteSignBytes">func</a> (BlockID) [WriteSignBytes](/src/target/block.go?s=10516:10586#L398)
``` go
func (blockID BlockID) WriteSignBytes(w io.Writer, n *int, err *error)
```



## <a name="BlockMeta">type</a> [BlockMeta](/src/target/block_meta.go?s=15:262#L1)
``` go
type BlockMeta struct {
    Hash        []byte        `json:"hash"`         // The block hash
    Header      *Header       `json:"header"`       // The block's Header
    PartsHeader PartSetHeader `json:"parts_header"` // The PartSetHeader, for transfer
}
```






### <a name="NewBlockMeta">func</a> [NewBlockMeta](/src/target/block_meta.go?s=264:327#L1)
``` go
func NewBlockMeta(block *Block, blockParts *PartSet) *BlockMeta
```




## <a name="CanonicalJSONBlockID">type</a> [CanonicalJSONBlockID](/src/target/canonical_json.go?s=98:263#L1)
``` go
type CanonicalJSONBlockID struct {
    Hash        []byte                     `json:"hash,omitempty"`
    PartsHeader CanonicalJSONPartSetHeader `json:"parts,omitempty"`
}
```






### <a name="CanonicalBlockID">func</a> [CanonicalBlockID](/src/target/canonical_json.go?s=1405:1464#L36)
``` go
func CanonicalBlockID(blockID BlockID) CanonicalJSONBlockID
```




## <a name="CanonicalJSONOnceProposal">type</a> [CanonicalJSONOnceProposal](/src/target/canonical_json.go?s=1070:1211#L23)
``` go
type CanonicalJSONOnceProposal struct {
    ChainID  string                `json:"chain_id"`
    Proposal CanonicalJSONProposal `json:"proposal"`
}
```









## <a name="CanonicalJSONOnceVote">type</a> [CanonicalJSONOnceVote](/src/target/canonical_json.go?s=1213:1336#L28)
``` go
type CanonicalJSONOnceVote struct {
    ChainID string            `json:"chain_id"`
    Vote    CanonicalJSONVote `json:"vote"`
}
```









## <a name="CanonicalJSONPartSetHeader">type</a> [CanonicalJSONPartSetHeader](/src/target/canonical_json.go?s=265:364#L1)
``` go
type CanonicalJSONPartSetHeader struct {
    Hash  []byte `json:"hash"`
    Total int    `json:"total"`
}
```






### <a name="CanonicalPartSetHeader">func</a> [CanonicalPartSetHeader](/src/target/canonical_json.go?s=1592:1665#L43)
``` go
func CanonicalPartSetHeader(psh PartSetHeader) CanonicalJSONPartSetHeader
```




## <a name="CanonicalJSONProposal">type</a> [CanonicalJSONProposal](/src/target/canonical_json.go?s=366:728#L5)
``` go
type CanonicalJSONProposal struct {
    BlockPartsHeader CanonicalJSONPartSetHeader `json:"block_parts_header"`
    Height           int                        `json:"height"`
    POLBlockID       CanonicalJSONBlockID       `json:"pol_block_id"`
    POLRound         int                        `json:"pol_round"`
    Round            int                        `json:"round"`
}
```






### <a name="CanonicalProposal">func</a> [CanonicalProposal](/src/target/canonical_json.go?s=1735:1799#L50)
``` go
func CanonicalProposal(proposal *Proposal) CanonicalJSONProposal
```




## <a name="CanonicalJSONVote">type</a> [CanonicalJSONVote](/src/target/canonical_json.go?s=730:946#L13)
``` go
type CanonicalJSONVote struct {
    BlockID CanonicalJSONBlockID `json:"block_id"`
    Height  int                  `json:"height"`
    Round   int                  `json:"round"`
    Type    byte                 `json:"type"`
}
```






### <a name="CanonicalVote">func</a> [CanonicalVote](/src/target/canonical_json.go?s=2081:2129#L60)
``` go
func CanonicalVote(vote *Vote) CanonicalJSONVote
```




## <a name="Commit">type</a> [Commit](/src/target/block.go?s=5711:6107#L202)
``` go
type Commit struct {
    // NOTE: The Precommits are in order of address to preserve the bonded ValidatorSet order.
    // Any peer with a block can gossip precommits by index with a peer without recalculating the
    // active ValidatorSet.
    BlockID    BlockID `json:"blockID"`
    Precommits []*Vote `json:"precommits"`
    // contains filtered or unexported fields
}
```
NOTE: Commit is empty for height 1, but never nil.










### <a name="Commit.BitArray">func</a> (\*Commit) [BitArray](/src/target/block.go?s=6845:6887#L256)
``` go
func (commit *Commit) BitArray() *BitArray
```



### <a name="Commit.FirstPrecommit">func</a> (\*Commit) [FirstPrecommit](/src/target/block.go?s=6109:6153#L215)
``` go
func (commit *Commit) FirstPrecommit() *Vote
```



### <a name="Commit.GetByIndex">func</a> (\*Commit) [GetByIndex](/src/target/block.go?s=7106:7155#L266)
``` go
func (commit *Commit) GetByIndex(index int) *Vote
```



### <a name="Commit.Hash">func</a> (\*Commit) [Hash](/src/target/block.go?s=8295:8330#L311)
``` go
func (commit *Commit) Hash() []byte
```



### <a name="Commit.Height">func</a> (\*Commit) [Height](/src/target/block.go?s=6425:6459#L231)
``` go
func (commit *Commit) Height() int
```



### <a name="Commit.IsCommit">func</a> (\*Commit) [IsCommit](/src/target/block.go?s=7194:7231#L270)
``` go
func (commit *Commit) IsCommit() bool
```



### <a name="Commit.Round">func</a> (\*Commit) [Round](/src/target/block.go?s=6552:6585#L238)
``` go
func (commit *Commit) Round() int
```



### <a name="Commit.Size">func</a> (\*Commit) [Size](/src/target/block.go?s=6742:6774#L249)
``` go
func (commit *Commit) Size() int
```



### <a name="Commit.StringIndented">func</a> (\*Commit) [StringIndented](/src/target/block.go?s=8559:8617#L322)
``` go
func (commit *Commit) StringIndented(indent string) string
```



### <a name="Commit.Type">func</a> (\*Commit) [Type](/src/target/block.go?s=6677:6710#L245)
``` go
func (commit *Commit) Type() byte
```



### <a name="Commit.ValidateBasic">func</a> (\*Commit) [ValidateBasic](/src/target/block.go?s=7302:7345#L277)
``` go
func (commit *Commit) ValidateBasic() error
```



## <a name="Data">type</a> [Data](/src/target/block.go?s=9087:9354#L341)
``` go
type Data struct {

    // Txs that will be applied by state @ block.Height+1.
    // NOTE: not all txs here are valid.  We're just agreeing on the order first.
    // This means that block.AppHash does not include these txs.
    Txs Txs `json:"txs"`
    // contains filtered or unexported fields
}
```









### <a name="Data.Hash">func</a> (\*Data) [Hash](/src/target/block.go?s=9356:9387#L352)
``` go
func (data *Data) Hash() []byte
```



### <a name="Data.StringIndented">func</a> (\*Data) [StringIndented](/src/target/block.go?s=9508:9562#L359)
``` go
func (data *Data) StringIndented(indent string) string
```



## <a name="DefaultSigner">type</a> [DefaultSigner](/src/target/priv_validator.go?s=1516:1566#L55)
``` go
type DefaultSigner struct {
    // contains filtered or unexported fields
}
```
Implements Signer







### <a name="NewDefaultSigner">func</a> [NewDefaultSigner](/src/target/priv_validator.go?s=1568:1625#L59)
``` go
func NewDefaultSigner(priv crypto.PrivKey) *DefaultSigner
```




### <a name="DefaultSigner.Sign">func</a> (\*DefaultSigner) [Sign](/src/target/priv_validator.go?s=1687:1745#L64)
``` go
func (ds *DefaultSigner) Sign(msg []byte) crypto.Signature
```
Implements Signer




## <a name="ErrVoteConflictingVotes">type</a> [ErrVoteConflictingVotes](/src/target/vote.go?s=541:606#L11)
``` go
type ErrVoteConflictingVotes struct {
    VoteA *Vote
    VoteB *Vote
}
```









### <a name="ErrVoteConflictingVotes.Error">func</a> (\*ErrVoteConflictingVotes) [Error](/src/target/vote.go?s=608:658#L16)
``` go
func (err *ErrVoteConflictingVotes) Error() string
```



## <a name="EventCache">type</a> [EventCache](/src/target/events.go?s=3613:3661#L108)
``` go
type EventCache interface {
    Fireable
    Flush()
}
```






### <a name="NewEventCache">func</a> [NewEventCache](/src/target/events.go?s=3734:3781#L117)
``` go
func NewEventCache(evsw EventSwitch) EventCache
```




## <a name="EventDataNewBlock">type</a> [EventDataNewBlock](/src/target/events.go?s=2362:2424#L55)
``` go
type EventDataNewBlock struct {
    Block *Block `json:"block"`
}
```









### <a name="EventDataNewBlock.AssertIsTMEventData">func</a> (EventDataNewBlock) [AssertIsTMEventData](/src/target/events.go?s=3093:3141#L87)
``` go
func (_ EventDataNewBlock) AssertIsTMEventData()
```



## <a name="EventDataNewBlockHeader">type</a> [EventDataNewBlockHeader](/src/target/events.go?s=2465:2536#L60)
``` go
type EventDataNewBlockHeader struct {
    Header *Header `json:"header"`
}
```
light weight event for benchmarking










### <a name="EventDataNewBlockHeader.AssertIsTMEventData">func</a> (EventDataNewBlockHeader) [AssertIsTMEventData](/src/target/events.go?s=3151:3205#L88)
``` go
func (_ EventDataNewBlockHeader) AssertIsTMEventData()
```



## <a name="EventDataRoundState">type</a> [EventDataRoundState](/src/target/events.go?s=2848:3048#L74)
``` go
type EventDataRoundState struct {
    Height int    `json:"height"`
    Round  int    `json:"round"`
    Step   string `json:"step"`

    // private, not exposed to websockets
    RoundState interface{} `json:"-"`
}
```
NOTE: This goes into the replay WAL










### <a name="EventDataRoundState.AssertIsTMEventData">func</a> (EventDataRoundState) [AssertIsTMEventData](/src/target/events.go?s=3267:3317#L90)
``` go
func (_ EventDataRoundState) AssertIsTMEventData()
```



## <a name="EventDataTx">type</a> [EventDataTx](/src/target/events.go?s=2566:2807#L65)
``` go
type EventDataTx struct {
    Tx    Tx            `json:"tx"`
    Data  []byte        `json:"data"`
    Log   string        `json:"log"`
    Code  abci.CodeType `json:"code"`
    Error string        `json:"error"` // this is redundant information for now
}
```
All txs fire EventDataTx










### <a name="EventDataTx.AssertIsTMEventData">func</a> (EventDataTx) [AssertIsTMEventData](/src/target/events.go?s=3209:3251#L89)
``` go
func (_ EventDataTx) AssertIsTMEventData()
```



## <a name="EventDataVote">type</a> [EventDataVote](/src/target/events.go?s=3050:3091#L83)
``` go
type EventDataVote struct {
    Vote *Vote
}
```









### <a name="EventDataVote.AssertIsTMEventData">func</a> (EventDataVote) [AssertIsTMEventData](/src/target/events.go?s=3325:3369#L91)
``` go
func (_ EventDataVote) AssertIsTMEventData()
```



## <a name="EventSwitch">type</a> [EventSwitch](/src/target/events.go?s=3561:3611#L104)
``` go
type EventSwitch interface {
    events.EventSwitch
}
```






### <a name="NewEventSwitch">func</a> [NewEventSwitch](/src/target/events.go?s=3663:3696#L113)
``` go
func NewEventSwitch() EventSwitch
```




## <a name="Eventable">type</a> [Eventable](/src/target/events.go?s=3502:3559#L100)
``` go
type Eventable interface {
    SetEventSwitch(EventSwitch)
}
```









## <a name="Fireable">type</a> [Fireable](/src/target/events.go?s=3456:3500#L96)
``` go
type Fireable interface {
    events.Fireable
}
```









## <a name="GenesisDoc">type</a> [GenesisDoc](/src/target/genesis.go?s=525:757#L15)
``` go
type GenesisDoc struct {
    GenesisTime time.Time          `json:"genesis_time"`
    ChainID     string             `json:"chain_id"`
    Validators  []GenesisValidator `json:"validators"`
    AppHash     []byte             `json:"app_hash"`
}
```






### <a name="GenesisDocFromJSON">func</a> [GenesisDocFromJSON](/src/target/genesis.go?s=1055:1127#L31)
``` go
func GenesisDocFromJSON(jsonBlob []byte) (genDoc *GenesisDoc, err error)
```




### <a name="GenesisDoc.SaveAs">func</a> (\*GenesisDoc) [SaveAs](/src/target/genesis.go?s=814:865#L23)
``` go
func (genDoc *GenesisDoc) SaveAs(file string) error
```
Utility method for saving GenensisDoc as JSON file.




## <a name="GenesisValidator">type</a> [GenesisValidator](/src/target/genesis.go?s=378:523#L9)
``` go
type GenesisValidator struct {
    PubKey crypto.PubKey `json:"pub_key"`
    Amount int64         `json:"amount"`
    Name   string        `json:"name"`
}
```









## <a name="Header">type</a> [Header](/src/target/block.go?s=3961:4582#L142)
``` go
type Header struct {
    ChainID        string    `json:"chain_id"`
    Height         int       `json:"height"`
    Time           time.Time `json:"time"`
    NumTxs         int       `json:"num_txs"` // XXX: Can we get rid of this?
    LastBlockID    BlockID   `json:"last_block_id"`
    LastCommitHash []byte    `json:"last_commit_hash"` // commit from validators from the last block
    DataHash       []byte    `json:"data_hash"`        // transactions
    ValidatorsHash []byte    `json:"validators_hash"`  // validators for the current block
    AppHash        []byte    `json:"app_hash"`         // state after txs from the previous block
}
```









### <a name="Header.Hash">func</a> (\*Header) [Hash](/src/target/block.go?s=4637:4667#L155)
``` go
func (h *Header) Hash() []byte
```
NOTE: hash is nil if required fields are missing.




### <a name="Header.StringIndented">func</a> (\*Header) [StringIndented](/src/target/block.go?s=5049:5102#L172)
``` go
func (h *Header) StringIndented(indent string) string
```



## <a name="Part">type</a> [Part](/src/target/part_set.go?s=363:530#L12)
``` go
type Part struct {
    Index int                `json:"index"`
    Bytes []byte             `json:"bytes"`
    Proof merkle.SimpleProof `json:"proof"`
    // contains filtered or unexported fields
}
```









### <a name="Part.Hash">func</a> (\*Part) [Hash](/src/target/part_set.go?s=532:563#L21)
``` go
func (part *Part) Hash() []byte
```



### <a name="Part.String">func</a> (\*Part) [String](/src/target/part_set.go?s=743:776#L32)
``` go
func (part *Part) String() string
```



### <a name="Part.StringIndented">func</a> (\*Part) [StringIndented](/src/target/part_set.go?s=814:868#L36)
``` go
func (part *Part) StringIndented(indent string) string
```



## <a name="PartSet">type</a> [PartSet](/src/target/part_set.go?s=1663:1805#L72)
``` go
type PartSet struct {
    // contains filtered or unexported fields
}
```






### <a name="NewPartSetFromData">func</a> [NewPartSetFromData](/src/target/part_set.go?s=1944:2003#L84)
``` go
func NewPartSetFromData(data []byte, partSize int) *PartSet
```
Returns an immutable, full PartSet from the data bytes.
The data bytes are split into "partSize" chunks, and merkle tree computed.


### <a name="NewPartSetFromHeader">func</a> [NewPartSetFromHeader](/src/target/part_set.go?s=2747:2803#L114)
``` go
func NewPartSetFromHeader(header PartSetHeader) *PartSet
```
Returns an empty PartSet ready to be populated.





### <a name="PartSet.AddPart">func</a> (\*PartSet) [AddPart](/src/target/part_set.go?s=3797:3862#L177)
``` go
func (ps *PartSet) AddPart(part *Part, verify bool) (bool, error)
```



### <a name="PartSet.BitArray">func</a> (\*PartSet) [BitArray](/src/target/part_set.go?s=3310:3349#L143)
``` go
func (ps *PartSet) BitArray() *BitArray
```



### <a name="PartSet.Count">func</a> (\*PartSet) [Count](/src/target/part_set.go?s=3631:3661#L163)
``` go
func (ps *PartSet) Count() int
```



### <a name="PartSet.GetPart">func</a> (\*PartSet) [GetPart](/src/target/part_set.go?s=4376:4419#L205)
``` go
func (ps *PartSet) GetPart(index int) *Part
```



### <a name="PartSet.GetReader">func</a> (\*PartSet) [GetReader](/src/target/part_set.go?s=4558:4598#L215)
``` go
func (ps *PartSet) GetReader() io.Reader
```



### <a name="PartSet.HasHeader">func</a> (\*PartSet) [HasHeader](/src/target/part_set.go?s=3169:3224#L135)
``` go
func (ps *PartSet) HasHeader(header PartSetHeader) bool
```



### <a name="PartSet.Hash">func</a> (\*PartSet) [Hash](/src/target/part_set.go?s=3425:3457#L149)
``` go
func (ps *PartSet) Hash() []byte
```



### <a name="PartSet.HashesTo">func</a> (\*PartSet) [HashesTo](/src/target/part_set.go?s=3511:3556#L156)
``` go
func (ps *PartSet) HashesTo(hash []byte) bool
```



### <a name="PartSet.Header">func</a> (\*PartSet) [Header](/src/target/part_set.go?s=3001:3042#L124)
``` go
func (ps *PartSet) Header() PartSetHeader
```



### <a name="PartSet.IsComplete">func</a> (\*PartSet) [IsComplete](/src/target/part_set.go?s=4487:4523#L211)
``` go
func (ps *PartSet) IsComplete() bool
```



### <a name="PartSet.StringShort">func</a> (\*PartSet) [StringShort](/src/target/part_set.go?s=5416:5455#L257)
``` go
func (ps *PartSet) StringShort() string
```



### <a name="PartSet.Total">func</a> (\*PartSet) [Total](/src/target/part_set.go?s=3714:3744#L170)
``` go
func (ps *PartSet) Total() int
```



## <a name="PartSetHeader">type</a> [PartSetHeader](/src/target/part_set.go?s=1091:1177#L49)
``` go
type PartSetHeader struct {
    Total int    `json:"total"`
    Hash  []byte `json:"hash"`
}
```









### <a name="PartSetHeader.Equals">func</a> (PartSetHeader) [Equals](/src/target/part_set.go?s=1355:1412#L62)
``` go
func (psh PartSetHeader) Equals(other PartSetHeader) bool
```



### <a name="PartSetHeader.IsZero">func</a> (PartSetHeader) [IsZero](/src/target/part_set.go?s=1288:1326#L58)
``` go
func (psh PartSetHeader) IsZero() bool
```



### <a name="PartSetHeader.String">func</a> (PartSetHeader) [String](/src/target/part_set.go?s=1179:1219#L54)
``` go
func (psh PartSetHeader) String() string
```



### <a name="PartSetHeader.WriteSignBytes">func</a> (PartSetHeader) [WriteSignBytes](/src/target/part_set.go?s=1488:1560#L66)
``` go
func (psh PartSetHeader) WriteSignBytes(w io.Writer, n *int, err *error)
```



## <a name="PartSetReader">type</a> [PartSetReader](/src/target/part_set.go?s=4723:4802#L222)
``` go
type PartSetReader struct {
    // contains filtered or unexported fields
}
```






### <a name="NewPartSetReader">func</a> [NewPartSetReader](/src/target/part_set.go?s=4804:4855#L228)
``` go
func NewPartSetReader(parts []*Part) *PartSetReader
```




### <a name="PartSetReader.Read">func</a> (\*PartSetReader) [Read](/src/target/part_set.go?s=4961:5020#L236)
``` go
func (psr *PartSetReader) Read(p []byte) (n int, err error)
```



## <a name="PrivValidator">type</a> [PrivValidator](/src/target/priv_validator.go?s=557:1241#L27)
``` go
type PrivValidator struct {
    Address       []byte           `json:"address"`
    PubKey        crypto.PubKey    `json:"pub_key"`
    LastHeight    int              `json:"last_height"`
    LastRound     int              `json:"last_round"`
    LastStep      int8             `json:"last_step"`
    LastSignature crypto.Signature `json:"last_signature"` // so we dont lose signatures
    LastSignBytes []byte           `json:"last_signbytes"` // so we dont lose signatures

    // PrivKey should be empty if a Signer other than the default is being used.
    PrivKey crypto.PrivKey `json:"priv_key"`
    Signer  `json:"-"`
    // contains filtered or unexported fields
}
```






### <a name="GenPrivValidator">func</a> [GenPrivValidator](/src/target/priv_validator.go?s=1899:1937#L73)
``` go
func GenPrivValidator() *PrivValidator
```
Generates a new validator with private key.


### <a name="LoadOrGenPrivValidator">func</a> [LoadOrGenPrivValidator](/src/target/priv_validator.go?s=2884:2943#L107)
``` go
func LoadOrGenPrivValidator(filePath string) *PrivValidator
```

### <a name="LoadPrivValidator">func</a> [LoadPrivValidator](/src/target/priv_validator.go?s=2458:2512#L93)
``` go
func LoadPrivValidator(filePath string) *PrivValidator
```




### <a name="PrivValidator.GetAddress">func</a> (\*PrivValidator) [GetAddress](/src/target/priv_validator.go?s=4092:4141#L156)
``` go
func (privVal *PrivValidator) GetAddress() []byte
```



### <a name="PrivValidator.Reset">func</a> (\*PrivValidator) [Reset](/src/target/priv_validator.go?s=3906:3943#L147)
``` go
func (privVal *PrivValidator) Reset()
```
NOTE: Unsafe!




### <a name="PrivValidator.Save">func</a> (\*PrivValidator) [Save](/src/target/priv_validator.go?s=3489:3525#L128)
``` go
func (privVal *PrivValidator) Save()
```



### <a name="PrivValidator.SetFile">func</a> (\*PrivValidator) [SetFile](/src/target/priv_validator.go?s=3352:3406#L122)
``` go
func (privVal *PrivValidator) SetFile(filePath string)
```



### <a name="PrivValidator.SetSigner">func</a> (\*PrivValidator) [SetSigner](/src/target/priv_validator.go?s=1777:1826#L68)
``` go
func (privVal *PrivValidator) SetSigner(s Signer)
```



### <a name="PrivValidator.SignProposal">func</a> (\*PrivValidator) [SignProposal](/src/target/priv_validator.go?s=4522:4606#L171)
``` go
func (privVal *PrivValidator) SignProposal(chainID string, proposal *Proposal) error
```



### <a name="PrivValidator.SignVote">func</a> (\*PrivValidator) [SignVote](/src/target/priv_validator.go?s=4171:4243#L160)
``` go
func (privVal *PrivValidator) SignVote(chainID string, vote *Vote) error
```



### <a name="PrivValidator.String">func</a> (\*PrivValidator) [String](/src/target/priv_validator.go?s=6462:6507#L231)
``` go
func (privVal *PrivValidator) String() string
```



## <a name="PrivValidatorsByAddress">type</a> [PrivValidatorsByAddress](/src/target/priv_validator.go?s=6689:6734#L237)
``` go
type PrivValidatorsByAddress []*PrivValidator
```









### <a name="PrivValidatorsByAddress.Len">func</a> (PrivValidatorsByAddress) [Len](/src/target/priv_validator.go?s=6736:6780#L239)
``` go
func (pvs PrivValidatorsByAddress) Len() int
```



### <a name="PrivValidatorsByAddress.Less">func</a> (PrivValidatorsByAddress) [Less](/src/target/priv_validator.go?s=6803:6857#L243)
``` go
func (pvs PrivValidatorsByAddress) Less(i, j int) bool
```



### <a name="PrivValidatorsByAddress.Swap">func</a> (PrivValidatorsByAddress) [Swap](/src/target/priv_validator.go?s=6923:6972#L247)
``` go
func (pvs PrivValidatorsByAddress) Swap(i, j int)
```



## <a name="Proposal">type</a> [Proposal](/src/target/proposal.go?s=324:712#L8)
``` go
type Proposal struct {
    Height           int              `json:"height"`
    Round            int              `json:"round"`
    BlockPartsHeader PartSetHeader    `json:"block_parts_header"`
    POLRound         int              `json:"pol_round"`    // -1 if null.
    POLBlockID       BlockID          `json:"pol_block_id"` // zero if null.
    Signature        crypto.Signature `json:"signature"`
}
```






### <a name="NewProposal">func</a> [NewProposal](/src/target/proposal.go?s=746:861#L18)
``` go
func NewProposal(height int, round int, blockPartsHeader PartSetHeader, polRound int, polBlockID BlockID) *Proposal
```
polRound: -1 if no polRound.





### <a name="Proposal.String">func</a> (\*Proposal) [String](/src/target/proposal.go?s=1044:1078#L28)
``` go
func (p *Proposal) String() string
```



### <a name="Proposal.WriteSignBytes">func</a> (\*Proposal) [WriteSignBytes](/src/target/proposal.go?s=1217:1299#L33)
``` go
func (p *Proposal) WriteSignBytes(chainID string, w io.Writer, n *int, err *error)
```



## <a name="Signable">type</a> [Signable](/src/target/signable.go?s=223:315#L3)
``` go
type Signable interface {
    WriteSignBytes(chainID string, w io.Writer, n *int, err *error)
}
```
Signable is an interface for all signable things.
It typically removes signatures before serializing.










## <a name="Signer">type</a> [Signer](/src/target/priv_validator.go?s=1433:1493#L50)
``` go
type Signer interface {
    Sign(msg []byte) crypto.Signature
}
```
This is used to sign votes.
It is the caller's duty to verify the msg before calling Sign,
eg. to avoid double signing.
Currently, the only callers are SignVote and SignProposal










## <a name="TMEventData">type</a> [TMEventData](/src/target/events.go?s=1466:1537#L27)
``` go
type TMEventData interface {
    events.EventData
    AssertIsTMEventData()
}
```
implements events.EventData










## <a name="Tx">type</a> [Tx](/src/target/tx.go?s=62:76#L1)
``` go
type Tx []byte
```









### <a name="Tx.Hash">func</a> (Tx) [Hash](/src/target/tx.go?s=329:355#L3)
``` go
func (tx Tx) Hash() []byte
```
NOTE: this is the hash of the go-wire encoded Tx.
Tx has no types at this level, so just length-prefixed.
Alternatively, it may make sense to add types here and let
[]byte be type 0x1 so we can have versioned txs if need be in the future.




## <a name="Txs">type</a> [Txs](/src/target/tx.go?s=401:414#L7)
``` go
type Txs []Tx
```









### <a name="Txs.Hash">func</a> (Txs) [Hash](/src/target/tx.go?s=416:444#L9)
``` go
func (txs Txs) Hash() []byte
```



## <a name="Validator">type</a> [Validator](/src/target/validator.go?s=304:508#L6)
``` go
type Validator struct {
    Address     []byte        `json:"address"`
    PubKey      crypto.PubKey `json:"pub_key"`
    VotingPower int64         `json:"voting_power"`
    Accum       int64         `json:"accum"`
}
```
Volatile state for each Validator
TODO: make non-volatile identity


	- Remove Accum - it can be computed, and now valset becomes identifying







### <a name="NewValidator">func</a> [NewValidator](/src/target/validator.go?s=510:579#L13)
``` go
func NewValidator(pubKey crypto.PubKey, votingPower int64) *Validator
```

### <a name="RandValidator">func</a> [RandValidator](/src/target/validator.go?s=2213:2292#L87)
``` go
func RandValidator(randPower bool, minPower int64) (*Validator, *PrivValidator)
```




### <a name="Validator.CompareAccum">func</a> (\*Validator) [CompareAccum](/src/target/validator.go?s=917:978#L30)
``` go
func (v *Validator) CompareAccum(other *Validator) *Validator
```
Returns the one with higher Accum.




### <a name="Validator.Copy">func</a> (\*Validator) [Copy](/src/target/validator.go?s=808:845#L24)
``` go
func (v *Validator) Copy() *Validator
```
Creates a new copy of the validator so we can mutate accum.
Panics if the validator is nil.




### <a name="Validator.Hash">func</a> (\*Validator) [Hash](/src/target/validator.go?s=1527:1560#L61)
``` go
func (v *Validator) Hash() []byte
```



### <a name="Validator.String">func</a> (\*Validator) [String](/src/target/validator.go?s=1339:1374#L50)
``` go
func (v *Validator) String() string
```



## <a name="ValidatorSet">type</a> [ValidatorSet](/src/target/validator_set.go?s=734:915#L14)
``` go
type ValidatorSet struct {
    Validators []*Validator // NOTE: persisted via reflect, must be exported.
    // contains filtered or unexported fields
}
```
ValidatorSet represent a set of *Validator at a given height.
The validators can be fetched by address or index.
The index is in order of .Address, so the indices are fixed
for all rounds of a given blockchain height.
On the other hand, the .AccumPower of each validator and
the designated .Proposer() of a set changes every round,
upon calling .IncrementAccum().
NOTE: Not goroutine-safe.
NOTE: All get/set to validators should copy the value for safety.
TODO: consider validator Accum overflow
TODO: move valset into an iavl tree where key is 'blockbonded|pubkey'







### <a name="NewValidatorSet">func</a> [NewValidatorSet](/src/target/validator_set.go?s=917:970#L22)
``` go
func NewValidatorSet(vals []*Validator) *ValidatorSet
```

### <a name="RandValidatorSet">func</a> [RandValidatorSet](/src/target/validator_set.go?s=9650:9743#L329)
``` go
func RandValidatorSet(numValidators int, votingPower int64) (*ValidatorSet, []*PrivValidator)
```
NOTE: PrivValidator are in order.





### <a name="ValidatorSet.Add">func</a> (\*ValidatorSet) [Add](/src/target/validator_set.go?s=4027:4087#L130)
``` go
func (valSet *ValidatorSet) Add(val *Validator) (added bool)
```



### <a name="ValidatorSet.Copy">func</a> (\*ValidatorSet) [Copy](/src/target/validator_set.go?s=1944:1992#L58)
``` go
func (valSet *ValidatorSet) Copy() *ValidatorSet
```



### <a name="ValidatorSet.GetByAddress">func</a> (\*ValidatorSet) [GetByAddress](/src/target/validator_set.go?s=2630:2714#L78)
``` go
func (valSet *ValidatorSet) GetByAddress(address []byte) (index int, val *Validator)
```



### <a name="ValidatorSet.GetByIndex">func</a> (\*ValidatorSet) [GetByIndex](/src/target/validator_set.go?s=3026:3108#L89)
``` go
func (valSet *ValidatorSet) GetByIndex(index int) (address []byte, val *Validator)
```



### <a name="ValidatorSet.HasAddress">func</a> (\*ValidatorSet) [HasAddress](/src/target/validator_set.go?s=2330:2389#L71)
``` go
func (valSet *ValidatorSet) HasAddress(address []byte) bool
```



### <a name="ValidatorSet.Hash">func</a> (\*ValidatorSet) [Hash](/src/target/validator_set.go?s=3753:3794#L119)
``` go
func (valSet *ValidatorSet) Hash() []byte
```



### <a name="ValidatorSet.IncrementAccum">func</a> (\*ValidatorSet) [IncrementAccum](/src/target/validator_set.go?s=1304:1357#L39)
``` go
func (valSet *ValidatorSet) IncrementAccum(times int)
```
TODO: mind the overflow when times and votingPower shares too large.




### <a name="ValidatorSet.Iterate">func</a> (\*ValidatorSet) [Iterate](/src/target/validator_set.go?s=5846:5922#L189)
``` go
func (valSet *ValidatorSet) Iterate(fn func(index int, val *Validator) bool)
```



### <a name="ValidatorSet.Proposer">func</a> (\*ValidatorSet) [Proposer](/src/target/validator_set.go?s=3473:3533#L107)
``` go
func (valSet *ValidatorSet) Proposer() (proposer *Validator)
```



### <a name="ValidatorSet.Remove">func</a> (\*ValidatorSet) [Remove](/src/target/validator_set.go?s=5160:5241#L169)
``` go
func (valSet *ValidatorSet) Remove(address []byte) (val *Validator, removed bool)
```



### <a name="ValidatorSet.Size">func</a> (\*ValidatorSet) [Size](/src/target/validator_set.go?s=3178:3216#L94)
``` go
func (valSet *ValidatorSet) Size() int
```



### <a name="ValidatorSet.String">func</a> (\*ValidatorSet) [String](/src/target/validator_set.go?s=8327:8370#L271)
``` go
func (valSet *ValidatorSet) String() string
```



### <a name="ValidatorSet.StringIndented">func</a> (\*ValidatorSet) [StringIndented](/src/target/validator_set.go?s=8410:8474#L275)
``` go
func (valSet *ValidatorSet) StringIndented(indent string) string
```



### <a name="ValidatorSet.TotalVotingPower">func</a> (\*ValidatorSet) [TotalVotingPower](/src/target/validator_set.go?s=3253:3305#L98)
``` go
func (valSet *ValidatorSet) TotalVotingPower() int64
```



### <a name="ValidatorSet.Update">func</a> (\*ValidatorSet) [Update](/src/target/validator_set.go?s=4858:4923#L156)
``` go
func (valSet *ValidatorSet) Update(val *Validator) (updated bool)
```



### <a name="ValidatorSet.VerifyCommit">func</a> (\*ValidatorSet) [VerifyCommit](/src/target/validator_set.go?s=6087:6194#L199)
``` go
func (valSet *ValidatorSet) VerifyCommit(chainID string, blockID BlockID, height int, commit *Commit) error
```
Verify that +2/3 of the set had signed the given signBytes




### <a name="ValidatorSet.VerifyCommitAny">func</a> (\*ValidatorSet) [VerifyCommitAny](/src/target/validator_set.go?s=7832:7942#L247)
``` go
func (valSet *ValidatorSet) VerifyCommitAny(chainID string, blockID BlockID, height int, commit *Commit) error
```
Verify that +2/3 of this set had signed the given signBytes.
Unlike VerifyCommit(), this function can verify commits with differeent sets.




## <a name="ValidatorsByAddress">type</a> [ValidatorsByAddress](/src/target/validator_set.go?s=8972:9009#L299)
``` go
type ValidatorsByAddress []*Validator
```









### <a name="ValidatorsByAddress.Len">func</a> (ValidatorsByAddress) [Len](/src/target/validator_set.go?s=9011:9050#L301)
``` go
func (vs ValidatorsByAddress) Len() int
```



### <a name="ValidatorsByAddress.Less">func</a> (ValidatorsByAddress) [Less](/src/target/validator_set.go?s=9072:9121#L305)
``` go
func (vs ValidatorsByAddress) Less(i, j int) bool
```



### <a name="ValidatorsByAddress.Swap">func</a> (ValidatorsByAddress) [Swap](/src/target/validator_set.go?s=9185:9229#L309)
``` go
func (vs ValidatorsByAddress) Swap(i, j int)
```



## <a name="Vote">type</a> [Vote](/src/target/vote.go?s=1065:1488#L39)
``` go
type Vote struct {
    ValidatorAddress []byte           `json:"validator_address"`
    ValidatorIndex   int              `json:"validator_index"`
    Height           int              `json:"height"`
    Round            int              `json:"round"`
    Type             byte             `json:"type"`
    BlockID          BlockID          `json:"block_id"` // zero if vote is nil.
    Signature        crypto.Signature `json:"signature"`
}
```
Represents a prevote, precommit, or commit vote from validators for consensus.










### <a name="Vote.Copy">func</a> (\*Vote) [Copy](/src/target/vote.go?s=1665:1695#L56)
``` go
func (vote *Vote) Copy() *Vote
```



### <a name="Vote.String">func</a> (\*Vote) [String](/src/target/vote.go?s=1738:1771#L61)
``` go
func (vote *Vote) String() string
```



### <a name="Vote.WriteSignBytes">func</a> (\*Vote) [WriteSignBytes](/src/target/vote.go?s=1490:1571#L49)
``` go
func (vote *Vote) WriteSignBytes(chainID string, w io.Writer, n *int, err *error)
```



## <a name="VoteSet">type</a> [VoteSet](/src/target/vote_set.go?s=1586:2119#L36)
``` go
type VoteSet struct {
    // contains filtered or unexported fields
}
```
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







### <a name="NewVoteSet">func</a> [NewVoteSet](/src/target/vote_set.go?s=2205:2302#L53)
``` go
func NewVoteSet(chainID string, height int, round int, type_ byte, valSet *ValidatorSet) *VoteSet
```
Constructs a new VoteSet struct used to accumulate votes for given height/round.





### <a name="VoteSet.AddVote">func</a> (\*VoteSet) [AddVote](/src/target/vote_set.go?s=3699:3766#L116)
``` go
func (voteSet *VoteSet) AddVote(vote *Vote) (added bool, err error)
```
Returns added=true if vote is valid and new.
Otherwise returns err=ErrVote[


	UnexpectedStep | InvalidIndex | InvalidAddress |
	InvalidSignature | InvalidBlockHash | ConflictingVotes ]

Duplicate votes return added=false, err=nil.
Conflicting votes return added=*, err=ErrVoteConflictingVotes.
NOTE: vote should not be mutated after adding.
NOTE: VoteSet must not be nil




### <a name="VoteSet.BitArray">func</a> (\*VoteSet) [BitArray](/src/target/vote_set.go?s=9430:9474#L309)
``` go
func (voteSet *VoteSet) BitArray() *BitArray
```



### <a name="VoteSet.BitArrayByBlockID">func</a> (\*VoteSet) [BitArrayByBlockID](/src/target/vote_set.go?s=9602:9670#L318)
``` go
func (voteSet *VoteSet) BitArrayByBlockID(blockID BlockID) *BitArray
```



### <a name="VoteSet.ChainID">func</a> (\*VoteSet) [ChainID](/src/target/vote_set.go?s=2787:2827#L72)
``` go
func (voteSet *VoteSet) ChainID() string
```



### <a name="VoteSet.GetByAddress">func</a> (\*VoteSet) [GetByAddress](/src/target/vote_set.go?s=10127:10185#L341)
``` go
func (voteSet *VoteSet) GetByAddress(address []byte) *Vote
```



### <a name="VoteSet.GetByIndex">func</a> (\*VoteSet) [GetByIndex](/src/target/vote_set.go?s=9950:10004#L332)
``` go
func (voteSet *VoteSet) GetByIndex(valIndex int) *Vote
```
NOTE: if validator has conflicting votes, returns "canonical" vote




### <a name="VoteSet.HasAll">func</a> (\*VoteSet) [HasAll](/src/target/vote_set.go?s=11027:11064#L384)
``` go
func (voteSet *VoteSet) HasAll() bool
```



### <a name="VoteSet.HasTwoThirdsAny">func</a> (\*VoteSet) [HasTwoThirdsAny](/src/target/vote_set.go?s=10828:10874#L375)
``` go
func (voteSet *VoteSet) HasTwoThirdsAny() bool
```



### <a name="VoteSet.HasTwoThirdsMajority">func</a> (\*VoteSet) [HasTwoThirdsMajority](/src/target/vote_set.go?s=10435:10486#L354)
``` go
func (voteSet *VoteSet) HasTwoThirdsMajority() bool
```



### <a name="VoteSet.Height">func</a> (\*VoteSet) [Height](/src/target/vote_set.go?s=2857:2893#L76)
``` go
func (voteSet *VoteSet) Height() int
```



### <a name="VoteSet.IsCommit">func</a> (\*VoteSet) [IsCommit](/src/target/vote_set.go?s=10608:10647#L363)
``` go
func (voteSet *VoteSet) IsCommit() bool
```



### <a name="VoteSet.MakeCommit">func</a> (\*VoteSet) [MakeCommit](/src/target/vote_set.go?s=12591:12635#L445)
``` go
func (voteSet *VoteSet) MakeCommit() *Commit
```



### <a name="VoteSet.Round">func</a> (\*VoteSet) [Round](/src/target/vote_set.go?s=2968:3003#L84)
``` go
func (voteSet *VoteSet) Round() int
```



### <a name="VoteSet.SetPeerMaj23">func</a> (\*VoteSet) [SetPeerMaj23](/src/target/vote_set.go?s=8534:8602#L274)
``` go
func (voteSet *VoteSet) SetPeerMaj23(peerID string, blockID BlockID)
```
If a peer claims that it has 2/3 majority for given blockKey, call this.
NOTE: if there are too many peers, or too much peer churn,
this can cause memory issues.
TODO: implement ability to remove peers too
NOTE: VoteSet must not be nil




### <a name="VoteSet.Size">func</a> (\*VoteSet) [Size](/src/target/vote_set.go?s=3190:3224#L100)
``` go
func (voteSet *VoteSet) Size() int
```



### <a name="VoteSet.String">func</a> (\*VoteSet) [String](/src/target/vote_set.go?s=11541:11580#L403)
``` go
func (voteSet *VoteSet) String() string
```



### <a name="VoteSet.StringIndented">func</a> (\*VoteSet) [StringIndented](/src/target/vote_set.go?s=11668:11728#L410)
``` go
func (voteSet *VoteSet) StringIndented(indent string) string
```



### <a name="VoteSet.StringShort">func</a> (\*VoteSet) [StringShort](/src/target/vote_set.go?s=12185:12229#L432)
``` go
func (voteSet *VoteSet) StringShort() string
```



### <a name="VoteSet.TwoThirdsMajority">func</a> (\*VoteSet) [TwoThirdsMajority](/src/target/vote_set.go?s=11271:11341#L390)
``` go
func (voteSet *VoteSet) TwoThirdsMajority() (blockID BlockID, ok bool)
```
Returns either a blockhash (or nil) that received +2/3 majority.
If there exists no such majority, returns (nil, PartSetHeader{}, false).




### <a name="VoteSet.Type">func</a> (\*VoteSet) [Type](/src/target/vote_set.go?s=3078:3113#L92)
``` go
func (voteSet *VoteSet) Type() byte
```



## <a name="VoteSetReader">type</a> [VoteSetReader](/src/target/vote_set.go?s=14405:14551#L509)
``` go
type VoteSetReader interface {
    Height() int
    Round() int
    Type() byte
    Size() int
    BitArray() *BitArray
    GetByIndex(int) *Vote
    IsCommit() bool
}
```
Common interface between *consensus.VoteSet and types.Commit














- - -
Generated by [godoc2md](http://godoc.org/github.com/davecheney/godoc2md)
