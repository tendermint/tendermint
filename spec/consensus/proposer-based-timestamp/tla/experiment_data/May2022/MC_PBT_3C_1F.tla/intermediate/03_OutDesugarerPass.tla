-------------------------- MODULE 03_OutDesugarerPass --------------------------

EXTENDS Integers, Sequences, FiniteSets, TLC, Apalache

CONSTANT
  (*
    @type: (Int -> Str);
  *)
  Proposer

VARIABLE
  (*
    @type: (Str -> <<<<Str, Int, Int>>, Int>>);
  *)
  decision

VARIABLE
  (*
    @type: (Int -> Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]));
  *)
  msgsPrecommit

VARIABLE
  (*
    @type: (<<Int, Str>> -> Int);
  *)
  beginRound

VARIABLE
  (*
    @type: (Int -> Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]));
  *)
  msgsPrevote

VARIABLE
  (*
    @type: Str;
  *)
  action

VARIABLE
  (*
    @type: (Str -> Int);
  *)
  lockedRound

VARIABLE
  (*
    @type: (Int -> Set([proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]));
  *)
  msgsPropose

VARIABLE
  (*
    @type: (Str -> Int);
  *)
  validRound

VARIABLE
  (*
    @type: (Str -> Str);
  *)
  step

VARIABLE
  (*
    @type: (Str -> Str);
  *)
  lockedValue

VARIABLE
  (*
    @type: (Str -> <<Str, Int, Int>>);
  *)
  validValue

VARIABLE
  (*
    @type: Int;
  *)
  realTime

VARIABLE
  (*
    @type: (Str -> Int);
  *)
  round

