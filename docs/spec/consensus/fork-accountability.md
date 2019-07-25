# Fork accountability

This document presents algorithms which are used to detect faulty validators in case of fork (so they can be 
punished accordingly). Tendermint guarantees (agreement, validity and termination) hold only in case 
faulty validators have at most 1/3 of voting power in the current validator set. In case this assumption
does not hold, Tendermint agreement might be violated and we then have a fork. We say that fork is a case in
which there are two commits for different block at the same height of the blockchain. Algorithms that we present
in this document ensures that in those cases we are able to detect faulty validators (and not mistakenly accuse
correct validators), and incentivize therefore validators to behave according to the protocol specification.

There are several scenarios in which fork might happen:

1. Equivocation: at least 1/3+ faulty validators where they prevote and precommit for different values during
   the same round r at a given height h. More precisely, faulty validators in this case sign prevote and/or precommit messages for values A and B during the same round.  

2. Amnesia: at least 1/3+ faulty validators precommit some value v in round r (value v is locked in round r) and then prevote for different value v' in higher round r' > r without previously unlocking value v correctly. In this case faulty processes "forget" that they have locked value v and prevote some other value.

3. Fantom validators: faulty validators vote (sign prevote and precommit messages) in heights in which they are not part of the validator sets.

4. Back to the past: faulty validators precommit some value v in round r from the past in which they haven't voted before. In this case some other value has been decided in a round r' > r, but there are some precommit messages signed by (correct) processes for value v in the round r. 

