# Interview Transcript with Tendermint core researcher, Zarko Milosevic, by Chjango

**ZM**: Regarding leader election, it's round robin, but a weighted one. You
take into account the amount of bonded tokens. Depending on how much weight
they have of voting power, they would be elected more frequently. So we do
rotate, but just the guys who are having more voting power would be elected
more frequently. We are having 4 validators, and 1 of them have 2 times more
voting power, they have 2 times more elected as a leader.

**CC**: 2x more absolute voting power or probabilistic voting power?

**ZM**: It's actually very deterministic. It's not probabilistic at all. See
[Tendermint proposal election specification][1]. In Tendermint, there is no
pseudorandom leader election. It's a deterministic protocol. So leader election
is a built-in function in the code, so you know exactly—depending on the voting
power in the validator set, you'd know who exactly would be the leader in round
x, x + 1, and so on. There is nothing random there; we are not trying to hide
who would be the leader. It's really well known. It's just that there is a
function, it's a mathematical function, and it's just basically—it's kind of an
implementation detail—it starts from the voting power, and when you are
elected, you get decreased some number, and in each round you keep increasing
depending on your voting power, so that you are elected after k rounds again.
But knowing the validator set and the voting power, it's very simple function,
you can calculate yourself to know exactly who would be next. For each round,
this function will return you the leader for that round. In every round, we do
this computation. It's all part of the same flow. It enforces the properties
which are: proportional to your voting power, you will be elected, and we keep
changing the leaders. So it can't happen to have one guy being more elected
than other guys, if they have the same voting power. So one time it will be guy
B, and next time it will be guy B1. So it's not random.

**CC**: Assuming the validator set remains unchanged for a month, then if you
run this function, are you able to know exactly who is going to go for that
entire month?

**ZM**: Yes.

**CC**: What're the attack scenarios for this?

**ZM**: This is something which is easily attacked by people who argue that
Tendermint is not decentralized enough. They say that by knowing the leader,
you can DDoS the leader. And by DDoSing the leader, you are able to stop the
progress. Because it's true. If you would be able to DDoS the leader, the
leader would not be able to propose and then effectively will not be making
progress. How we are addressing this thing is Sentry Architecture. So the
validator—or at least a proper validator—will never be available. You don't
know the ip address of the validator. You are never able to open the connection
to the validator. So validator is spawning sentry nodes and this is the single
administration domain and there is only connection from validator in the sense
of sentry nodes. And ip address of validator is not shared in the p2p network.
It’s completely private. This is our answer to DDoS attack. By playing clever
at this sentry node architecture and spawning additional sentry nodes in case,
for ex your sentry nodes are being DDoS’d, bc your sentry nodes are public,
then you will be able to connect to sentry nodes. this is where we will expect
the validator to be clever enough that so that in case they are DDoS’d at the
sentry level, they will spawn a different sentry node and then you communicate
through them. We are in a sense pushing the responsibility on the validator.

**CC**: So if I understand this correctly, the public identity of the validator
doesn’t even matter because that entity can obfuscate where their real full
nodes reside via a proxy through this sentry architecture.

**ZM**: Exactly. So you do know what is the address or identity of the validator
but you don’t know the network address of it; you’re not able to attack it
because you don’t know where they are. They are completely obfuscated by the
sentry nodes. There is now, if you really want to figure out….There is the
Tendermint protocol, the structure of the protocol is not fully decentralized
in the sense that the flow of information is going from the round proposer, or
the round coordinator, to other nodes, and then after they receive this it’s
basically like [inaudible: “O to 1”]. So by tracking where this information is
coming from, you might be able to identify who are the sentry nodes behind it.
So if you are doing some network analysis, you might be able to deduce
something. If the thing would be completely stuck, where the validator would
never change their sentry nodes or ip addresses of sentry nodes, it could be
possible to deduce something. This is where economic game comes into play. We
are doing an economics game there. We say that it’s a validator business. If
they are not able to hide themselves well enough, they’ll be DDoS’d and they
will be kicked out of the active validator set. So it’s in their interest.

[Proposer Selection Procedure in Tendermint][1]. This is how it should work no
matter what implementation.

**CC**: Going back to the proposer, lets say the validator does get DDoS’d, then
the proposer goes down. What happens?

