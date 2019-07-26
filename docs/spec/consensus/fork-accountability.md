# Fork accountability

This document presents algorithms which are used to detect faulty validators in case of fork (so they can be 
punished accordingly). Tendermint guarantees (agreement, validity and termination) hold only in case 
faulty validators have at most 1/3 of voting power in the current validator set. In case this assumption
does not hold, Tendermint agreement might be violated and we then have a fork. We say that fork is a case in
which there are two commits for different block at the same height of the blockchain. Algorithms that we present
in this document ensures that in those cases we are able to detect faulty validators (and not mistakenly accuse
correct validators), and incentivize therefore validators to behave according to the protocol specification.

## Fork scenarios

There are several scenarios in which fork might happen:

1. Equivocation: faulty validators sign multiple vote messages (prevote and/or precommit) for different values during
   the same round r at a given height h. 

2. Amnesia: faulty validators precommit some value v in round r (value v is locked in round r) and then prevote for different value v' in higher round r' > r without previously correctly unlocking value v. In this case faulty processes "forget" that they have locked value v and prevote some other value in the following rounds.

3. Back to the past: faulty validators precommit some value v in round r from the past in which they haven't voted before. In this case some other value has been decided in a round r' > r, but there are some precommit messages signed by (correct) processes for value v in the round r. 

4. Fantom validators: faulty validators vote (sign prevote and precommit messages) in heights in which they are not part of the validator sets (at the main chain).

5. Lunatic validator: faulty validator that sign vote messages to support arbitrary application state. 

## Types of victims

We consider three types of potential attack victims: 

- full node
- lite client with sequential header verification (LCS) and 
- lite client with bisection based header verification (LCB).

In case of full node, attack surface is constrained to the blockchain as a correct full node executes 
all transactions so it can verify if computed application state diverge from the proposed state. Therefore,
full node can be tricked to follow forked chain by committing different set of transactions to a blockchain.

In case of lite clients, attack surface is bigger as lite client only verified headers without executing transactions.
Therefore, faulty nodes can in some cases attack lite clients by arbitrarily changing state if there is a sufficient
number of voting power that can commit such state. 

Q: Is there an important difference between LCS and LCB, i.e., are there some attacks that are possible with LCB but not
with LCS?

### Equivocation based attacks

In case of equivocation based attacks, faulty validators sign multiple votes (prevote and/or precommit) in the same
round of some height. This attack can be executed on both full nodes and lite clients. It requires at least 1/3+ of voting power to be executed. 

Example scenario: let's consider faulty proposer that propose block A to 1/3 of voting power equivalent correct processes (let's denote this set with C1) and proposes block B to 1/3 of voting power equivalent correct processes
(lets denote this set with C2). Validators from the set C1 and C2 prevote for A and B respectively. Faulty validators
that have +1/3 of voting power prevote both for A and B. Therefore correct validators from set C1 and C2 will observe 
+2/3 of prevotes for values A and B respectively and precommit for values A and B (respectively). Faulty processes 
precomit for both values A and B, so we have commits for both values A and B. This is the attack on the full node level,
that of course extends also to the lite clients, but it is easily detected at the main chain level as we assume that 
correct nodes are able to eventually talk to each other. Creating an evidence of misbehavior is simple in this case
as we have multiple messages signed by the same processes for different values in the same round. 

In case of lite clients, a variant of this attack is possible where faulty activity is visible only to a lite clients.
In this case +2/3 of voting power is needed. Note that once equivocation is used to attack lite client it opens space
for different kind of attacks as application state can be diverged in any direction. For example it can modify validator set such that it contains only validators that don't have any stake bonded. In order to detect such (equivocation based attack) lite client need to cross check it's state with some correct validator (or to obtain
hash of the state from the main chain using out of band channels). In case of equivocation attack client would 
be able to create evidence of misbehavior but this would require to pull potentially lot of data from 
correct full nodes. Maybe we need to figure out different architecture where client that is attacked with push
all it's data for the current unbonding period to a correct node that will inspect this data and submit corresponding
evidence. There are also architectures that assumes a special role (sometimes called fisherman) whose goal is to collect
as much as possible useful data from the network, to do analysis and create evidence transactions. That functionality
is outside the scope of this document.

Note that after lite client is fooled by a fork, that normally means that an attacker can change in an arbitrarily
way application state and validator set. Difference between LCS and LCB might only be in the amount of voting power
needed to convince lite client about arbitrary state. In case of LCB where security threshold is at minimum (+1/3 of 
voting power of trusted validator set), an attacker can arbitrarily modify application state with +1/3 of voting power,
while in case of LCS it requires +2/3 of voting power. 