VARIABLE
  (*
    @type: Set([id: <<Str, Int, Int>>, proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
  *)
  evidence

VARIABLE
  (*
    @type: (<<Int, Str>> -> Int);
  *)
  proposalReceptionTime

VARIABLE
  (*
    @type: (Str -> Int);
  *)
  localClock

(*
  @type: (() => Int);
*)
NilRound == -1

(*
  @type: (() => Bool);
*)
TypeAliases == TRUE

(*
  @type: (() => Set(Str));
*)
Values == { "v0", "v1" } \union {"v2"}

(*
  @type: (() => Set(Int));
*)
Timestamps == 0 .. 7

(*
  @type: (([proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]) => Str);
*)
MessageValue(msg) == msg["proposal"][1]

(*
  @type: ((Int, Int) => Int);
*)
Min2(a, b) == IF a <= b THEN a ELSE b

(*
  @type: ((<<Str, Int, Int>>, Int) => <<<<Str, Int, Int>>, Int>>);
*)
Decision(p, r) == <<p, r>>

(*
  @type: ((Int) => Bool);
*)
ValidTime(t) == t < 7

(*
  @type: (() => Set(Str));
*)
AllProcs == { "c1", "c2", "c3" } \union {"f4"}

(*
  @type: (([proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]) => Int);
*)
MessageRound(msg) == msg["proposal"][3]

(*
  @type: ((Int, Int) => Int);
*)
Max2(a, b) == IF a >= b THEN a ELSE b

(*
  @type: ((Seq(a1060), ((a1060) => Bool)) => Seq(a1060));
*)
SelectSeq(__s, __Test(_)) ==
  LET (*
    @type: ((Seq(a1060), a1060) => Seq(a1060));
  *)
  __AppendIfTest(__res, __e) ==
    IF __Test(__e) THEN Append(__res, __e) ELSE __res
  IN
  ApaFoldSeqLeft(__AppendIfTest, <<>>, __s)

(*
  @type: (([proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]) => Int);
*)
MessageTime(msg) == msg["proposal"][2]

(*
  @type: (() => Int);
*)
NilTimestamp == -1

(*
  @type: ((Int, Int) => Bool);
*)
IsTimely(processTime, messageTime) ==
  processTime >= messageTime - 2 /\ processTime <= (messageTime + 2) + 2

(*
  @type: ((Set([id: <<Str, Int, Int>>, proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int])) => Int);
*)
Card(S) ==
  LET (*
    @type: ((Int, [id: <<Str, Int, Int>>, proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]) => Int);
  *)
  PlusOne(i, m) == i + 1
  IN
  ApaFoldSet(PlusOne, 0, S)

(*
  @type: (((Int -> a1041), Int, Int) => Seq(a1041));
*)
FunAsSeq(__fn, __len, __capacity) ==
  LET (*@type: ((Int) => a1041); *) __FunAsSeq_elem_ctor(__i) == __fn[__i] IN
  SubSeq(MkSeq(__capacity, __FunAsSeq_elem_ctor), 1, __len)

(*
  @type: (() => Set(Int));
*)
Rounds == 0 .. 3

(*
  @type: (() => Int);
*)
THRESHOLD2 == 2 * 1 + 1

(*
  @type: ((a1036) => a1036);
*)
Id(v) == v

(*
  @type: (() => Int);
*)
THRESHOLD1 == 1 + 1

(*
  @type: ((Str, Int, Int) => <<Str, Int, Int>>);
*)
Proposal(v, t, r) == <<v, t, r>>

(*
  @type: (() => Str);
*)
NilValue == "None"

(*
  @type: ((Set(Int)) => Int);
*)
Min(S) == ApaFoldSet(Min2, 7, S)

(*
  @type: (() => Bool);
*)
ArbitraryProposer == Proposer \in [(Rounds) -> (AllProcs)]

(*
  @type: (() => Bool);
*)
CorrectProposer == Proposer \in [(Rounds) -> { "c1", "c2", "c3" }]

(*
  @type: (() => <<(Str -> Int), Int>>);
*)
temporalVars == <<localClock, realTime>>

(*
  @type: (((Int -> Set([id: <<Str, Int, Int>>, proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int])), Set([id: <<Str, Int, Int>>, proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int])) => Bool);
*)
BenignAndSubset(msgfun, set) ==
  (\A r \in Rounds:
      msgfun[r] \subseteq set /\ (\A m \in msgfun[r]: r = m["round"]))

(*
  @type: (() => Set(<<Str, Int, Int>>));
*)
ValidProposals == { "v0", "v1" } \X (2 .. 7) \X Rounds

(*
  @type: ((Str, Int, <<Str, Int, Int>>, Int) => Bool);
*)
BroadcastProposal(pSrc, pRound, pProposal, pValidRound) ==
  LET (*
    @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
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

(*
  @type: (() => Bool);
*)
CyclicalProposer ==
  LET (*
    @type: (() => Seq(Str));
  *)
  ProcOrder ==
    LET (*
      @type: ((Seq(a980), a980) => Seq(a980));
    *)
    App(s, e) == Append(s, e)
    IN
    ApaFoldSet(App, <<>>, (AllProcs))
  IN
  Proposer = [ r \in Rounds |-> (ProcOrder)[(1 + (r % 4))] ]

(*
  @type: ((<<Str, Int, Int>>, Str, Int) => Bool);
*)
IsFirstProposedInRound(prop, src, r) ==
  \E msg \in msgsPropose[r]:
    msg["proposal"] = prop /\ msg["src"] = src /\ msg["validRound"] = NilRound

(*
  @type: (() => <<(Int -> Set([proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int])), (Int -> Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str])), (Int -> Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str])), Set([id: <<Str, Int, Int>>, proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]), (<<Int, Str>> -> Int)>>);
*)
bookkeepingVars ==
  <<msgsPropose, msgsPrevote, msgsPrecommit, evidence, proposalReceptionTime>>

(*
  @type: (() => <<Str, Int, Int>>);
*)
NilProposal == <<(NilValue), (NilTimestamp), (NilRound)>>

(*
  @type: (() => Bool);
*)
SynchronizedLocalClocks ==
  \A p \in { "c1", "c2", "c3" }:
    \A q \in { "c1", "c2", "c3" }:
      p /= q
        => (localClock[p] >= localClock[q] /\ localClock[p] - localClock[q] < 2)
          \/ (localClock[p] < localClock[q] /\ localClock[q] - localClock[p] < 2)

(*
  @type: ((Set(Int)) => Int);
*)
Max(S) == ApaFoldSet(Max2, (NilTimestamp), S)

(*
  @type: ((Str, Int, <<Str, Int, Int>>) => Bool);
*)
BroadcastPrevote(pSrc, pRound, pId) ==
  LET (*
    @type: (() => [id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]);
  *)
  newMsg == [type |-> "PREVOTE", src |-> pSrc, round |-> pRound, id |-> pId]
  IN
  msgsPrevote'
      = [ msgsPrevote EXCEPT ![pRound] = msgsPrevote[pRound] \union {(newMsg)} ]

