# Fork Detection and processing of misbehavior with lite clients

As explained in [fork-accountability](fork-accountability.md) document we say that a fork is a case in which an honest process (validator, full node or lite client)
observed two commits for different blocks at the same height of the blockchain. With Tendermint consensus protocol, a fork can only
happen if [Tendermint-Failure-Model](light-client.md#tendermint-failure-model) does not hold, i.e., we have more than 1/3 of voting power under control
of faulty validators in the relevant validator set.
In this document we focus on detections of forks observed only by lite clients, i.e., we assume that there are no forks 
on the main chain, and that a fork is used as an attack to some lite client.

There are several sub-problems we will cover in this document:

- a problem of detecting a fork by an honest lite client,
  
- identifying faulty validators (and not mistakenly accusing correct validators) and
  
- processing misbehavior on the main chain (if possible).  

## Detecting forks

A fork is defined as a deviation (of list of blocks) from the main chain. In this document we will assume that there exists 
a main chain *C1* (sequence of blocks) and an honest full node *F* that is synced to the chain *C1*. Then a fork corresponds 
to an existence of a commit for block *B* for height *h* that is different from a block at height *h*
on the chain *C1*. Furthermore, a commit for the block *B* is a *valid* fork if it contains at least a single valid signature 
by a validator from the validator set at height *h* at chain *C1*. This definition of valid fork is needed to differentiate
forks that can provide material for punishment mechanisms (slashing) from bogus data that looks like a valid commit
but is signed by processes that are not tracked by the system (and therefore can't be punished).

As explained in [lite-client](light-client.md) document a lite client establish a connection with a full node and it verifies application states 
of interest by downloading and verifying corresponding block headers. To be able to detect a fork, we assume an additional 
module (fork detection module) that runs in parallel with the core lite client module, and whose role is fork detection. 
The fork detection module maintains a list of peers (full nodes) and it periodically establishes a connection to some peer 
(called witness) and verify correctness of headers downloaded by main module (by cross-checking with headers downloaded
from witnesses). In case there are two different headers for the same height signed by overlapping validator sets,
then a client downloads corresponding commit and creates a proof of valid fork.

## Identifying faulty validators 

Once an honest lite client detects a valid fork it needs to submit it to an honest full node for further processing so
that faulty validators can be detected and punished. For the purpose of this document we assume that a lite client
is able to submit a proof of fork to an honest full node in reasonable time (how we can ensure that will be addressed in a
separate document). 

For the purpose of this document we assume existence of the following information for each commit: hash of the block, 
height, round, validator set and a set of signatures by the processes in the given validator set. An honest full node starts
the procedure that detects misbehaving validators from the following information: for some height *h*, there are two
conflicting commits *C1* and *C2*, where *C1* is a commit from the main chain. The detection procedure works like this:  

1) if there are processes from the set *signers(C2)* that are not part of *C1.valset*, they misbehaved as they are signing 
   protocol messages in heights they are not validators. In this case commit *C2* is a self-contained evidence of misbehavior 
   for those processes that can be simply verified by every honest validators without additional information.  
   For processes from the set *signers(C2)* that are part of *C1.valset* we need additional checks:

2) if *C1.round == C2.round*, and some processes signed different precommit messages in both commits, then it is an 
   equivocation misbehavior. Similarly as above, commits *C1* and *C2* are an evidence of misbehavior. Note that in this case
   there might be additional misbehavior, but its detection require more complex detection procedure that we explain next.  

3) if *C1.round != C2.round* we need to run full detection procedure. We assume (in the first version) that the full detection
   procedure is executed by a centralized, trusted component called monitor that will be processing proof of forks and 
   identifying faulty processes (and optionally generating evidences of misbehavior). Monitor is an honest full node that runs 
   on the main chain. Monitor is triggered by receiving two conflicting commits where *C1.round != C2.round*. The procedure starts
   by declaring processes that are in the set *C1.valset* as suspected. This can be for example done by posting a special transaction
   on the main chain. After suspected processes are declared, they are obliged to submit vote sets (prevote and precommit messages) for 
   the given height for the subset of rounds *[C1.round, C2.round]* to the monitor process within *MONITOR_RESPONSE_PERIOD*.
   Validators that do not submit its vote sets within this period are considered faulty (Q: how can we create verifiable
   proof of absence of vote sets?). 

   Once monitor receives vote sets from all validators or after *MONITOR_RESPONSE_PERIOD* is over, it executes the following
   protocol:

   1) Preprocessing step: the received messages from all vote sets are added to the appropriate vote sets of the senders. 
      More precisely, if in the vote set of process *p1* there is a message *m* sent by process *p2*, we will add this message to
      the vote set of the process *p2*. In essence, during this step we are computing what we call *complete vote sets*: union of 
      vote sets sent by some process *p* together with messages sent by *p* that are found in other vote sets.
   
   2) Analysis step: we analyze vote sets of all processes independently and determine whether they are correct or faulty. Analysis 
      of the vote set works as follows:
      
      - if more than one PREVOTE/PRECOMMIT message is sent in a round - faulty process
      - if PRECOMMIT message is sent without receiving *+2/3* voting-power equivalent appropriate PREVOTE messages - faulty process
      - if PREVOTE message is sent for the value V’ in round R’ and the PRECOMMIT message had been sent for the value V in round R by 
        the same process (R’ > R) and there are no *+2/3* voting-power equivalent PREVOTE(VR, V’) messages 
        *(VR ≥ 0 and VR > R and VR < R’)* as the justification for sending PREVOTE(R’, V’) - faulty process
      

At the end of the protocol, status of every process is determined (correct or faulty) and in case of faulty processes list of misbehavior are 
also computed. The proofs that this approach guarantee that only faulty processes are ever suspected can be found [here] (https://docs.google.com/document/d/11ZhMsCj3y7zIZz4udO9l25xqb0kl7gmWqNpGVRzOeyY/edit). 

Given the same vote sets as input, the protocol executed by monitor always deterministically output the same result, so it can be 
verified by any process which has access to the vote sets. As monitor is in this proposal trusted component, we need a mechanism 
that ensures that the output of the monitor procedure could be verifiable by every validator. 

Q: What is the most efficient way to satisfy this requirement? Can it be done in a way that does not require that validators 
have access to the vote sets?