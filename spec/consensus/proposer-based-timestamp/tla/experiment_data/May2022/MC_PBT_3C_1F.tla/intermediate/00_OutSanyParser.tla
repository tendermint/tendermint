--------------------------- MODULE 00_OutSanyParser ---------------------------

EXTENDS Integers, Sequences, FiniteSets, TLC, Apalache

CONSTANT
  (*
    @type:  ROUND -> PROCESS;
  *)
  Proposer

VARIABLE
  (*
    a process step
    @type:  PROCESS -> DECISION;
  *)
  decision

VARIABLE
  (*
    PREVOTE messages broadcast in the system, Rounds -> Messages
    @type:  ROUND -> Set(PREMESSAGE);
  *)
  msgsPrecommit

VARIABLE
  (*
    @type:  <<ROUND, PROCESS>> -> TIME;
  *)
  beginRound

VARIABLE
  (*
    PROPOSE messages broadcast in the system, Rounds -> Messages
    @type:  ROUND -> Set(PREMESSAGE);
  *)
  msgsPrevote

VARIABLE
  (*
    the messages that were used by the correct processes to make transitions
    @type:  ACTION;
  *)
  action

VARIABLE
  (*
    a locked value
    @type:  PROCESS -> ROUND;
  *)
  lockedRound

VARIABLE
  (*
    @type:  ROUND -> Set(PROPMESSAGE);
  *)
  msgsPropose

VARIABLE
  (*
    a valid value
    @type:  PROCESS -> ROUND;
  *)
  validRound

VARIABLE
  (*
    a process round number
    @type:  PROCESS -> STEP;
  *)
  step

VARIABLE
  (*
    process decision
    @type:  PROCESS -> VALUE;
  *)
  lockedValue

VARIABLE
  (*
    a locked round
    @type:  PROCESS -> PROPOSAL;
  *)
  validValue

VARIABLE
  (*
    a process local clock: Corr -> Ticks
    @type:  TIME;
  *)
  realTime

VARIABLE
  (*
    @type:  PROCESS -> ROUND;
  *)
  round

VARIABLE
  (*
    PRECOMMIT messages broadcast in the system, Rounds -> Messages
    @type:  Set(MESSAGE);
  *)
  evidence

VARIABLE
  (*
    we use this variable to see which action was taken
    @type:  <<ROUND,PROCESS>> -> TIME;
  *)
  proposalReceptionTime

VARIABLE
  (*
    @type:  PROCESS -> TIME;
  *)
  localClock

(*
  the set of clock ticks
  @type:  ROUND;
*)
NilRound == -1

(*
  @typeAlias:  PROCESS = Str;
  @typeAlias:  VALUE = Str;
  @typeAlias:  STEP = Str;
  @typeAlias:  ROUND = Int;
  @typeAlias:  ACTION = Str;
  @typeAlias:  TRACE = Seq(Str);
  @typeAlias:  TIME = Int;
  @typeAlias:  PROPOSAL = <<VALUE, TIME, ROUND>>;
  @typeAlias:  DECISION = <<PROPOSAL, ROUND>>;
  @typeAlias:  PROPMESSAGE = [ type: STEP, src: PROCESS, round: ROUND, proposal: PROPOSAL, validRound: ROUND ];
  @typeAlias:  PREMESSAGE = [ type: STEP, src: PROCESS, round: ROUND, id: PROPOSAL ];
  @typeAlias:  MESSAGE = [ type: STEP, src: PROCESS, round: ROUND, proposal: PROPOSAL, validRound: ROUND, id: PROPOSAL ];
*)
TypeAliases == TRUE

(*
  @type:  Set(VALUE);
*)
Values == { "v0", "v1" } \union {"v2"}

(*
  the set of potential rounds
  @type:  Set(TIME);
*)
Timestamps == 0 .. 7

(*
  @type:  (PROPMESSAGE) => VALUE;
*)
MessageValue(msg) == msg["proposal"][1]

(*
  a quorum when having N > 3 * T
  @type:  (TIME, TIME) => TIME;
*)
Min2(a, b) == IF a <= b THEN a ELSE b

(*
  [PBTS-DECISION-ROUND.0]
  @type:  (PROPOSAL, ROUND) => DECISION;
*)
Decision(p, r) == <<p, r>>

(*
  Time validity check. If we want MaxTimestamp = \infty, set ValidTime(t) == TRUE
*)
ValidTime(t) == t < 7

(*
  ************************** DEFINITIONS ***********************************
  @type:  Set(PROCESS);
*)
AllProcs == { "c1", "c2", "c3" } \union {"f4"}

(*
  @type:  (PROPMESSAGE) => ROUND;
*)
MessageRound(msg) == msg["proposal"][3]