(*
  @type: ((Str, Int) => Bool);
*)
StartRound(p, r) ==
  step[p] /= "DECIDED"
    /\ round' = [ round EXCEPT ![p] = r ]
    /\ step' = [ step EXCEPT ![p] = "PROPOSE" ]
    /\ beginRound' = [ beginRound EXCEPT ![<<r, p>>] = localClock[p] ]

(*
  @type: (() => Bool);
*)
ConsensusTimeValid ==
  \A p \in { "c1", "c2", "c3" }:
    \E v \in { "v0", "v1" }:
      \E t \in Timestamps:
        \E pr \in Rounds:
          \E dr \in Rounds:
            decision[p] = Decision((Proposal(v, t, pr)), dr)
              => (\E q \in { "c1", "c2", "c3" }:
                LET (*
                  @type: (() => Int);
                *)
                propRecvTime == proposalReceptionTime[pr, q]
                IN
                beginRound[pr, q] <= propRecvTime
                  /\ beginRound[(pr + 1), q] >= propRecvTime
                  /\ IsTimely((propRecvTime), t))

(*
  @type: (((Int -> Set([id: <<Str, Int, Int>>, proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]))) => Bool);
*)
BenignRoundsInMessages(msgfun) ==
  \A r \in Rounds: \A m \in msgfun[r]: r = m["round"]

(*
  @type: ((Str, Int, <<Str, Int, Int>>) => Bool);
*)
BroadcastPrecommit(pSrc, pRound, pId) ==
  LET (*
    @type: (() => [id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]);
  *)
  newMsg == [type |-> "PRECOMMIT", src |-> pSrc, round |-> pRound, id |-> pId]
  IN
  msgsPrecommit'
      = [
        msgsPrecommit EXCEPT
          ![pRound] = msgsPrecommit[pRound] \union {(newMsg)}
      ]

(*
  @type: (() => Set(Int));
*)
RoundsOrNil == Rounds \union {(NilRound)}

(*
  @type: (() => Set(<<Str, Int, Int>>));
*)
Proposals == Values \X Timestamps \X Rounds

(*
  @type: (() => <<(Str -> Int), (Str -> Str), (Str -> <<<<Str, Int, Int>>, Int>>), (Str -> Str), (Str -> Int), (Str -> <<Str, Int, Int>>), (Str -> Int)>>);
*)
coreVars ==
  <<round, step, decision, lockedValue, lockedRound, validValue, validRound>>

(*
  @type: (() => Set(Str));
*)
ValuesOrNil == Values \union {(NilValue)}

(*
  @type: ((<<Str, Int, Int>>) => Bool);
*)
IsValid(p) == p \in ValidProposals

