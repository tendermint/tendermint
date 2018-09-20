# Light Client

A light client is a process that connects to the Tendermint Full Node(s) and then tries to verify the Merkle proofs
about the blockchain application. In this document we describe mechanisms that ensures that the Tendermint light client
has the same level of security as Full Node processes (without being itself a Full Node).

To be able to validate a Merkle proof, a light client needs to validate the blockchain header that contains the root app hash.
Validating a blockchain header in Tendermint consists in verifying that the header is committed (signed) by >2/3 of the
voting power of the corresponding validator set. As the validator set is a dynamic set (it is changing), one of the
core functionality of the light client is updating the current validator set, that is then used to verify the
blockchain header, and further the corresponding Merkle proofs.

For the purpose of this light client specification, we assume that the Tendermint Full Node exposes the following functions over
Tendermint RPC:

```golang
Header(height int64) (SignedHeader, error) // returns signed header for the given height
Validators(height int64) (ResultValidators, error) // returns validator set for the given height
LastHeader(valSetNumber int64) (SignedHeader, error)   // returns last header signed by the validator set with the given validator set number

type SignedHeader struct {
    Header        Header
    Commit        Commit
    ValSetNumber  int64
}

type ResultValidators struct {
    BlockHeight   int64
    Validators    []Validator
    // time the current validator set is initialised, i.e, time of the last validator change before header BlockHeight
    ValSetTime    int64
}
```

We assume that Tendermint keeps track of the validator set changes and that each time a validator set is changed it is
being assigned the next sequence number. We can call this number the validator set sequence number. Tendermint also remembers
the Time from the header when the next validator set is initialised (starts to be in power), and we refer to this time
as validator set init time.
Furthermore, we assume that each validator set change is signed (committed) by the current validator set. More precisely,
given a block `H` that contains transactions that are modifying the current validator set, the Merkle root hash of the next
validator set (modified based on transactions from block H) will be in block `H+1` (and signed by the current validator
set), and then starting from the block `H+2`, it will be signed by the next validator set.

Note that the real Tendermint RPC API is slightly different (for example, response messages contain more data and function
names are slightly different); we shortened (and modified) it for the purpose of this document to make the spec more
clear and simple. Furthermore, note that in case of the third function, the returned header has `ValSetNumber` equals to
`valSetNumber+1`.

Locally, light client manages the following state:

```golang
valSet        []Validator    // current validator set (last known and verified validator set)
valSetNumber  int64          // sequence number of the current validator set
valSetHash    []byte         // hash of the current validator set
valSetTime    int64          // time when the current validator set is initialised
```

The light client is initialised with the trusted validator set, for example based on the known validator set hash,
validator set sequence number and the validator set init time.
The core of the light client logic is captured by the VerifyAndUpdate function that is used to 1) verify if the given header is valid,
and 2) update the validator set (when the given header is valid and it is more recent than the seen headers).

```golang
VerifyAndUpdate(signedHeader SignedHeader):
  assertThat signedHeader.valSetNumber >= valSetNumber
  if isValid(signedHeader) and signedHeader.Header.Time <= valSetTime + UNBONDING_PERIOD then
    setValidatorSet(signedHeader)
    return true
  else
    updateValidatorSet(signedHeader.ValSetNumber)
    return VerifyAndUpdate(signedHeader)

isValid(signedHeader SignedHeader):
  valSetOfTheHeader = Validators(signedHeader.Header.Height)
  assertThat Hash(valSetOfTheHeader) == signedHeader.Header.ValSetHash
  assertThat signedHeader is passing basic validation
  if votingPower(signedHeader.Commit) > 2/3 * votingPower(valSetOfTheHeader) then return true
  else
    return false

setValidatorSet(signedHeader SignedHeader):
  nextValSet = Validators(signedHeader.Header.Height)
  assertThat Hash(nextValSet) == signedHeader.Header.ValidatorsHash
  valSet = nextValSet.Validators
  valSetHash = signedHeader.Header.ValidatorsHash
  valSetNumber = signedHeader.ValSetNumber
  valSetTime = nextValSet.ValSetTime

votingPower(commit Commit):
  votingPower = 0
  for each precommit in commit.Precommits do:
    if precommit.ValidatorAddress is in valSet and signature of the precommit verifies then
      votingPower += valSet[precommit.ValidatorAddress].VotingPower
  return votingPower

votingPower(validatorSet []Validator):
  for each validator in validatorSet do:
    votingPower += validator.VotingPower
  return votingPower

updateValidatorSet(valSetNumberOfTheHeader):
  while valSetNumber != valSetNumberOfTheHeader do
    signedHeader = LastHeader(valSetNumber)
    if isValid(signedHeader) then
      setValidatorSet(signedHeader)
    else return error
  return
```

Note that in the logic above we assume that the light client will always go upward with respect to header verifications,
i.e., that it will always be used to verify more recent headers. In case a light client needs to be used to verify older
headers (go backward) the same mechanisms and similar logic can be used. In case a call to the FullNode or subsequent
checks fail, a light client need to implement some recovery strategy, for example connecting to other FullNode.