(*
  Min(S) == CHOOSE x \in S : \A y \in S : x <= y
  @type:  (TIME, TIME) => TIME;
*)
Max2(a, b) == IF a >= b THEN a ELSE b

(*
  *
 * The subsequence of s consisting of all elements s[i] such that
 * Test(s[i]) is true.
 * Apalache uses the TLA+ definition via FoldSeq.
 *
 *
  @type:  (Seq(a), a => Bool) => Seq(a);
*)
SelectSeq(__s, __Test(_)) ==
  LET __AppendIfTest(__res, __e) ==
    IF __Test(__e) THEN Append(__res, __e) ELSE __res
  IN
  ApaFoldSeqLeft(__AppendIfTest, <<>>, __s)

(*
  @type:  (PROPMESSAGE) => TIME;
*)
MessageTime(msg) == msg["proposal"][2]

(*
  a special value to denote a nil round, outside of Rounds
  @type:  TIME;
*)
NilTimestamp == -1

(*
  @type:  (TIME, TIME) => Bool;
*)
IsTimely(processTime, messageTime) ==
  processTime >= messageTime - 2 /\ processTime <= (messageTime + 2) + 2

(*
  Max(S) == CHOOSE x \in S : \A y \in S : y <= x
  @type:  (Set(MESSAGE)) => Int;
*)
Card(S) ==
  LET (*@type:  (Int, MESSAGE) => Int; *) PlusOne(i, m) == i + 1 IN
  ApaFoldSet(PlusOne, 0, S)

(*
  *
 * As TLA+ is untyped, one can use function- and sequence-specific operators
 * interchangeably. However, to maintain correctness w.r.t. our type-system,
 * an explicit cast is needed when using functions as sequences.
 * FunAsSeq reinterprets a function over integers as a sequence.
 *
 * The parameters have the following meaning:
 *
 *  - fn is the function from 1..len that should be interpreted as a sequence.
 *  - len is the length of the sequence, len = Cardinality(DOMAIN fn),
 *    len may be a variable, a computable expression, etc.
 *  - capacity is a static upper bound on the length, that is, len <= capacity.
 *
 *
  @type:  ((Int -> a), Int, Int) => Seq(a);
*)
FunAsSeq(__fn, __len, __capacity) ==
  LET __FunAsSeq_elem_ctor(__i) == __fn[__i] IN
  SubSeq(MkSeq(__capacity, __FunAsSeq_elem_ctor), 1, __len)

(*
  the set of all processes
  @type:  Set(ROUND);
*)
Rounds == 0 .. 3

(*
  at least one process is not faulty
  @type:  Int;
*)
THRESHOLD2 == 2 * 1 + 1

(*
  a value hash is modeled as identity
  @type:  (t) => t;
*)
Id(v) == v

(*
  the two thresholds that are used in the algorithm
  @type:  Int;
*)
THRESHOLD1 == 1 + 1

(*
  [PBTS-PROPOSE.0]
  @type:  (VALUE, TIME, ROUND) => PROPOSAL;
*)
Proposal(v, t, r) == <<v, t, r>>

(*
  the set of all values
  @type:  VALUE;
*)
NilValue == "None"

(*
  @type:  (Set(TIME)) => TIME;
*)
Min(S) == ApaFoldSet(Min2, 7, S)

ArbitraryProposer == Proposer \in [(Rounds) -> (AllProcs)]

CorrectProposer == Proposer \in [(Rounds) -> { "c1", "c2", "c3" }]

(*
  a reference Newtonian real time
*)
temporalVars == <<localClock, realTime>>

(*
  @type:  (ROUND -> Set(MESSAGE), Set(MESSAGE)) => Bool;
*)
BenignAndSubset(msgfun, set) ==
  (\A r \in Rounds:
      msgfun[r] \subseteq set /\ (\A m \in msgfun[r]: r = m["round"]))

ValidProposals == { "v0", "v1" } \X (2 .. 7) \X Rounds

(*
  *********************** MESSAGE PASSING *******************************
  @type:  (PROCESS, ROUND, PROPOSAL, ROUND) => Bool;
*)
BroadcastProposal(pSrc, pRound, pProposal, pValidRound) ==
  LET (*
    @type:  PROPMESSAGE;
  *)
  newMsg ==
    [type |-> "PROPOSAL",
      src |-> pSrc,
      round |-> pRound,
      proposal |-> pProposal,
      validRound |-> pValidRound]
  IN
  msgsPropose'
      = [ msgsPropose EXCEPT ![pRound] = msgsPropose[pRound] \union {(newMsg)} ]

CyclicalProposer ==
  LET ProcOrder ==
    LET App(s, e) == Append(s, e) IN ApaFoldSet(App, <<>>, (AllProcs))
  IN
  Proposer = [ r \in Rounds |-> (ProcOrder)[(1 + (r % 4))] ]