(*
  @type: ((Str) => Bool);
*)
OnQuorumOfNilPrevotes(p) ==
  step[p] = "PREVOTE"
    /\ LET (*
      @type: (() => Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]));
    *)
    PV == { m \in msgsPrevote[round[p]]: m["id"] = Id((NilProposal)) }
    IN
    Cardinality((PV)) >= THRESHOLD2
      /\ evidence' = PV \union evidence
      /\ BroadcastPrecommit(p, round[p], (Id((NilProposal))))
      /\ step' = [ step EXCEPT ![p] = "PRECOMMIT" ]
      /\ (temporalVars' = temporalVars /\ beginRound' := beginRound)
      /\ (round' := round
        /\ decision' := decision
        /\ lockedValue' := lockedValue
        /\ lockedRound' := lockedRound
        /\ validValue' := validValue
        /\ validRound' := validRound)
      /\ (msgsPropose' := msgsPropose
        /\ msgsPrevote' := msgsPrevote
        /\ proposalReceptionTime' := proposalReceptionTime)
      /\ action' = "OnQuorumOfNilPrevotes"

(*
  @type: (() => Bool);
*)
AdvanceRealTime ==
  ValidTime(realTime)
    /\ (\E t \in Timestamps:
      t > realTime
        /\ realTime' = t
        /\ localClock'
          = [ p \in { "c1", "c2", "c3" } |-> localClock[p] + t - realTime ])
    /\ (coreVars' = coreVars
      /\ bookkeepingVars' = bookkeepingVars
      /\ beginRound' := beginRound)
    /\ action' = "AdvanceRealTime"

(*
  @type: ((Int) => Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]));
*)
FaultyPrevotes(r) == [type: {"PREVOTE"}, src: {"f4"}, round: {r}, id: Proposals]

(*
  @type: (() => (Int -> Int));
*)
lastBeginRound ==
  [ r \in Rounds |-> Max({ beginRound[r, p]: p \in { "c1", "c2", "c3" } }) ]

(*
  @type: (() => Set([proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]));
*)
AllFaultyProposals ==
  [type: {"PROPOSAL"},
    src: {"f4"},
    round: Rounds,
    proposal: Proposals,
    validRound: RoundsOrNil]

(*
  @type: ((Str) => Bool);
*)
InsertProposal(p) ==
  LET (*@type: (() => Int); *) r == round[p] IN
  p = Proposer[(r)]
    /\ step[p] = "PROPOSE"
    /\ (\A m \in msgsPropose[(r)]: m["src"] /= p)
    /\ (\E v \in { "v0", "v1" }:
      LET (*
        @type: (() => <<Str, Int, Int>>);
      *)
      proposal ==
        IF validValue[p] /= NilProposal
        THEN validValue[p]
        ELSE Proposal(v, localClock[p], (r))
      IN
      BroadcastProposal(p, (r), (proposal), validRound[p]))
    /\ (temporalVars' = temporalVars /\ coreVars' = coreVars)
    /\ (msgsPrevote' := msgsPrevote
      /\ msgsPrecommit' := msgsPrecommit
      /\ evidence' := evidence
      /\ proposalReceptionTime' := proposalReceptionTime)
    /\ beginRound' := beginRound
    /\ action' = "InsertProposal"

(*
  @type: (() => (Int -> Int));
*)
firstBeginRound ==
  [ r \in Rounds |-> Min({ beginRound[r, p]: p \in { "c1", "c2", "c3" } }) ]

(*
  @type: (() => Bool);
*)
ConsensusValidValue ==
  \A p \in { "c1", "c2", "c3" }:
    LET (*@type: (() => <<Str, Int, Int>>); *) prop == decision[p][1] IN
    prop /= NilProposal => (prop)[1] \in { "v0", "v1" }

(*
  @type: ((Str) => Bool);
*)
UponProposalInPrevoteOrCommitAndPrevote(p) ==
  \E v \in { "v0", "v1" }:
    \E t \in Timestamps:
      \E vr \in RoundsOrNil:
        LET (*@type: (() => Int); *) r == round[p] IN
        LET (*
          @type: (() => <<Str, Int, Int>>);
        *)
        prop == Proposal(v, t, (r))
        IN
        step[p] \in { "PREVOTE", "PRECOMMIT" }
          /\ LET (*
            @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
          *)
          msg ==
            [type |-> "PROPOSAL",
              src |-> Proposer[(r)],
              round |-> r,
              proposal |-> prop,
              validRound |-> vr]
          IN
          msg \in msgsPropose[(r)]
            /\ LET (*
              @type: (() => Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]));
            *)
            PV == { m \in msgsPrevote[(r)]: m["id"] = Id((prop)) }
            IN
            Cardinality((PV)) >= THRESHOLD2
              /\ evidence' = (PV \union {(msg)}) \union evidence
          /\ (IF step[p] = "PREVOTE"
          THEN lockedValue' = [ lockedValue EXCEPT ![p] = v ]
            /\ lockedRound' = [ lockedRound EXCEPT ![p] = r ]
            /\ BroadcastPrecommit(p, (r), (Id((prop))))
            /\ step' = [ step EXCEPT ![p] = "PRECOMMIT" ]
          ELSE lockedValue' := lockedValue
            /\ lockedRound' := lockedRound
            /\ msgsPrecommit' := msgsPrecommit
            /\ step' := step)
          /\ validValue' = [ validValue EXCEPT ![p] = prop ]
          /\ validRound' = [ validRound EXCEPT ![p] = r ]
          /\ (temporalVars' = temporalVars /\ beginRound' := beginRound)
          /\ (round' := round /\ decision' := decision)
          /\ (msgsPropose' := msgsPropose
            /\ msgsPrevote' := msgsPrevote
            /\ proposalReceptionTime' := proposalReceptionTime)
          /\ action' = "UponProposalInPrevoteOrCommitAndPrevote"

(*
  @type: ((Str) => Bool);
*)
OnTimeoutPropose(p) ==
  step[p] = "PROPOSE"
    /\ p /= Proposer[round[p]]
    /\ BroadcastPrevote(p, round[p], (NilProposal))
    /\ step' = [ step EXCEPT ![p] = "PREVOTE" ]
    /\ (temporalVars' = temporalVars /\ beginRound' := beginRound)
    /\ (round' := round
      /\ decision' := decision
      /\ lockedValue' := lockedValue
      /\ lockedRound' := lockedRound
      /\ validValue' := validValue
      /\ validRound' := validRound)
    /\ (msgsPropose' := msgsPropose
      /\ msgsPrecommit' := msgsPrecommit
      /\ evidence' := evidence
      /\ proposalReceptionTime' := proposalReceptionTime)
    /\ action' = "OnTimeoutPropose"

(*
  @type: (() => Bool);
*)
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
  @type: (() => Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]));
*)
AllFaultyPrecommits ==
  [type: {"PRECOMMIT"}, src: {"f4"}, round: Rounds, id: Proposals]

(*
  @type: ((Int) => Set([proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]));
*)
FaultyProposals(r) ==
  [type: {"PROPOSAL"},
    src: {"f4"},
    round: {r},
    proposal: Proposals,
    validRound: RoundsOrNil]

(*
  @type: (() => Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]));