**ZM**: How the proposal mechanism works—there’s nothing special there—it goes
through a sequence of rounds. Normal execution of Tendermint is that for each
height, we are going through a sequence of rounds, starting from round 0, and
then we are incrementing through the rounds. The nodes are moving through the
rounds as part of normal procedure until they decide to commit. In case you
have one proposer—the proposer of a single round—being DDoS’d, we will probably
not decide in that round, because he will not be able to send his proposal. So
we will go to the next round, and hopefully the next proposer will be able to
communicate with the validators and then we’ll decide in the next round.

**CC**: Are there timeouts between one round to another, if a round gets
skipped?

**ZM**: There are timeouts. It’s a bit more complex. I think we have 5 timeouts.
We may be able to simplify this a bit. What is important to understand is: The
only condition which needs to be satisfied so we can go to the next round is
that your validator is able to communicate with more than 2/3rds of voting
power. To be able to move to the next round, you need to receive more than
2/3rd of voting power equivalent of pre-commit messages.

We have two kinds of messages: 1) Proposal: Where the current round proposer is
suggesting how the next block should look like. This is first one. Every round
starts with proposer sending a proposal. And then there are two more rounds of
voting, where the validator is trying to agree whether they will commit the
proposal or not. And the first of such vote messages is called `pre-vote` and
the second one is `pre-commit`. Now, to be able to move between steps, between
a `pre-vote` and `pre-commit` step, you need to receive enough number of
messages where if message is sent by validator A, then also this message has a
weight, or voting power which is equal to the voting power of the validator who
sent this message. Before you receive more than 2/3 of voting power messages, you are not
able to move to the higher round. Only when you receive more than 2/3 of
messages, you actually start the timeout. The timeout is happening only after
you receive enough messages. And it happens because of the asynchrony of the
message communication so you give more time to guys with this timeout to
receive some messages which are maybe delayed.

**CC**: In this way that you just described via the whole network gossiping
before we commit a block, that is what makes Tendermint BFT deterministic in a
partially synchronous setting vs Bitcoin which has synchrony assumptions
whereby blocks are first mined and then gossiped to the network.

**ZM**: It's true that in Bitcoin, this is where the synchrony assumption comes
to play because if they're not able to communicate timely, they are not able to
converge to a single longest chain. Why are they not able to decrease timeout
in Bitcoin? Because if they would decrease, there would be so many forks that
they won't be able to converge to a single chain. By increasing this
complexity and the block time, they're able to have not so many forks. This is
effectively the timing assumption—the block duration in a sense because it's
enough time so that the decided block is propagated through the network before
someone else start deciding on the same block and creating forks. It's very
different from the consensus algorithms in a distributed computing setup where
Tendermint fits. In Tendermint, where we talk about the timing dependency, they
are really part of this 3-communication step protocol I just explained. We have
the following assumption: If the good guys are not able to communicate timely
and reliably without having message loss within a round, the Tendermint will
not make progress—it will not be making blocks. So if you are in a completely
asynchronous network where messages get lost or delayed unpredictably,
Tendermint will not make progress, it will not create forks, but it will not
decide, it will not tell you what is the next block. For termination, it's a
liveness property of consensus. It's a guarantee to decide. We do need timing
assumptions. Within a round, correct validators are able to communicate to each
other the consensus messages, not the transactions, but consensus messages.
They need to communicate in a timely and reliable fashion. But this doesn't
need to hold forever. It's just that what we are assuming when we say it's a
partially synchronous system, we assume that the system will be going through a
period of asynchrony, where we don't have this guarantee; the messages will be
delayed or some will be lost and then will not make progress for some period of
time, or we're not guaranteed to make progress. And the period of synchrony
where these guarantees hold. And if we think about internet, internet is best
described using such a model. Sometimes when we send a message to SF to
Belgrade, it takes 100 ms, sometimes it takes 300 ms, sometimes it takes 1 s.
But in most cases, it takes 100 ms or less than this.

