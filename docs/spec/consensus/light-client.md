# Lite Client

A lite client is a process that connects to the Tendermint Full Node(s) and then tries to verify application data using the Merkle proofs. In this document we describe mechanisms that ensures that the Tendermint lite client
has the same level of security as Full Node processes (without being itself a Full Node).

## Lite client requirements from Tendermint and Proof of Stake modules

Before explaining lite client mechanisms and operations we need to define some requirements expected from the Tendermint blockchain. Tendermint provides a deterministic, Byzantine fault-tolerant, source of time (called
[BFT Time](/Users/zarkomilosevic/go-workspace/src/github.com/tendermint/tendermint/docs/spec/consensus/bft-time.md)).
BFT time is monotonically increasing and in case of at most 1/3 of voting power equivalent of faulty validators guaranteed to be close to the wall time of correct validators. For correct functioning of lite client we need a guarantee that BFT time does not drift more than some known parameter BFT_TIME_DRIFT_BOUND (that should normally be measured in hours, maybe even days) from client wall time. Note that this requirement currently only holds in case 
at most 1/3 of voting power equivalent of validators report wrong time, but we might need to strengthen this requirement further to also be able to tolerate time-related misbehavior of more than 1/3 voting power equivalent of validators (https://github.com/tendermint/tendermint/issues/2653, https://github.com/tendermint/tendermint/issues/2840).  

Furthermore, lite client security is tightly coupled with the notion of UNBONDING_PERIOD that is at the core of the security of proof of stake blockchain systems (for example Cosmos Hub). UNBONDING_PERIOD is period of time that needs to pass from the withdraw event until stake is liquid. During this period unbonded validator cannot participate in the consensus protocol (and is therefore not rewarded) but can be slashed for misbehavior (done either before withdraw event or during UNBONDING_PERIOD). This is used to protect against a validator attacking the network and then immediately withdrawing his stake. Cosmos Hub is currently enforcing a 21-day UNBONDING_PERIOD. Note that UNBONDING_PERIOD is measured with respect to BFT time and that this has significant effect on the security of lite client operation as validators are not slashable outside their UNBONDING_PERIOD. There is a hidden implicit assumptions regarding the UNBONDING_PERIOD: we assume that Tendermint will always generate blocks within duration of UNBONDING_PERIOD. If chain halts for the duration of UNBONDING_PERIOD security of lite clients are jeopardized. Probably more secure solution would be defining UNBONDING_PERIOD as a hybrid of wall time and logical time (number of block heights). In that case UNBONDING_PERIOD is over when the both conditions are true. In that case no assumption is being made on the chain progress (which is in theory hard to make as Tendermint operate in partially synchronous system model), and system is secure (including lite clients) even in case of long halts.   

## Lite client initialization

Lite client is initialized using a trusted socially verified validator set hash. This is part of the security model for
proof of stake blockchains (see https://github.com/tendermint/tendermint/issues/1771). We assume that initialization 
procedure receive as input the following parameters: 

- height h: height of the blockchain for which trusted validator set hash is provided and
- trusted validator set hash.

Given this information, light client initialization logic will call `Validators` method on the node it is connected to 
obtain `ResultValidators` that contains validators that has committed the block h. Then we check if MerkleRoot
of the validator set is equal to the trusted validator set hash. If verification failed, initialization exits with error, otherwise it proceeds. 

Next step is determining if the block at hight h is correctly signed by the obtained validator set. This is achieved by 
calling `Commit` method for a given height that returns `SignedHeader` that is a header together with the set of 
`Precommit` messages that commit the corresponding block. Light client logic then needs to verify if the set of signatures defined the valid commit for the given block. If this verification fails, initialization exits with error, otherwise it proceeds. 

Final step (this should actually be part of the second step) of the initialization procedure is ensuring that we operate within UNBONDING_PERIOD of the trusted validator set. This can be expressed with the following formula:

`header.Time + UNBONDING_PERIOD <= Now() - BFT_TIME_DRIFT_BOUND`.

Note that outside this time window lite client cannot trust validator set as validators could potentially unbonded its stake so security of the lite client does not hold as they are not slashable for its actions. Therefore, they can eclipse client and cheat about the system state without risk of being punished for such misbehavior. 

Note that this formula shows a fundamental dependence of lite client security on the wall time. If UNBONDING_PERIOD
would be defined only in terms of logical time (block heights), lite client will not have a way to know if trusted validator set is still withing its UNBONDING_PERIOD as it does not have a way of reliably determining top of the chain.
Therefore, it seems that having BFT time in sync with standard notions of time (for example NTP) is necessarily for correct operations of the system. 

Lite client security depends also on the guarantee that faulty validator behavior will be punished. Therefore if a client detect faulty behavior we need to guarantee that proof of misbehavior evidence transaction will be committed within UNBONDING_PERIOD of faulty validators so it can be slashed. This can be achieved by having client considering
time period smaller than UNBONDING_PERIOD. Let's denote this time period LITE_CLIENT_CERTIFICATE_PERIOD, and we can assume that it's set to a fraction of UNBONDING_PERIOD. For example, it can be set to be half of UNBONDING_PERIOD. 
This normally leaves a lot of time to client to ensure that evidence transaction is committed by the network during
the UNBONDING_PERIOD. As these periods are normally measured in days, a client can in the worst case rely on 
social channels to ensure that evidence transaction is proposed by some validator. Note that we might also assume that as part of validator service some validators might have "hot line" that can be used to submit evidence of misbehavior. 

Given a trusted validator set whose trust is established by the header H, we say that this validator set is trusted the latest until `H.Time + LITE_CLIENT_CERTIFICATE_PERIOD`, i.e., the trusted period is defined by the following formula:
`header.Time + LITE_CLIENT_CERTIFICATE_PERIOD <= Now() - BFT_TIME_DRIFT_BOUND`. If this formula does not hold, we say
that trust in the given validator set is revoked and client need to establish a new root of trust.

For the purpose of this lite client specification, we assume that the Tendermint Full Node exposes the following functions over Tendermint RPC:

```golang
Validators(height int64) (ResultValidators, error) // returns validator set for the given height
Commit(height int64) (SignedHeader, error) // returns signed header for the given height

type ResultValidators struct {
    BlockHeight   int64
    Validators    []Validator
}

type SignedHeader struct {
    Header        Header
    Commit        Commit
}
```
Locally, lite client manages the following state:

```golang
type LiteClientState struct {
  ValSet            []Validator        // validator set at the given height
  ValSetHash        []byte             // hash of the current validator set
  NextValSet        []Validator        // next validator set
  NextValSetHash    []byte             // hash of the next validator set
  Height            int64              // current height
  ValSetTime        int64              // time when the current and next validator set is initialized
}
```

Initialization procedure is captured by the following pseudo code:

```golang
Init(height int64, valSetHash Hash): (LiteClientState, error) {
  state = LiteClientState {}
  valSet = Validators(height).Validators
  if Hash(valSet) != valSetHash then return (nil, error)

  signedHeader = Commit(height)
  if signedHeader.Header.Time + LITE_CLIENT_CERTIFICATE_PERIOD > Now() - BFT_TIME_DRIFT_BOUND then return (nil, error)
  if votingPower(signedHeader.Commit) <= 2/3 totalVotingPower(valSet) then return (nil, error)
  
  nextValSet = Validators(height+1).Validators
  if Hash(nextValSet) != signedHeader.Header.NextValSetHash then return (nil, error)

  state.ValSet = nextValSet
  state.ValSetHash = signedHeader.Header.NextValSetHash
  state.Height = height
  state.ValSetTime = signedHeader.Header.Time
  
  return state, nil
}  
```

To be able to validate a Merkle proof, a light client needs to validate the blockchain header that contains the root app hash.Validating a blockchain header in Tendermint consists in verifying that the header is committed (signed) by >2/3 of the voting power of the corresponding validator set. As the validator set is a dynamic set (it is changing), one of the core functionality of the lite client is updating the current validator set, that is then used to verify the
blockchain header, and further the corresponding Merkle proofs.

We assume that each validator set change is signed (committed) by the current validator set. More precisely,
given a block `H` that contains transactions that are modifying the current validator set, the Merkle root hash of the next validator set (modified based on transactions from block H) are in block `H+1` (and signed by the current validator
set), and then starting from the block `H+2`, it will be signed by the next validator set.

The core of the light client logic is captured by the VerifyAndUpdate function that is used to update
trusted validator set to the one from the given height.

```golang
VerifyAndUpdate(state LiteClientState, height int64): error {
  if signedHeader.Header.Height <= height then return error
  
  // check if certification period of the current validator set is still secure
  if state.ValSetTime + LITE_CLIENT_CERTIFICATE_PERIOD > Now() - BFT_TIME_DRIFT_BOUND then return error

  for i = state.Height+1; i < height; i++ {
    signedHeader = Commit(i)
    if votingPower(signedHeader.Commit) <= 2/3 totalVotingPower(state.ValSet) then return error
    if signedHeader.Header.ValidatorsHash != state.ValSetHash then return error

    nextValSetHash = signedHeader.Header.NextValidatorsHash

    nextValSet = Validators(i+1).Validators
    if Hash(nextValSet) != nextValSetHash then return (0, error)

    // at this point we can install new valset
    state.ValSet = nextValSet
    state.ValSetHash = nextValSetHash
    state.Height = i
    state.ValSetTime = signedHeader.Header.Time
  }  
}

Note that in the logic above we assume that the lite client will always go upward with respect to header verifications,
i.e., that it will always be used to verify more recent headers. Going backward is problematic as having trust in 
some validator set at height h does not give us trust in validator set before height h. Therefore, if lite client query
is about smaller height, trusted validator set for smaller heights is needed. 

In case a call to the FullNode or subsequent checks fail, a light client need to implement some recovery strategy, for example connecting to other FullNode.