*)
AllFaultyPrevotes ==
  [type: {"PREVOTE"}, src: {"f4"}, round: Rounds, id: Proposals]

(*
  @type: ((Str) => Bool);
*)
OnRoundCatchup(p) ==
  \E r \in Rounds:
    r > round[p]
      /\ LET (*
        @type: (() => Set([id: <<Str, Int, Int>>, proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]));
      *)
      RoundMsgs ==
        (msgsPropose[r] \union msgsPrevote[r]) \union msgsPrecommit[r]
      IN
      \E MyEvidence \in SUBSET RoundMsgs:
        LET (*
          @type: (() => Set(Str));
        *)
        Faster == { m["src"]: m \in MyEvidence }
        IN
        Cardinality((Faster)) >= THRESHOLD1
          /\ evidence' = MyEvidence \union evidence
          /\ StartRound(p, r)
          /\ temporalVars' = temporalVars
          /\ (decision' := decision
            /\ lockedValue' := lockedValue
            /\ lockedRound' := lockedRound
            /\ validValue' := validValue
            /\ validRound' := validRound)
          /\ (msgsPropose' := msgsPropose
            /\ msgsPrevote' := msgsPrevote
            /\ msgsPrecommit' := msgsPrecommit
            /\ proposalReceptionTime' := proposalReceptionTime)
          /\ action' = "OnRoundCatchup"

(*
  @type: ((Int) => Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]));
*)
FaultyPrecommits(r) ==
  [type: {"PRECOMMIT"}, src: {"f4"}, round: {r}, id: Proposals]

(*
  @type: (() => Set([proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]));
*)
AllProposals ==
  [type: {"PROPOSAL"},
    src: AllProcs,
    round: Rounds,
    proposal: Proposals,
    validRound: RoundsOrNil]