IsFirstProposedInRound(prop, src, r) ==
  \E msg \in msgsPropose[r]:
    msg["proposal"] = prop /\ msg["src"] = src /\ msg["validRound"] = NilRound

(*
  used to keep track when a process receives a message

 Action is excluded from the tuple, because it always changes
*)
bookkeepingVars ==
  <<msgsPropose, msgsPrevote, msgsPrecommit, evidence, proposalReceptionTime>>

(*
  @type:  PROPOSAL;
*)
NilProposal == <<(NilValue), (NilTimestamp), (NilRound)>>

(*
  **************************** TIME *************************************
 [PBTS-CLOCK-PRECISION.0]
  @type:  Bool;
*)
SynchronizedLocalClocks ==
  \A p \in { "c1", "c2", "c3" }:
    \A q \in { "c1", "c2", "c3" }:
      p /= q
        => (localClock[p] >= localClock[q] /\ localClock[p] - localClock[q] < 2)
          \/ (localClock[p] < localClock[q] /\ localClock[q] - localClock[p] < 2)

(*
  @type:  (Set(TIME)) => TIME;
*)
Max(S) == ApaFoldSet(Max2, (NilTimestamp), S)

(*
  @type:  (PROCESS, ROUND, PROPOSAL) => Bool;
*)
BroadcastPrevote(pSrc, pRound, pId) ==
  LET (*
    @type:  PREMESSAGE;
  *)
  newMsg == [type |-> "PREVOTE", src |-> pSrc, round |-> pRound, id |-> pId]
  IN
  msgsPrevote'
      = [ msgsPrevote EXCEPT ![pRound] = msgsPrevote[pRound] \union {(newMsg)} ]

(*
  *************** MESSAGE PROCESSING TRANSITIONS ************************
 lines 12-13
  @type:  (PROCESS, ROUND) => Bool;
*)
StartRound(p, r) ==
  step[p] /= "DECIDED"
    /\ round' = [ round EXCEPT ![p] = r ]
    /\ step' = [ step EXCEPT ![p] = "PROPOSE" ]
    /\ beginRound' = [ beginRound EXCEPT ![<<r, p>>] = localClock[p] ]

(*
  [PBTS-INV-MONOTONICITY.0]

 TODO: we would need to compare timestamps of blocks from different height

 [PBTS-INV-TIMELY.0]
*)
ConsensusTimeValid ==
  \A p \in { "c1", "c2", "c3" }:
    \E v \in { "v0", "v1" }:
      \E t \in Timestamps:
        \E pr \in Rounds:
          \E dr \in Rounds:
            decision[p] = Decision((Proposal(v, t, pr)), dr)
              => (\E q \in { "c1", "c2", "c3" }:
                LET propRecvTime == proposalReceptionTime[pr, q] IN
                beginRound[pr, q] <= propRecvTime
                  /\ beginRound[(pr + 1), q] >= propRecvTime
                  /\ IsTimely((propRecvTime), t))

(*
  @type:  (ROUND -> Set(MESSAGE)) => Bool;
*)
BenignRoundsInMessages(msgfun) ==
  \A r \in Rounds: \A m \in msgfun[r]: r = m["round"]

(*
  @type:  (PROCESS, ROUND, PROPOSAL) => Bool;
*)
BroadcastPrecommit(pSrc, pRound, pId) ==
  LET (*
    @type:  PREMESSAGE;
  *)
  newMsg == [type |-> "PRECOMMIT", src |-> pSrc, round |-> pRound, id |-> pId]
  IN
  msgsPrecommit'
      = [
        msgsPrecommit EXCEPT
          ![pRound] = msgsPrecommit[pRound] \union {(newMsg)}
      ]

(*
  a special value to denote a nil timestamp, outside of Ticks
  @type:  Set(ROUND);
*)
RoundsOrNil == Rounds \union {(NilRound)}

(*
  a special value for a nil round, outside of Values
  @type:  Set(PROPOSAL);
*)
Proposals == Values \X Timestamps \X Rounds

(*
  a valid round
*)
coreVars ==
  <<round, step, decision, lockedValue, lockedRound, validValue, validRound>>

(*
  @type:  Set(VALUE);
*)
ValuesOrNil == Values \union {(NilValue)}

(*
  The validity predicate
  @type:  (PROPOSAL) => Bool;
*)
IsValid(p) == p \in ValidProposals

