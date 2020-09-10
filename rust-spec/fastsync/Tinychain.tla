-------------------------- MODULE Tinychain ----------------------------------
(* A very abstract model of Tendermint blockchain. Its only purpose is to highlight
   the relation between validator sets, next validator sets, and last commits.
 *)

EXTENDS Integers

\* type annotation
a <: b == a

\* the type of validator sets, e.g., STRING
VST == STRING

\* LastCommit type.
\* It contains:
\* 1) the flag of whether the block id equals to the hash of the previous block, and
\* 2) the set of the validators who have committed the block.
\* In the implementation, blockId is the hash of the previous block, which cannot be forged.
\* We abstract block id into whether it equals to the hash of the previous block or not.
LCT == [blockIdEqRef |-> BOOLEAN, committers |-> VST]

\* Block type.
\* A block contains its height, validator set, next validator set, and last commit.
\* Moreover, it contains the flag that tells us whether the block is equal to the one
\* on the reference chain (this is an abstraction of hash).
BT == [height |-> Int, hashEqRef |-> BOOLEAN, wellFormed |-> BOOLEAN,
       VS |-> VST, NextVS |-> VST, lastCommit |-> LCT]

CONSTANTS
    (*
       A set of abstract values, each value representing a set of validators.
       For the purposes of this specification, they can be any values,
       e.g., "s1", "s2", etc.
     *)
    VALIDATOR_SETS,
    (* a nil validator set that is outside of VALIDATOR_SETS *)
    NIL_VS,
    (* The maximal height, up to which the blockchain may grow *)
    MAX_HEIGHT
    
Heights == 1..MAX_HEIGHT    

\* the set of all potential commits
Commits == [blockIdEqRef: BOOLEAN, committers: VALIDATOR_SETS]

\* the set of all potential blocks, not necessarily coming from the blockchain
Blocks ==
  [height: Heights, hashEqRef: BOOLEAN, wellFormed: BOOLEAN,
   VS: VALIDATOR_SETS, NextVS: VALIDATOR_SETS, lastCommit: Commits]

\* Does the chain contain a sound sequence of blocks that could be produced by
\* a 2/3 of faulty validators. This operator can be used to initialise the chain!
\* Since we are abstracting validator sets with VALIDATOR_SETS, which are
\* 2/3 quorums, we just compare committers to those sets. In a more detailed
\* specification, one would write the \subseteq operator instead of equality.
IsCorrectChain(chain) ==
    \* restrict the structure of the blocks, to decrease the TLC search space
    LET OkCommits == [blockIdEqRef: {TRUE}, committers: VALIDATOR_SETS]
        OkBlocks == [height: Heights, hashEqRef: {TRUE}, wellFormed: {TRUE},
                     VS: VALIDATOR_SETS, NextVS: VALIDATOR_SETS, lastCommit: OkCommits]
    IN
    /\ chain \in [1..MAX_HEIGHT -> OkBlocks]
    /\ \A h \in 1..MAX_HEIGHT:
        LET b == chain[h] IN
        /\ b.height = h     \* the height is correct
        /\ h > 1 =>
             LET p == chain[h - 1] IN
             /\ b.VS = p.NextVS     \* the validators propagate from the previous block
             /\ b.lastCommit.committers = p.VS  \* and they are the committers
    

\* The basic properties of blocks on the blockchain:
\* They should pass the validity check and they may verify the next block.    

\* Does the block pass the consistency check against the next validators of the previous block
IsMatchingValidators(block, nextVS) ==
    \* simply check that the validator set is propagated correctly.
    \* (the implementation tests hashes and the application state)
    block.VS = nextVS

\* Does the block verify the commit (of the next block)
PossibleCommit(block, commit) ==
    \* the commits are signed by the block validators
    /\ commit.committers = block.VS
    \* The block id in the commit matches the block hash (abstract comparison).
    \* (The implementation has extensive tests for that.)
    \* this is an abstraction of: commit.blockId = hash(block)
    \* 
    \* These are possible scenarios on the concrete hashes:
    \*
    \* scenario 1: commit.blockId = 10 /\ hash(block) = 10 /\ hash(ref) = 10
    \* scenario 2: commit.blockId = 20 /\ hash(block) = 20 /\ block.VS /= ref.VS
    \* scenario 3: commit.blockId = 50 /\ hash(block) = 100
    \* scenario 4: commit.blockId = 10 /\ hash(block) = 100
    \* scenario 5: commit.blockId = 100 /\ hash(block) = 10
    /\ commit.blockIdEqRef = block.hashEqRef
    \* the following test would be cheating, as we do not have access to the
    \* reference chain:
    \* /\ commit.blockIdEqRef

\* Basic invariants

\* every block has the validator set that is chosen by its predecessor
ValidBlockInv(chain) ==
    \A h \in 2..MAX_HEIGHT:
        IsMatchingValidators(chain[h], chain[h - 1].NextVS)

\* last commit of every block is signed by the validators of the predecessor     
VerifiedBlockInv(chain) ==
    \A h \in 2..MAX_HEIGHT:
        PossibleCommit(chain[h - 1], chain[h].lastCommit)

==================================================================================