(*
  @type: ((Str) => Bool);
*)
UponQuorumOfPrevotesAny(p) ==
  step[p] = "PREVOTE"
    /\ (\E MyEvidence \in SUBSET (msgsPrevote[round[p]]):
      LET (*
        @type: (() => Set(Str));
      *)
      Voters == { m["src"]: m \in MyEvidence }
      IN
      Cardinality((Voters)) >= THRESHOLD2
        /\ evidence' = MyEvidence \union evidence
        /\ BroadcastPrecommit(p, round[p], (NilProposal))
        /\ step' = [ step EXCEPT ![p] = "PRECOMMIT" ]
        /\ (temporalVars' = temporalVars /\ beginRound' := beginRound)
        /\ (round' := round
          /\ decision' := decision
          /\ lockedValue' := lockedValue
          /\ lockedRound' := lockedRound
          /\ validValue' := validValue
          /\ validRound' := validRound)
        /\ (msgsPropose' := msgsPropose
          /\ msgsPrevote' := msgsPrevote
          /\ proposalReceptionTime' := proposalReceptionTime)
        /\ action' = "UponQuorumOfPrevotesAny")

(*
  @type: ((Str) => Bool);
*)
ReceiveProposal(p) ==
  \E v \in Values:
    \E t \in Timestamps:
      LET (*@type: (() => Int); *) r == round[p] IN
        LET (*
          @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
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
          /\ (temporalVars' = temporalVars /\ coreVars' = coreVars)
          /\ (msgsPropose' := msgsPropose
            /\ msgsPrevote' := msgsPrevote
            /\ msgsPrecommit' := msgsPrecommit
            /\ evidence' := evidence)
          /\ beginRound' := beginRound
          /\ action' = "ReceiveProposal"

(*
  @type: ((Int) => Set([proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]));
*)
RoundProposals(r) ==
  [type: {"PROPOSAL"},
    src: AllProcs,
    round: {r},
    proposal: Proposals,
    validRound: RoundsOrNil]

(*
  @type: (() => <<<<Str, Int, Int>>, Int>>);
*)
NilDecision == <<(NilProposal), (NilRound)>>

(*
  @type: (() => Set(<<<<Str, Int, Int>>, Int>>));
*)
Decisions == Proposals \X Rounds

(*
  @type: ((Str) => Bool);
*)
UponQuorumOfPrecommitsAny(p) ==
  (\E MyEvidence \in SUBSET (msgsPrecommit[round[p]]):
      LET (*
        @type: (() => Set(Str));
      *)
      Committers == { m["src"]: m \in MyEvidence }
      IN
      Cardinality((Committers)) >= THRESHOLD2
        /\ evidence' = MyEvidence \union evidence
        /\ round[p] + 1 \in Rounds
        /\ StartRound(p, (round[p] + 1))
        /\ temporalVars' = temporalVars
        /\ (decision' := decision
          /\ lockedValue' := lockedValue
          /\ lockedRound' := lockedRound
          /\ validValue' := validValue
          /\ validRound' := validRound)
        /\ (msgsPropose' := msgsPropose
          /\ msgsPrevote' := msgsPrevote
          /\ msgsPrecommit' := msgsPrecommit
          /\ proposalReceptionTime' := proposalReceptionTime)
        /\ action' = "UponQuorumOfPrecommitsAny")

(*
  @type: (() => Bool);
*)
CInit == ArbitraryProposer

(*
  @type: ((Str) => Bool);
*)
UponProposalInPrecommitNoDecision(p) ==
  decision[p] = NilDecision
    /\ (\E v \in { "v0", "v1" }:
      \E t \in Timestamps:
        \E r \in Rounds:
          \E pr \in Rounds:
            \E vr \in RoundsOrNil:
              LET (*
                @type: (() => <<Str, Int, Int>>);
              *)
              prop == Proposal(v, t, pr)
              IN
              LET (*
                  @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
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
                  /\ LET (*
                    @type: (() => Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]));
                  *)
                  PV == { m \in msgsPrecommit[r]: m["id"] = Id((prop)) }
                  IN
                  Cardinality((PV)) >= THRESHOLD2
                    /\ evidence' = (PV \union {(msg)}) \union evidence
                    /\ decision'
                      = [ decision EXCEPT ![p] = Decision((prop), r) ]
                    /\ step' = [ step EXCEPT ![p] = "DECIDED" ]
                    /\ temporalVars' = temporalVars
                    /\ (round' := round
                      /\ lockedValue' := lockedValue
                      /\ lockedRound' := lockedRound
                      /\ validValue' := validValue
                      /\ validRound' := validRound)
                    /\ (msgsPropose' := msgsPropose
                      /\ msgsPrevote' := msgsPrevote
                      /\ msgsPrecommit' := msgsPrecommit
                      /\ proposalReceptionTime' := proposalReceptionTime)
                    /\ beginRound' := beginRound
                    /\ action' = "UponProposalInPrecommitNoDecision")

(*
  @type: (() => Bool);
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
                  LET (*
                    @type: (() => <<Str, Int, Int>>);
                  *)
                  prop == Proposal(v, t, pr)
                  IN
                  decision[p] = Decision((prop), r1)
                    /\ decision[q] = Decision((prop), r2))

(*
  @type: ((Str) => Bool);
*)
UponProposalInPropose(p) ==
  \E v \in Values:
    \E t \in Timestamps:
      LET (*@type: (() => Int); *) r == round[p] IN
      LET (*@type: (() => <<Str, Int, Int>>); *) prop == Proposal(v, t, (r)) IN
      step[p] = "PROPOSE"
        /\ LET (*
          @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
        *)
        msg ==
          [type |-> "PROPOSAL",
            src |-> Proposer[(r)],
            round |-> r,
            proposal |-> prop,
            validRound |-> NilRound]
        IN
        evidence' = {(msg)} \union evidence
        /\ LET (*
          @type: (() => <<Str, Int, Int>>);
        *)
        mid ==
          IF IsTimely(proposalReceptionTime[(r), p], t)
            /\ IsValid((prop))
            /\ (lockedRound[p] = NilRound \/ lockedValue[p] = v)
          THEN Id((prop))
          ELSE NilProposal
        IN
        BroadcastPrevote(p, (r), (mid))
        /\ step' = [ step EXCEPT ![p] = "PREVOTE" ]
        /\ (temporalVars' = temporalVars /\ beginRound' := beginRound)
        /\ (round' := round
          /\ decision' := decision
          /\ lockedValue' := lockedValue
          /\ lockedRound' := lockedRound
          /\ validValue' := validValue
          /\ validRound' := validRound)
        /\ (msgsPropose' := msgsPropose
          /\ msgsPrecommit' := msgsPrecommit
          /\ proposalReceptionTime' := proposalReceptionTime)
        /\ action' = "UponProposalInPropose"

(*
  @type: (() => Bool);
*)
FaultyBroadcast ==
  ~TRUE
    /\ action' = "FaultyBroadcast"
    /\ (\E r \in Rounds:
      (\E msgs \in SUBSET (FaultyProposals(r)):
          msgsPropose'
              = [ msgsPropose EXCEPT ![r] = msgsPropose[r] \union msgs ]
            /\ (coreVars' = coreVars
              /\ temporalVars' = temporalVars
              /\ beginRound' := beginRound)
            /\ (msgsPrevote' := msgsPrevote
              /\ msgsPrecommit' := msgsPrecommit
              /\ evidence' := evidence
              /\ proposalReceptionTime' := proposalReceptionTime))
        \/ (\E msgs \in SUBSET (FaultyPrevotes(r)):
          msgsPrevote'
              = [ msgsPrevote EXCEPT ![r] = msgsPrevote[r] \union msgs ]
            /\ (coreVars' = coreVars
              /\ temporalVars' = temporalVars
              /\ beginRound' := beginRound)
            /\ (msgsPropose' := msgsPropose
              /\ msgsPrecommit' := msgsPrecommit
              /\ evidence' := evidence
              /\ proposalReceptionTime' := proposalReceptionTime))
        \/ (\E msgs \in SUBSET (FaultyPrecommits(r)):
          msgsPrecommit'
              = [ msgsPrecommit EXCEPT ![r] = msgsPrecommit[r] \union msgs ]
            /\ (coreVars' = coreVars
              /\ temporalVars' = temporalVars
              /\ beginRound' := beginRound)
            /\ (msgsPropose' := msgsPropose
              /\ msgsPrevote' := msgsPrevote
              /\ evidence' := evidence
              /\ proposalReceptionTime' := proposalReceptionTime)))

(*
  @type: ((Str) => Bool);
*)
UponProposalInProposeAndPrevote(p) ==
  \E v \in Values:
    \E t \in Timestamps:
      \E vr \in Rounds:
        \E pr \in Rounds:
          LET (*@type: (() => Int); *) r == round[p] IN
          LET (*
            @type: (() => <<Str, Int, Int>>);
          *)
          prop == Proposal(v, t, pr)
          IN
          ((step[p] = "PROPOSE" /\ 0 <= vr) /\ vr < r)
            /\ pr <= vr
            /\ LET (*
              @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
            *)
            msg ==
              [type |-> "PROPOSAL",
                src |-> Proposer[(r)],
                round |-> r,
                proposal |-> prop,
                validRound |-> vr]
            IN
            msg \in msgsPropose[(r)]
              /\ LET (*
                @type: (() => Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]));
              *)
              PV == { m \in msgsPrevote[vr]: m["id"] = Id((prop)) }
              IN
              Cardinality((PV)) >= THRESHOLD2
                /\ evidence' = (PV \union {(msg)}) \union evidence
            /\ LET (*
              @type: (() => <<Str, Int, Int>>);
            *)
            mid ==
              IF IsValid((prop)) /\ (lockedRound[p] <= vr \/ lockedValue[p] = v)
              THEN Id((prop))
              ELSE NilProposal
            IN
            BroadcastPrevote(p, (r), (mid))
            /\ step' = [ step EXCEPT ![p] = "PREVOTE" ]
            /\ (temporalVars' = temporalVars /\ beginRound' := beginRound)
            /\ (round' := round
              /\ decision' := decision
              /\ lockedValue' := lockedValue
              /\ lockedRound' := lockedRound
              /\ validValue' := validValue
              /\ validRound' := validRound)
            /\ (msgsPropose' := msgsPropose
              /\ msgsPrecommit' := msgsPrecommit
              /\ proposalReceptionTime' := proposalReceptionTime)
            /\ action' = "UponProposalInProposeAndPrevote"

(*
  @type: (() => Bool);
*)
TimeLiveness ==
  \A r \in Rounds \ {3}:
    \A v \in { "v0", "v1" }:
      LET (*@type: (() => Str); *) p == Proposer[r] IN
      p \in { "c1", "c2", "c3" }
        => (\E t \in Timestamps:
          LET (*
            @type: (() => <<Str, Int, Int>>);
          *)
          prop == Proposal(v, t, r)
          IN
          IsFirstProposedInRound((prop), (p), r)
            /\ LET (*@type: (() => Int); *) tOffset == (t + 2) + 2 IN
            (firstBeginRound)[r] <= t
              /\ t <= (lastBeginRound)[r]
              /\ (lastBeginRound)[r] <= tOffset
              /\ tOffset <= (firstBeginRound)[(r + 1)]
            => (\A q \in { "c1", "c2", "c3" }:
              LET (*
                @type: (() => <<<<Str, Int, Int>>, Int>>);
              *)
              dq == decision[q]
              IN
              dq /= NilDecision => (dq)[1] = prop))

(*
  @type: (() => Bool);
*)
InitPreloadAllMsgs ==
  msgsPropose \in [(Rounds) -> (SUBSET AllFaultyProposals)]
    /\ msgsPrevote \in [(Rounds) -> (SUBSET AllFaultyPrevotes)]
    /\ msgsPrecommit \in [(Rounds) -> (SUBSET AllFaultyPrecommits)]
    /\ BenignRoundsInMessages(msgsPropose)
    /\ BenignRoundsInMessages(msgsPrevote)
    /\ BenignRoundsInMessages(msgsPrecommit)

(*
  @type: (() => Bool);
*)
InitGen ==
  msgsPropose \in [(Rounds) -> Gen(5)]
    /\ msgsPrevote \in [(Rounds) -> Gen(5)]
    /\ msgsPrecommit \in [(Rounds) -> Gen(5)]
    /\ BenignAndSubset(msgsPropose, (AllFaultyProposals))
    /\ BenignAndSubset(msgsPrevote, (AllFaultyPrevotes))
    /\ BenignAndSubset(msgsPrecommit, (AllFaultyPrecommits))

(*
  @type: (() => Bool);
*)
InitMsgs ==
  (TRUE /\ InitGen)
    \/ (~TRUE
      /\ msgsPropose = [ r \in Rounds |-> {} ]
      /\ msgsPrevote = [ r \in Rounds |-> {} ]
      /\ msgsPrecommit = [ r \in Rounds |-> {} ])

(*
  @type: (() => Bool);
*)
Inv ==
  AgreementOnValue
    /\ ConsensusValidValue
    /\ ConsensusTimeValid
    /\ TimeLiveness

(*
  @type: ((Str) => Bool);
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
  @type: (() => Bool);
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
      = [ r_p \in Rounds \X { "c1", "c2", "c3" } |-> NilTimestamp ]
    /\ evidence = {}
    /\ action = "Init"
    /\ beginRound
      = [
        r_c \in Rounds \X { "c1", "c2", "c3" } |->
          IF r_c[1] = 0 THEN localClock[r_c[2]] ELSE 7
      ]

(*
  @type: (() => Bool);
*)
Next ==
  AdvanceRealTime
    \/ FaultyBroadcast
    \/ (SynchronizedLocalClocks
      /\ (\E p \in { "c1", "c2", "c3" }: MessageProcessing(p)))

================================================================================