(*
  lines 44-46
  @type:  (PROCESS) => Bool;
*)
OnQuorumOfNilPrevotes(p) ==
  step[p] = "PREVOTE"
    /\ LET PV == { m \in msgsPrevote[round[p]]: m["id"] = Id((NilProposal)) } IN
    Cardinality((PV)) >= THRESHOLD2
      /\ evidence' = PV \union evidence
      /\ BroadcastPrecommit(p, round[p], (Id((NilProposal))))
      /\ step' = [ step EXCEPT ![p] = "PRECOMMIT" ]
      /\ UNCHANGED (<<(temporalVars), beginRound>>)
      /\ UNCHANGED (<<
        round, decision, lockedValue, lockedRound, validValue, validRound
      >>)
      /\ UNCHANGED (<<msgsPropose, msgsPrevote, proposalReceptionTime>>)
      /\ action' = "OnQuorumOfNilPrevotes"

(*
  ******************** PROTOCOL TRANSITIONS *****************************
 advance the global clock
  @type:  Bool;
*)
AdvanceRealTime ==
  ValidTime(realTime)
    /\ (\E t \in Timestamps:
      t > realTime
        /\ realTime' = t
        /\ localClock'
          = [ p \in { "c1", "c2", "c3" } |-> localClock[p] + t - realTime ])
    /\ UNCHANGED (<<(coreVars), (bookkeepingVars), beginRound>>)
    /\ action' = "AdvanceRealTime"

(*
  @type:  (ROUND) => Set(PREMESSAGE);
*)
FaultyPrevotes(r) == [type: {"PREVOTE"}, src: {"f4"}, round: {r}, id: Proposals]

lastBeginRound ==
  [ r \in Rounds |-> Max({ beginRound[r, p]: p \in { "c1", "c2", "c3" } }) ]

(*
  @type:  Set(PROPMESSAGE);
*)
AllFaultyProposals ==
  [type: {"PROPOSAL"},
    src: {"f4"},
    round: Rounds,
    proposal: Proposals,
    validRound: RoundsOrNil]

(*
  lines 14-19, a proposal may be sent later
  @type:  (PROCESS) => Bool;
*)
InsertProposal(p) ==
  LET r == round[p] IN
  p = Proposer[(r)]
    /\ step[p] = "PROPOSE"
    /\ (\A m \in msgsPropose[(r)]: m["src"] /= p)
    /\ (\E v \in { "v0", "v1" }:
      LET proposal ==
        IF validValue[p] /= NilProposal
        THEN validValue[p]
        ELSE Proposal(v, localClock[p], (r))
      IN
      BroadcastProposal(p, (r), (proposal), validRound[p]))
    /\ UNCHANGED (<<(temporalVars), (coreVars)>>)
    /\ UNCHANGED (<<
      msgsPrevote, msgsPrecommit, evidence, proposalReceptionTime
    >>)
    /\ UNCHANGED beginRound
    /\ action' = "InsertProposal"

firstBeginRound ==
  [ r \in Rounds |-> Min({ beginRound[r, p]: p \in { "c1", "c2", "c3" } }) ]

(*
  [PBTS-INV-VALID.0]
*)
ConsensusValidValue ==
  \A p \in { "c1", "c2", "c3" }:
    LET prop == decision[p][1] IN
    prop /= NilProposal => (prop)[1] \in { "v0", "v1" }

(*
  lines 36-46

 [PBTS-ALG-NEW-PREVOTE.0]
  @type:  (PROCESS) => Bool;
*)
UponProposalInPrevoteOrCommitAndPrevote(p) ==
  \E v \in { "v0", "v1" }:
    \E t \in Timestamps:
      \E vr \in RoundsOrNil:
        LET r == round[p] IN
        LET (*@type:  PROPOSAL; *) prop == Proposal(v, t, (r)) IN
        step[p] \in { "PREVOTE", "PRECOMMIT" }
          /\ LET (*
            @type:  PROPMESSAGE;
          *)
          msg ==
            [type |-> "PROPOSAL",
              src |-> Proposer[(r)],
              round |-> r,
              proposal |-> prop,
              validRound |-> vr]
          IN
          msg \in msgsPropose[(r)]
            /\ LET PV == { m \in msgsPrevote[(r)]: m["id"] = Id((prop)) } IN
            Cardinality((PV)) >= THRESHOLD2
              /\ evidence' = (PV \union {(msg)}) \union evidence
          /\ (IF step[p] = "PREVOTE"
          THEN lockedValue' = [ lockedValue EXCEPT ![p] = v ]
            /\ lockedRound' = [ lockedRound EXCEPT ![p] = r ]
            /\ BroadcastPrecommit(p, (r), (Id((prop))))
            /\ step' = [ step EXCEPT ![p] = "PRECOMMIT" ]
          ELSE UNCHANGED (<<lockedValue, lockedRound, msgsPrecommit, step>>))
          /\ validValue' = [ validValue EXCEPT ![p] = prop ]
          /\ validRound' = [ validRound EXCEPT ![p] = r ]
          /\ UNCHANGED (<<(temporalVars), beginRound>>)
          /\ UNCHANGED (<<round, decision>>)
          /\ UNCHANGED (<<msgsPropose, msgsPrevote, proposalReceptionTime>>)
          /\ action' = "UponProposalInPrevoteOrCommitAndPrevote"