### Amnesia based attacks

In case of amnesia faulty validator lock some value v in some round r, and then vote for different value v' in higher
rounds without correctly unlocking value v. This attack can be used both on full nodes and lite clients. 

Example scenario: +2/3 of voting power commit a value A in round r. Remaining validators haven't locked any value.
+1/3 of faulty validators ignore the fact that they locked a value A and propose a different block B in the following
rounds r' > r. As 1/3 of correct processes haven't locked any value they would accept proposal for B and they commit
a block B. Note that in this case +1/3 of faulty validators does not need to commit an equivocation as they only vote once per round. Detecting faulty validators in case of such attack require fork accountability mechanism described in 
this document https://docs.google.com/document/d/11ZhMsCj3y7zIZz4udO9l25xqb0kl7gmWqNpGVRzOeyY/edit?usp=sharing. 

If a lite client is attacked using this attack with +1/3 of voting power, attacker can not arbitrarily change application state. In case there is an attack with +2/3 of voting power, an attacker can arbitrarily change application state. Note that in this case at least +1/3 of faulty validators locked value A in round r, so if they sign different value in follow up rounds this will be detectable by the the fork accountability mechanisms. The remaining 1/3 of faulty processes might not locked value A in round r so they can't be detected using this mechanism. Only in case they signed something which conflicts with the application this can be used against them. Otherwise they don't do anything incorrect. Note that this case is not covered by the report https://docs.google.com/document/d/11ZhMsCj3y7zIZz4udO9l25xqb0kl7gmWqNpGVRzOeyY/edit?usp=sharing as it only assumes at most 2/3 of faulty validators.

Q: do we need to define a special kind of attack for the case where a validator sign arbitrarily state? It seems that
detecting such attack requires different mechanism that would require as an evidence a sequence of blocks that lead to
that state. This might be very tricky to implement. 

### Back to the past

In this kind of attacks faulty validators take advantage of the fact that he hasn't signed messages in some 
of the past rounds. Due to asynchronous network in which Tendermint operate we cannot easily differentiate between
such an attack and delayed message. This kind of attack can be used at both full nodes and lite clients.

Example scenario: in a round r of height h we have 1/3 of correct validators precommitting a value A, 1/3 of correct
validators precomit nil, 1/3 of faulty validators don't send any message and 1 faulty validator precommit nil.
In some round r' > r, +1/3 of faulty validators together with 1/3 of correct that haven't locked any value commit some other value B. +1/3 of faulty validators now "go back to the past" and sign precomit message for value A in round r.
Together with precomit messages of correct processes this for a commit for value A. Note that only a faulty validator
that previously precomit nil did equivocation, while other 1/3 of faulty validators actually executed an
attack that has exactly the same sequence of messages as part of amnesia attack. Detecting this kind of attack boil down to mechanisms for equivocation and amnesia. 

Q: should we keep this as a separate kind of attacks? It seems that equivocation, amnesia and fantom validators 
are the only kind of attack we need to support and this gives us security also in other cases. This would not
be surprising as equivocation and amnesia are attacks that followed from the protocol and fantom attack is 
not really an attack to Tendermint but more to the Proof of stake module. 

### Fantom validators

In case of fantom validators, processes that are not part of the current validator set but are still bonded (
as attack happen during their unbonding period) can be part of the attack by signing vote messages. This attack 
can be executed against both full nodes and lite clients. 

Example scenario: a lite client has a trust in a header for height h (and the corresponding validator set VS1). 
As part of bisection header verification it verifies header at height h+k with new validator set VS2. 
Some of the validators from VS2 signed the header at height h+k that is different from the corresponding header
at the main chain. Detecting these kind of attacks is easy as it just requires verifying if processes are signing messages in heights in which they are not part of the validator set.  

Note that we can have fantom validators based attacks as a follow up of equivocation or amnesia based attack where 
forked state contains validators that are not part of the validator set at the main chain. In this case they keep
signing messages contributed to a forked chain although they are not part of the validator set on the main chain.
This attack can also be used to attack full node during a period of time he is eclipsed. 

### Lunatic validator

Lunatic validator agrees to sign commit messages for arbitrary application state. It is used to attack lite clients. 
Note that detecting this behavior require application knowledge. Detecting this behavior can probably be done by
referring to the block before the one in which height happen.  

Q: can we say that in this case a validator ignore to check if proposed value is valid before voting for it?