There is one thing which would be really nice if you understand it. In a global
wide area network, we can't make assumption on the communication unless we are
very conservative about this. If you want to be very fast, then we can't make
assumption and say we'll be for sure communicating with 1 ms communication
delay. Because of the complexity and various congestion issues on the network,
it might happen that during a short period of time, this doesn't hold. If this
doesn't hold and you depend on this for correctness of your protocol, you will
have a fork. So the partially synchronous protocol, most of them like
Tendermint, they don't depend on the timing assumption from the internet for
correctness. This is where we state: safety always. So we never make a fork no
matter how bad our estimates about the internet communication delays are. We'll
never make a fork, but we do make some assumptions, and these assumptions are
built-in our timeouts in our protocol which are actually adaptive. So we are
adapting to the current condition and this is where we're saying...We do assume
some properties, or some communication delays, to eventually hold on the
network. During this period, we guarantee that we will be deciding and
committing blocks. And we will be doing this very fast. We will be basically on
the speed of the current network.

**CC**: We make liveness assumptions based on the integrity of the validator
businesses, assuming they're up and running fine.

**ZM**: This is where we are saying, the protocol will be live if we have at
most 1/3, or a bit less than 1/3, of faulty validators. Which means that all
other guys should be online and available. This is also for liveness. This is
related to the condition that we are not able to make progress in rounds if we
don't receive enough messages. If half of our voting power, or half of our
validators are down, we don't have enough messages, so the protocol is
completely blocked. It doesn't make progress in a round, which means it's not
able to be signed. So it's completely critical for Tendermint that we make
progress in rounds. It's like breathing. Tendermint is breathing. If there is
no progress, it's dead; it's blocked, we're not able to breathe, that's why
we're not able to make progress.

**CC**: How does Tendermint compare to other consensus algos?

**ZM**: Tendermint is a very interesting protocol. From an academic point of
view, I'm convinced that there is value there. Hopefully, we prove it by
publishing it on some good conference. What is novel is, if we compare first
Tendermint to this existing BFT problem, it's a continuation of academic
research on BFT consensus. What is novel in Tendermint is that it somehow
merges consensus protocol with gossip. This is completely novel idea.
Originally, in BFT, people were assuming the single administration domain,
small number of nodes, local area network, 4-7 nodes max. If you look at the
research paper, 99% of them have this kind of setup. Wide area was studied but
there is significantly less work in wide area networks. No one studied how to
scale those protocols to hundreds or thousands of nodes before blockchain. It
was always a single administration domain. So in Tendermint now, you are able
to reach consensus among different administration domains which are potentially
hundreds of them in wide area network. The system model is potentially harder
because we have more nodes and wide area network. The second thing is that:
normally, in bft protocols, the protocol itself are normally designed in a way
that has two phases, or two parts. The one which is called normal case, which
is normally quite simple, in this normal case. In spite of some failures, which
are part of the normal execution of the protocol, like for example leader
crashes or leader being DDoS'd, they need to go through a quite complex
protocol, which is like being called view change or leader election or
whatever. These two parts of the same protocol are having quite different
complexity. And most of the people only understand this normal case. In
Tendermint, there is no this difference. We have only one protocol, there are
not two protocols. It's always the same steps and they are much closer to the
normal case than this complex view change protocol.

_This is a bit too technical but this is on a high level things to remember,
that: The system it addresses it's harder than the others and the algorithm
complexity in Tendermint is simpler._ The initial goal of Jae and Bucky which
is inspired by Raft, is that it's simpler so normal engineers could understand.

**CC**: Can you expand on the termination requirement?

_Important point about Liveness in Tendermint_

**ZM**: In Tendermint, we are saying, for termination, we are making assumption
that the system is partially synchronous. And in a partially synchronous system
model, we are able to mathematically prove that the protocol will make
decisions; it will decide.

**CC**: What is a persistent peer?

**ZM**: It's a list of peer identities, which you will try to establish
connection to them, in case connection is broken, Tendermint will automatically
try to reestablish connection. These are important peers, you will really try
persistently to establish connection to them. For other peers, you just drop it
and try from your address book to connect to someone else. The address book is a
list of peers which you discover that they exist, because we are talking about a
very dynamic network—so the nodes are coming and going away—and the gossiping
protocol is discovering new nodes and gossiping them around. So every node will
keep the list of new nodes it discovers, and when you need to establish
connection to a peer, you'll look to address book and get some addresses from
there. There's categorization/ranking of nodes there.

[1]: https://github.com/tendermint/tendermint/blob/master/docs/spec/reactors/consensus/proposer-selection.md