(*
  the actions below are not essential for safety, but added for completeness

 lines 20-21 + 57-60
  @type:  (PROCESS) => Bool;
*)
OnTimeoutPropose(p) ==
  step[p] = "PROPOSE"
    /\ p /= Proposer[round[p]]
    /\ BroadcastPrevote(p, round[p], (NilProposal))
    /\ step' = [ step EXCEPT ![p] = "PREVOTE" ]
    /\ UNCHANGED (<<(temporalVars), beginRound>>)
    /\ UNCHANGED (<<
      round, decision, lockedValue, lockedRound, validValue, validRound
    >>)
    /\ UNCHANGED (<<
      msgsPropose, msgsPrecommit, evidence, proposalReceptionTime
    >>)
    /\ action' = "OnTimeoutPropose"

DisagreementOnValue ==
  \E p \in { "c1", "c2", "c3" }:
    \E q \in { "c1", "c2", "c3" }:
      \E p1 \in ValidProposals:
        \E p2 \in ValidProposals:
          \E r1 \in Rounds:
            \E r2 \in Rounds:
              p1 /= p2
                /\ decision[p] = Decision(p1, r1)
                /\ decision[q] = Decision(p2, r2)

(*
  @type:  Set(PREMESSAGE);
*)
AllFaultyPrecommits ==
  [type: {"PRECOMMIT"}, src: {"f4"}, round: Rounds, id: Proposals]

(*
  the minimum of the local clocks at the time any process entered a new round

 to see a type invariant, check TendermintAccInv3 
******************** PROTOCOL INITIALIZATION *****************************
  @type:  (ROUND) => Set(PROPMESSAGE);
*)
FaultyProposals(r) ==
  [type: {"PROPOSAL"},
    src: {"f4"},
    round: {r},
    proposal: Proposals,
    validRound: RoundsOrNil]

(*
  @type:  Set(PREMESSAGE);
*)
AllFaultyPrevotes ==
  [type: {"PREVOTE"}, src: {"f4"}, round: Rounds, id: Proposals]

(*
  lines 55-56
  @type:  (PROCESS) => Bool;
*)
OnRoundCatchup(p) ==
  \E r \in Rounds:
    r > round[p]
      /\ LET RoundMsgs ==
        (msgsPropose[r] \union msgsPrevote[r]) \union msgsPrecommit[r]
      IN
      \E MyEvidence \in SUBSET RoundMsgs:
        LET Faster == { m["src"]: m \in MyEvidence } IN
        Cardinality((Faster)) >= THRESHOLD1
          /\ evidence' = MyEvidence \union evidence
          /\ StartRound(p, r)
          /\ UNCHANGED temporalVars
          /\ UNCHANGED (<<
            decision, lockedValue, lockedRound, validValue, validRound
          >>)
          /\ UNCHANGED (<<
            msgsPropose, msgsPrevote, msgsPrecommit, proposalReceptionTime
          >>)
          /\ action' = "OnRoundCatchup"

(*
  @type:  (ROUND) => Set(PREMESSAGE);
*)
FaultyPrecommits(r) ==
  [type: {"PRECOMMIT"}, src: {"f4"}, round: {r}, id: Proposals]

(*
  @type:  Set(PROPMESSAGE);
*)
AllProposals ==
  [type: {"PROPOSAL"},
    src: AllProcs,
    round: Rounds,
    proposal: Proposals,
    validRound: RoundsOrNil]

(*
  lines 34-35 + lines 61-64 (onTimeoutPrevote)
  @type:  (PROCESS) => Bool;
*)
UponQuorumOfPrevotesAny(p) ==
  step[p] = "PREVOTE"
    /\ (\E MyEvidence \in SUBSET (msgsPrevote[round[p]]):
      LET Voters == { m["src"]: m \in MyEvidence } IN
      Cardinality((Voters)) >= THRESHOLD2
        /\ evidence' = MyEvidence \union evidence
        /\ BroadcastPrecommit(p, round[p], (NilProposal))
        /\ step' = [ step EXCEPT ![p] = "PRECOMMIT" ]
        /\ UNCHANGED (<<(temporalVars), beginRound>>)
        /\ UNCHANGED (<<
          round, decision, lockedValue, lockedRound, validValue, validRound
        >>)
        /\ UNCHANGED (<<msgsPropose, msgsPrevote, proposalReceptionTime>>)
        /\ action' = "UponQuorumOfPrevotesAny")

(*
  a new action used to register the proposal and note the reception time.

 [PBTS-RECEPTION-STEP.0]
  @type:  (PROCESS) => Bool;
*)
ReceiveProposal(p) ==
  \E v \in Values:
    \E t \in Timestamps:
      LET r == round[p] IN
        LET (*
          @type:  PROPMESSAGE;
        *)
        msg ==
          [type |-> "PROPOSAL",
            src |-> Proposer[round[p]],
            round |-> round[p],
            proposal |-> Proposal(v, t, (r)),
            validRound |-> NilRound]
        IN
        msg \in msgsPropose[round[p]]
          /\ proposalReceptionTime[(r), p] = NilTimestamp
          /\ proposalReceptionTime'
            = [ proposalReceptionTime EXCEPT ![<<(r), p>>] = localClock[p] ]
          /\ UNCHANGED (<<(temporalVars), (coreVars)>>)
          /\ UNCHANGED (<<msgsPropose, msgsPrevote, msgsPrecommit, evidence>>)
          /\ UNCHANGED beginRound
          /\ action' = "ReceiveProposal"

(*
  @type:  (ROUND) => Set(PROPMESSAGE);
*)
RoundProposals(r) ==
  [type: {"PROPOSAL"},
    src: AllProcs,
    round: {r},
    proposal: Proposals,
    validRound: RoundsOrNil]

(*
  @type:  DECISION;
*)
NilDecision == <<(NilProposal), (NilRound)>>

(*
  @type:  Set(DECISION);
*)
Decisions == Proposals \X Rounds

(*
  lines 47-48 + 65-67 (onTimeoutPrecommit)
  @type:  (PROCESS) => Bool;
*)
UponQuorumOfPrecommitsAny(p) ==
  (\E MyEvidence \in SUBSET (msgsPrecommit[round[p]]):
      LET Committers == { m["src"]: m \in MyEvidence } IN
      Cardinality((Committers)) >= THRESHOLD2
        /\ evidence' = MyEvidence \union evidence
        /\ round[p] + 1 \in Rounds
        /\ StartRound(p, (round[p] + 1))
        /\ UNCHANGED temporalVars
        /\ UNCHANGED (<<
          decision, lockedValue, lockedRound, validValue, validRound
        >>)
        /\ UNCHANGED (<<
          msgsPropose, msgsPrevote, msgsPrecommit, proposalReceptionTime
        >>)
        /\ action' = "UponQuorumOfPrecommitsAny")

(*
  run Apalache with --cinit=CInit
*)
CInit == ArbitraryProposer

(*
  lines 49-54        

 [PBTS-ALG-DECIDE.0]
  @type:  (PROCESS) => Bool;
*)
UponProposalInPrecommitNoDecision(p) ==
  decision[p] = NilDecision
    /\ (\E v \in { "v0", "v1" }:
      \E t \in Timestamps:
        \E r \in Rounds:
          \E pr \in Rounds:
            \E vr \in RoundsOrNil:
              LET (*@type:  PROPOSAL; *) prop == Proposal(v, t, pr) IN
              LET (*
                  @type:  PROPMESSAGE;
                *)
                msg ==
                  [type |-> "PROPOSAL",
                    src |-> Proposer[r],
                    round |-> r,
                    proposal |-> prop,
                    validRound |-> vr]
                IN
                msg \in msgsPropose[r]
                  /\ proposalReceptionTime[r, p] /= NilTimestamp
                  /\ LET PV == { m \in msgsPrecommit[r]: m["id"] = Id((prop)) }
                  IN
                  Cardinality((PV)) >= THRESHOLD2
                    /\ evidence' = (PV \union {(msg)}) \union evidence
                    /\ decision'
                      = [ decision EXCEPT ![p] = Decision((prop), r) ]
                    /\ step' = [ step EXCEPT ![p] = "DECIDED" ]
                    /\ UNCHANGED temporalVars
                    /\ UNCHANGED (<<
                      round, lockedValue, lockedRound, validValue, validRound
                    >>)
                    /\ UNCHANGED (<<
                      msgsPropose, msgsPrevote, msgsPrecommit, proposalReceptionTime
                    >>)
                    /\ UNCHANGED beginRound
                    /\ action' = "UponProposalInPrecommitNoDecision")

(*
  ************************** INVARIANTS ************************************
 [PBTS-INV-AGREEMENT.0]
*)
AgreementOnValue ==
  \A p \in { "c1", "c2", "c3" }:
    \A q \in { "c1", "c2", "c3" }:
      decision[p] /= NilDecision /\ decision[q] /= NilDecision
        => (\E v \in { "v0", "v1" }:
          \E t \in Timestamps:
            \E pr \in Rounds:
              \E r1 \in Rounds:
                \E r2 \in Rounds:
                  LET prop == Proposal(v, t, pr) IN
                  decision[p] = Decision((prop), r1)
                    /\ decision[q] = Decision((prop), r2))

(*
  lines 22-27
  @type:  (PROCESS) => Bool;
*)
UponProposalInPropose(p) ==
  \E v \in Values:
    \E t \in Timestamps:
      LET r == round[p] IN
      LET (*@type:  PROPOSAL; *) prop == Proposal(v, t, (r)) IN
      step[p] = "PROPOSE"
        /\ LET (*
          @type:  PROPMESSAGE;
        *)
        msg ==
          [type |-> "PROPOSAL",
            src |-> Proposer[(r)],
            round |-> r,
            proposal |-> prop,
            validRound |-> NilRound]
        IN
        evidence' = {(msg)} \union evidence
        /\ LET mid ==
          IF IsTimely(proposalReceptionTime[(r), p], t)
            /\ IsValid((prop))
            /\ (lockedRound[p] = NilRound \/ lockedValue[p] = v)
          THEN Id((prop))
          ELSE NilProposal
        IN
        BroadcastPrevote(p, (r), (mid))
        /\ step' = [ step EXCEPT ![p] = "PREVOTE" ]
        /\ UNCHANGED (<<(temporalVars), beginRound>>)
        /\ UNCHANGED (<<
          round, decision, lockedValue, lockedRound, validValue, validRound
        >>)
        /\ UNCHANGED (<<msgsPropose, msgsPrecommit, proposalReceptionTime>>)
        /\ action' = "UponProposalInPropose"

(*
  Faulty processes send messages
*)
FaultyBroadcast ==
  ~TRUE
    /\ action' = "FaultyBroadcast"
    /\ (\E r \in Rounds:
      (\E msgs \in SUBSET (FaultyProposals(r)):
          msgsPropose'
              = [ msgsPropose EXCEPT ![r] = msgsPropose[r] \union msgs ]
            /\ UNCHANGED (<<(coreVars), (temporalVars), beginRound>>)
            /\ UNCHANGED (<<
              msgsPrevote, msgsPrecommit, evidence, proposalReceptionTime
            >>))
        \/ (\E msgs \in SUBSET (FaultyPrevotes(r)):
          msgsPrevote'
              = [ msgsPrevote EXCEPT ![r] = msgsPrevote[r] \union msgs ]
            /\ UNCHANGED (<<(coreVars), (temporalVars), beginRound>>)
            /\ UNCHANGED (<<
              msgsPropose, msgsPrecommit, evidence, proposalReceptionTime
            >>))
        \/ (\E msgs \in SUBSET (FaultyPrecommits(r)):
          msgsPrecommit'
              = [ msgsPrecommit EXCEPT ![r] = msgsPrecommit[r] \union msgs ]
            /\ UNCHANGED (<<(coreVars), (temporalVars), beginRound>>)
            /\ UNCHANGED (<<
              msgsPropose, msgsPrevote, evidence, proposalReceptionTime
            >>)))

(*
  lines 28-33        

 [PBTS-ALG-OLD-PREVOTE.0]
  @type:  (PROCESS) => Bool;
*)
UponProposalInProposeAndPrevote(p) ==
  \E v \in Values:
    \E t \in Timestamps:
      \E vr \in Rounds:
        \E pr \in Rounds:
          LET r == round[p] IN
          LET (*@type:  PROPOSAL; *) prop == Proposal(v, t, pr) IN
          ((step[p] = "PROPOSE" /\ 0 <= vr) /\ vr < r)
            /\ pr <= vr
            /\ LET (*
              @type:  PROPMESSAGE;
            *)
            msg ==
              [type |-> "PROPOSAL",
                src |-> Proposer[(r)],
                round |-> r,
                proposal |-> prop,
                validRound |-> vr]
            IN
            msg \in msgsPropose[(r)]
              /\ LET PV == { m \in msgsPrevote[vr]: m["id"] = Id((prop)) } IN
              Cardinality((PV)) >= THRESHOLD2
                /\ evidence' = (PV \union {(msg)}) \union evidence
            /\ LET mid ==
              IF IsValid((prop)) /\ (lockedRound[p] <= vr \/ lockedValue[p] = v)
              THEN Id((prop))
              ELSE NilProposal
            IN
            BroadcastPrevote(p, (r), (mid))
            /\ step' = [ step EXCEPT ![p] = "PREVOTE" ]
            /\ UNCHANGED (<<(temporalVars), beginRound>>)
            /\ UNCHANGED (<<
              round, decision, lockedValue, lockedRound, validValue, validRound
            >>)
            /\ UNCHANGED (<<msgsPropose, msgsPrecommit, proposalReceptionTime>>)
            /\ action' = "UponProposalInProposeAndPrevote"

TimeLiveness ==
  \A r \in Rounds \ {3}:
    \A v \in { "v0", "v1" }:
      LET p == Proposer[r] IN
      p \in { "c1", "c2", "c3" }
        => (\E t \in Timestamps:
          LET prop == Proposal(v, t, r) IN
          IsFirstProposedInRound((prop), (p), r)
            /\ LET tOffset == (t + 2) + 2 IN
            (firstBeginRound)[r] <= t
              /\ t <= (lastBeginRound)[r]
              /\ (lastBeginRound)[r] <= tOffset
              /\ tOffset <= (firstBeginRound)[(r + 1)]
            => (\A q \in { "c1", "c2", "c3" }:
              LET dq == decision[q] IN dq /= NilDecision => (dq)[1] = prop))

InitPreloadAllMsgs ==
  msgsPropose \in [(Rounds) -> (SUBSET AllFaultyProposals)]
    /\ msgsPrevote \in [(Rounds) -> (SUBSET AllFaultyPrevotes)]
    /\ msgsPrecommit \in [(Rounds) -> (SUBSET AllFaultyPrecommits)]
    /\ BenignRoundsInMessages(msgsPropose)
    /\ BenignRoundsInMessages(msgsPrevote)
    /\ BenignRoundsInMessages(msgsPrecommit)

InitGen ==
  msgsPropose \in [(Rounds) -> Gen(5)]
    /\ msgsPrevote \in [(Rounds) -> Gen(5)]
    /\ msgsPrecommit \in [(Rounds) -> Gen(5)]
    /\ BenignAndSubset(msgsPropose, (AllFaultyProposals))
    /\ BenignAndSubset(msgsPrevote, (AllFaultyPrevotes))
    /\ BenignAndSubset(msgsPrecommit, (AllFaultyPrecommits))

InitMsgs ==
  (TRUE /\ InitGen)
    \/ (~TRUE
      /\ msgsPropose = [ r \in Rounds |-> {} ]
      /\ msgsPrevote = [ r \in Rounds |-> {} ]
      /\ msgsPrecommit = [ r \in Rounds |-> {} ])

(*
  a conjunction of all invariants
*)
Inv ==
  AgreementOnValue
    /\ ConsensusValidValue
    /\ ConsensusTimeValid
    /\ TimeLiveness

(*
  process timely messages
  @type:  (PROCESS) => Bool;
*)
MessageProcessing(p) ==
  InsertProposal(p)
    \/ ReceiveProposal(p)
    \/ UponProposalInPropose(p)
    \/ UponProposalInProposeAndPrevote(p)
    \/ UponQuorumOfPrevotesAny(p)
    \/ UponProposalInPrevoteOrCommitAndPrevote(p)
    \/ UponQuorumOfPrecommitsAny(p)
    \/ UponProposalInPrecommitNoDecision(p)
    \/ OnTimeoutPropose(p)
    \/ OnQuorumOfNilPrevotes(p)
    \/ OnRoundCatchup(p)

(*
  The initial states of the protocol. Some faults can be in the system already.
*)
Init ==
  round = [ p \in { "c1", "c2", "c3" } |-> 0 ]
    /\ localClock \in [{ "c1", "c2", "c3" } -> (2 .. 2 + 2)]
    /\ realTime = 0
    /\ step = [ p \in { "c1", "c2", "c3" } |-> "PROPOSE" ]
    /\ decision = [ p \in { "c1", "c2", "c3" } |-> NilDecision ]
    /\ lockedValue = [ p \in { "c1", "c2", "c3" } |-> NilValue ]
    /\ lockedRound = [ p \in { "c1", "c2", "c3" } |-> NilRound ]
    /\ validValue = [ p \in { "c1", "c2", "c3" } |-> NilProposal ]
    /\ validRound = [ p \in { "c1", "c2", "c3" } |-> NilRound ]
    /\ InitMsgs
    /\ proposalReceptionTime
      = [ r \in Rounds, p \in { "c1", "c2", "c3" } |-> NilTimestamp ]
    /\ evidence = {}
    /\ action = "Init"
    /\ beginRound
      = [
        r \in Rounds,
        c \in { "c1", "c2", "c3" } |->
          IF r = 0 THEN localClock[c] ELSE 7
      ]

(*
  * A system transition. In this specificatiom, the system may eventually deadlock,
 * e.g., when all processes decide. This is expected behavior, as we focus on safety.
*)
Next ==
  AdvanceRealTime
    \/ FaultyBroadcast
    \/ (SynchronizedLocalClocks
      /\ (\E p \in { "c1", "c2", "c3" }: MessageProcessing(p)))

================================================================================
