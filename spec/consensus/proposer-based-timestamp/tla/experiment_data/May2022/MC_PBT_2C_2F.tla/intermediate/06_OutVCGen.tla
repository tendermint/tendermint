------------------------------ MODULE 06_OutVCGen ------------------------------

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
  @type: (() => Bool);
*)
CInit == Proposer \in [(0 .. 3) -> ({ "c1", "c2" } \union { "f3", "f4" })]

(*
  @type: (() => Bool);
*)
Inv ==
  (\A p$22 \in { "c1", "c2" }:
      \A q$6 \in { "c1", "c2" }:
        decision[p$22] /= <<<<"None", (-1), (-1)>>, (-1)>>
          /\ decision[q$6] /= <<<<"None", (-1), (-1)>>, (-1)>>
          => (\E v$10 \in { "v0", "v1" }:
            \E t$10 \in 0 .. 7:
              \E pr$5 \in 0 .. 3:
                \E r1$3 \in 0 .. 3:
                  \E r2$3 \in 0 .. 3:
                    LET (*
                      @type: (() => <<Str, Int, Int>>);
                    *)
                    prop_si_8 == <<v$10, t$10, pr$5>>
                    IN
                    decision[p$22] = <<(prop_si_8), r1$3>>
                      /\ decision[q$6] = <<(prop_si_8), r2$3>>))
    /\ (\A p$23 \in { "c1", "c2" }:
      LET (*
        @type: (() => <<Str, Int, Int>>);
      *)
      prop_si_9 == decision[p$23][1]
      IN
      prop_si_9 /= <<"None", (-1), (-1)>> => (prop_si_9)[1] \in { "v0", "v1" })
    /\ (\A p$24 \in { "c1", "c2" }:
      \E v$11 \in { "v0", "v1" }:
        \E t$11 \in 0 .. 7:
          \E pr$6 \in 0 .. 3:
            \E dr$2 \in 0 .. 3:
              decision[p$24] = <<<<v$11, t$11, pr$6>>, dr$2>>
                => (\E q$7 \in { "c1", "c2" }:
                  LET (*
                    @type: (() => Int);
                  *)
                  propRecvTime_si_2 == proposalReceptionTime[pr$6, q$7]
                  IN
                  beginRound[pr$6, q$7] <= propRecvTime_si_2
                    /\ beginRound[(pr$6 + 1), q$7] >= propRecvTime_si_2
                    /\ (propRecvTime_si_2 >= t$11 - 2
                      /\ propRecvTime_si_2 <= (t$11 + 2) + 2)))
    /\ (\A r$31 \in 0 .. 3 \ {3}:
      \A v$12 \in { "v0", "v1" }:
        LET (*@type: (() => Str); *) p_si_25 == Proposer[r$31] IN
        p_si_25 \in { "c1", "c2" }
          => (\E t$12 \in 0 .. 7:
            LET (*
              @type: (() => <<Str, Int, Int>>);
            *)
            prop_si_10 == <<v$12, t$12, r$31>>
            IN
            (\E msg$8 \in msgsPropose[r$31]:
                msg$8["proposal"] = prop_si_10
                  /\ msg$8["src"] = p_si_25
                  /\ msg$8["validRound"] = -1)
              /\ LET (*@type: (() => Int); *) tOffset_si_2 == (t$12 + 2) + 2 IN
              [
                  r$32 \in 0 .. 3 |->
                    ApaFoldSet(LET (*
                      @type: ((Int, Int) => Int);
                    *)
                    Min2_si_4(a_si_7, b_si_7) == IF a$7 <= b$7 THEN a$7 ELSE b$7
                    IN
                    Min2$4, 7, {
                      beginRound[r$32, p$26]:
                        p$26 \in { "c1", "c2" }
                    })
                ][
                  r$31
                ]
                  <= t$12
                /\ t$12
                  <= [
                    r$33 \in 0 .. 3 |->
                      ApaFoldSet(LET (*
                        @type: ((Int, Int) => Int);
                      *)
                      Max2_si_4(a_si_8, b_si_8) ==
                        IF a$8 >= b$8 THEN a$8 ELSE b$8
                      IN
                      Max2$4, (-1), {
                        beginRound[r$33, p$27]:
                          p$27 \in { "c1", "c2" }
                      })
                  ][
                    r$31
                  ]
                /\ [
                  r$34 \in 0 .. 3 |->
                    ApaFoldSet(LET (*
                      @type: ((Int, Int) => Int);
                    *)
                    Max2_si_5(a_si_9, b_si_9) == IF a$9 >= b$9 THEN a$9 ELSE b$9
                    IN
                    Max2$5, (-1), {
                      beginRound[r$34, p$28]:
                        p$28 \in { "c1", "c2" }
                    })
                ][
                  r$31
                ]
                  <= tOffset_si_2
                /\ tOffset_si_2
                  <= [
                    r$35 \in 0 .. 3 |->
                      ApaFoldSet(LET (*
                        @type: ((Int, Int) => Int);
                      *)
                      Min2_si_5(a_si_10, b_si_10) ==
                        IF a$10 <= b$10 THEN a$10 ELSE b$10
                      IN
                      Min2$5, 7, {
                        beginRound[r$35, p$29]:
                          p$29 \in { "c1", "c2" }
                      })
                  ][
                    (r$31 + 1)
                  ]
              => (\A q$8 \in { "c1", "c2" }:
                LET (*
                  @type: (() => <<<<Str, Int, Int>>, Int>>);
                *)
                dq_si_2 == decision[q$8]
                IN
                dq_si_2 /= <<<<"None", (-1), (-1)>>, (-1)>>
                  => (dq_si_2)[1] = prop_si_10)))

(*
  @type: (() => Bool);
*)
Init ==
  round = [ p$10 \in { "c1", "c2" } |-> 0 ]
    /\ localClock \in [{ "c1", "c2" } -> (2 .. 2 + 2)]
    /\ realTime = 0
    /\ step = [ p$11 \in { "c1", "c2" } |-> "PROPOSE" ]
    /\ decision
      = [ p$12 \in { "c1", "c2" } |-> <<<<"None", (-1), (-1)>>, (-1)>> ]
    /\ lockedValue = [ p$13 \in { "c1", "c2" } |-> "None" ]
    /\ lockedRound = [ p$14 \in { "c1", "c2" } |-> -1 ]
    /\ validValue = [ p$15 \in { "c1", "c2" } |-> <<"None", (-1), (-1)>> ]
    /\ validRound = [ p$16 \in { "c1", "c2" } |-> -1 ]
    /\ ((TRUE
        /\ (msgsPropose \in [(0 .. 3) -> Gen(5)]
          /\ msgsPrevote \in [(0 .. 3) -> Gen(5)]
          /\ msgsPrecommit \in [(0 .. 3) -> Gen(5)]
          /\ ((\A r$43 \in 0 .. 3:
              msgsPropose[r$43]
                  \subseteq [type: {"PROPOSAL"},
                    src: { "f3", "f4" },
                    round: 0 .. 3,
                    proposal:
                      ({ "v0", "v1" } \union {"v2"}) \X (0 .. 7) \X (0 .. 3),
                    validRound: 0 .. 3 \union {(-1)}]
                /\ (\A m$29 \in msgsPropose[r$43]: r$43 = m$29["round"])))
          /\ ((\A r$44 \in 0 .. 3:
              msgsPrevote[r$44]
                  \subseteq [type: {"PREVOTE"},
                    src: { "f3", "f4" },
                    round: 0 .. 3,
                    id: ({ "v0", "v1" } \union {"v2"}) \X (0 .. 7) \X (0 .. 3)]
                /\ (\A m$30 \in msgsPrevote[r$44]: r$44 = m$30["round"])))
          /\ ((\A r$45 \in 0 .. 3:
              msgsPrecommit[r$45]
                  \subseteq [type: {"PRECOMMIT"},
                    src: { "f3", "f4" },
                    round: 0 .. 3,
                    id: ({ "v0", "v1" } \union {"v2"}) \X (0 .. 7) \X (0 .. 3)]
                /\ (\A m$31 \in msgsPrecommit[r$45]: r$45 = m$31["round"])))))
      \/ (~TRUE
        /\ msgsPropose = [ r$46 \in 0 .. 3 |-> {} ]
        /\ msgsPrevote = [ r$47 \in 0 .. 3 |-> {} ]
        /\ msgsPrecommit = [ r$48 \in 0 .. 3 |-> {} ]))
    /\ proposalReceptionTime = [ r_p$1 \in (0 .. 3) \X { "c1", "c2" } |-> -1 ]
    /\ evidence = {}
    /\ action = "Init"
    /\ beginRound
      = [
        r_c$1 \in (0 .. 3) \X { "c1", "c2" } |->
          IF r_c$1[1] = 0 THEN localClock[r_c$1[2]] ELSE 7
      ]

(*
  @type: (() => Bool);
*)
Next ==
  (realTime < 7
      /\ (\E t$18 \in 0 .. 7:
        t$18 > realTime
          /\ realTime' = t$18
          /\ localClock'
            = [ p$30 \in { "c1", "c2" } |-> localClock[p$30] + t$18 - realTime ])
      /\ (<<
          round, step, decision, lockedValue, lockedRound, validValue, validRound
        >>'
          = <<
            round, step, decision, lockedValue, lockedRound, validValue, validRound
          >>
        /\ <<
          msgsPropose, msgsPrevote, msgsPrecommit, evidence, proposalReceptionTime
        >>'
          = <<
            msgsPropose, msgsPrevote, msgsPrecommit, evidence, proposalReceptionTime
          >>
        /\ beginRound' := beginRound)
      /\ action' = "AdvanceRealTime")
    \/ (~TRUE
      /\ action' = "FaultyBroadcast"
      /\ (\E r$49 \in 0 .. 3:
        (\E msgs$4 \in SUBSET ([type: {"PROPOSAL"},
            src: { "f3", "f4" },
            round: {r$49},
            proposal: ({ "v0", "v1" } \union {"v2"}) \X (0 .. 7) \X (0 .. 3),
            validRound: 0 .. 3 \union {(-1)}]):
            msgsPropose'
                = [
                  msgsPropose EXCEPT
                    ![r$49] = msgsPropose[r$49] \union msgs$4
                ]
              /\ (<<
                  round, step, decision, lockedValue, lockedRound, validValue, validRound
                >>'
                  = <<
                    round, step, decision, lockedValue, lockedRound, validValue,
                    validRound
                  >>
                /\ <<localClock, realTime>>' = <<localClock, realTime>>
                /\ beginRound' := beginRound)
              /\ (msgsPrevote' := msgsPrevote
                /\ msgsPrecommit' := msgsPrecommit
                /\ evidence' := evidence
                /\ proposalReceptionTime' := proposalReceptionTime))
          \/ (\E msgs$5 \in SUBSET ([type: {"PREVOTE"},
            src: { "f3", "f4" },
            round: {r$49},
            id: ({ "v0", "v1" } \union {"v2"}) \X (0 .. 7) \X (0 .. 3)]):
            msgsPrevote'
                = [
                  msgsPrevote EXCEPT
                    ![r$49] = msgsPrevote[r$49] \union msgs$5
                ]
              /\ (<<
                  round, step, decision, lockedValue, lockedRound, validValue, validRound
                >>'
                  = <<
                    round, step, decision, lockedValue, lockedRound, validValue,
                    validRound
                  >>
                /\ <<localClock, realTime>>' = <<localClock, realTime>>
                /\ beginRound' := beginRound)
              /\ (msgsPropose' := msgsPropose
                /\ msgsPrecommit' := msgsPrecommit
                /\ evidence' := evidence
                /\ proposalReceptionTime' := proposalReceptionTime))
          \/ (\E msgs$6 \in SUBSET ([type: {"PRECOMMIT"},
            src: { "f3", "f4" },
            round: {r$49},
            id: ({ "v0", "v1" } \union {"v2"}) \X (0 .. 7) \X (0 .. 3)]):
            msgsPrecommit'
                = [
                  msgsPrecommit EXCEPT
                    ![r$49] = msgsPrecommit[r$49] \union msgs$6
                ]
              /\ (<<
                  round, step, decision, lockedValue, lockedRound, validValue, validRound
                >>'
                  = <<
                    round, step, decision, lockedValue, lockedRound, validValue,
                    validRound
                  >>
                /\ <<localClock, realTime>>' = <<localClock, realTime>>
                /\ beginRound' := beginRound)
              /\ (msgsPropose' := msgsPropose
                /\ msgsPrevote' := msgsPrevote
                /\ evidence' := evidence
                /\ proposalReceptionTime' := proposalReceptionTime))))
    \/ ((\A p$31 \in { "c1", "c2" }:
        \A q$9 \in { "c1", "c2" }:
          p$31 /= q$9
            => (localClock[p$31] >= localClock[q$9]
                /\ localClock[p$31] - localClock[q$9] < 2)
              \/ (localClock[p$31] < localClock[q$9]
                /\ localClock[q$9] - localClock[p$31] < 2))
      /\ (\E p$17 \in { "c1", "c2" }:
        LET (*@type: (() => Int); *) r_si_50 == round[p$17] IN
          p$17 = Proposer[(r_si_50)]
            /\ step[p$17] = "PROPOSE"
            /\ (\A m$32 \in msgsPropose[(r_si_50)]: m$32["src"] /= p$17)
            /\ (\E v$19 \in { "v0", "v1" }:
              LET (*
                @type: (() => <<Str, Int, Int>>);
              *)
              proposal_si_3 ==
                IF validValue[p$17] /= <<"None", (-1), (-1)>>
                THEN validValue[p$17]
                ELSE <<v$19, localClock[p$17], (r_si_50)>>
              IN
              LET (*
                  @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
                *)
                newMsg_si_18 ==
                  [type |-> "PROPOSAL",
                    src |-> p$17,
                    round |-> r_si_50,
                    proposal |-> proposal_si_3,
                    validRound |-> validRound[p$17]]
                IN
                msgsPropose'
                    = [
                      msgsPropose EXCEPT
                        ![r_si_50] =
                          msgsPropose[(r_si_50)] \union {(newMsg_si_18)}
                    ])
            /\ (<<localClock, realTime>>' = <<localClock, realTime>>
              /\ <<
                round, step, decision, lockedValue, lockedRound, validValue, validRound
              >>'
                = <<
                  round, step, decision, lockedValue, lockedRound, validValue, validRound
                >>)
            /\ (msgsPrevote' := msgsPrevote
              /\ msgsPrecommit' := msgsPrecommit
              /\ evidence' := evidence
              /\ proposalReceptionTime' := proposalReceptionTime)
            /\ beginRound' := beginRound
            /\ action' = "InsertProposal"
          \/ (\E v$20 \in { "v0", "v1" } \union {"v2"}:
            \E t$19 \in 0 .. 7:
              LET (*@type: (() => Int); *) r_si_51 == round[p$17] IN
                LET (*
                  @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
                *)
                msg_si_14 ==
                  [type |-> "PROPOSAL",
                    src |-> Proposer[round[p$17]],
                    round |-> round[p$17],
                    proposal |-> <<v$20, t$19, (r_si_51)>>,
                    validRound |-> -1]
                IN
                msg_si_14 \in msgsPropose[round[p$17]]
                  /\ proposalReceptionTime[(r_si_51), p$17] = -1
                  /\ proposalReceptionTime'
                    = [
                      proposalReceptionTime EXCEPT
                        ![<<(r_si_51), p$17>>] = localClock[p$17]
                    ]
                  /\ (<<localClock, realTime>>' = <<localClock, realTime>>
                    /\ <<
                      round, step, decision, lockedValue, lockedRound, validValue,
                      validRound
                    >>'
                      = <<
                        round, step, decision, lockedValue, lockedRound, validValue,
                        validRound
                      >>)
                  /\ (msgsPropose' := msgsPropose
                    /\ msgsPrevote' := msgsPrevote
                    /\ msgsPrecommit' := msgsPrecommit
                    /\ evidence' := evidence)
                  /\ beginRound' := beginRound
                  /\ action' = "ReceiveProposal")
          \/ (\E v$21 \in { "v0", "v1" } \union {"v2"}:
            \E t$20 \in 0 .. 7:
              LET (*@type: (() => Int); *) r_si_52 == round[p$17] IN
              LET (*
                @type: (() => <<Str, Int, Int>>);
              *)
              prop_si_15 == <<v$21, t$20, (r_si_52)>>
              IN
              step[p$17] = "PROPOSE"
                /\ LET (*
                  @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
                *)
                msg_si_15 ==
                  [type |-> "PROPOSAL",
                    src |-> Proposer[(r_si_52)],
                    round |-> r_si_52,
                    proposal |-> prop_si_15,
                    validRound |-> -1]
                IN
                evidence' = {(msg_si_15)} \union evidence
                /\ LET (*
                  @type: (() => <<Str, Int, Int>>);
                *)
                mid_si_5 ==
                  IF (proposalReceptionTime[(r_si_52), p$17] >= t$20 - 2
                      /\ proposalReceptionTime[(r_si_52), p$17]
                        <= (t$20 + 2) + 2)
                    /\ prop_si_15 \in { "v0", "v1" } \X (2 .. 7) \X (0 .. 3)
                    /\ (lockedRound[p$17] = -1 \/ lockedValue[p$17] = v$21)
                  THEN prop_si_15
                  ELSE <<"None", (-1), (-1)>>
                IN
                LET (*
                  @type: (() => [id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]);
                *)
                newMsg_si_19 ==
                  [type |-> "PREVOTE",
                    src |-> p$17,
                    round |-> r_si_52,
                    id |-> mid_si_5]
                IN
                msgsPrevote'
                    = [
                      msgsPrevote EXCEPT
                        ![r_si_52] =
                          msgsPrevote[(r_si_52)] \union {(newMsg_si_19)}
                    ]
                /\ step' = [ step EXCEPT ![p$17] = "PREVOTE" ]
                /\ (<<localClock, realTime>>' = <<localClock, realTime>>
                  /\ beginRound' := beginRound)
                /\ (round' := round
                  /\ decision' := decision
                  /\ lockedValue' := lockedValue
                  /\ lockedRound' := lockedRound
                  /\ validValue' := validValue
                  /\ validRound' := validRound)
                /\ (msgsPropose' := msgsPropose
                  /\ msgsPrecommit' := msgsPrecommit
                  /\ proposalReceptionTime' := proposalReceptionTime)
                /\ action' = "UponProposalInPropose")
          \/ (\E v$22 \in { "v0", "v1" } \union {"v2"}:
            \E t$21 \in 0 .. 7:
              \E vr$7 \in 0 .. 3:
                \E pr$9 \in 0 .. 3:
                  LET (*@type: (() => Int); *) r_si_53 == round[p$17] IN
                  LET (*
                    @type: (() => <<Str, Int, Int>>);
                  *)
                  prop_si_16 == <<v$22, t$21, pr$9>>
                  IN
                  ((step[p$17] = "PROPOSE" /\ 0 <= vr$7) /\ vr$7 < r_si_53)
                    /\ pr$9 <= vr$7
                    /\ LET (*
                      @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
                    *)
                    msg_si_16 ==
                      [type |-> "PROPOSAL",
                        src |-> Proposer[(r_si_53)],
                        round |-> r_si_53,
                        proposal |-> prop_si_16,
                        validRound |-> vr$7]
                    IN
                    msg_si_16 \in msgsPropose[(r_si_53)]
                      /\ LET (*
                        @type: (() => Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]));
                      *)
                      PV_si_9 ==
                        { m$33 \in msgsPrevote[vr$7]: m$33["id"] = prop_si_16 }
                      IN
                      Cardinality((PV_si_9)) >= 2 * 1 + 1
                        /\ evidence'
                          = (PV_si_9 \union {(msg_si_16)}) \union evidence
                    /\ LET (*
                      @type: (() => <<Str, Int, Int>>);
                    *)
                    mid_si_6 ==
                      IF prop_si_16 \in { "v0", "v1" } \X (2 .. 7) \X (0 .. 3)
                        /\ (lockedRound[p$17] <= vr$7
                          \/ lockedValue[p$17] = v$22)
                      THEN prop_si_16
                      ELSE <<"None", (-1), (-1)>>
                    IN
                    LET (*
                      @type: (() => [id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]);
                    *)
                    newMsg_si_20 ==
                      [type |-> "PREVOTE",
                        src |-> p$17,
                        round |-> r_si_53,
                        id |-> mid_si_6]
                    IN
                    msgsPrevote'
                        = [
                          msgsPrevote EXCEPT
                            ![r_si_53] =
                              msgsPrevote[(r_si_53)] \union {(newMsg_si_20)}
                        ]
                    /\ step' = [ step EXCEPT ![p$17] = "PREVOTE" ]
                    /\ (<<localClock, realTime>>' = <<localClock, realTime>>
                      /\ beginRound' := beginRound)
                    /\ (round' := round
                      /\ decision' := decision
                      /\ lockedValue' := lockedValue
                      /\ lockedRound' := lockedRound
                      /\ validValue' := validValue
                      /\ validRound' := validRound)
                    /\ (msgsPropose' := msgsPropose
                      /\ msgsPrecommit' := msgsPrecommit
                      /\ proposalReceptionTime' := proposalReceptionTime)
                    /\ action' = "UponProposalInProposeAndPrevote")
          \/ (step[p$17] = "PREVOTE"
            /\ (\E MyEvidence$7 \in SUBSET (msgsPrevote[round[p$17]]):
              LET (*
                @type: (() => Set(Str));
              *)
              Voters_si_3 == { m$34["src"]: m$34 \in MyEvidence$7 }
              IN
              Cardinality((Voters_si_3)) >= 2 * 1 + 1
                /\ evidence' = MyEvidence$7 \union evidence
                /\ LET (*
                  @type: (() => [id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]);
                *)
                newMsg_si_21 ==
                  [type |-> "PRECOMMIT",
                    src |-> p$17,
                    round |-> round[p$17],
                    id |-> <<"None", (-1), (-1)>>]
                IN
                msgsPrecommit'
                    = [
                      msgsPrecommit EXCEPT
                        ![round[p$17]] =
                          msgsPrecommit[round[p$17]] \union {(newMsg_si_21)}
                    ]
                /\ step' = [ step EXCEPT ![p$17] = "PRECOMMIT" ]
                /\ (<<localClock, realTime>>' = <<localClock, realTime>>
                  /\ beginRound' := beginRound)
                /\ (round' := round
                  /\ decision' := decision
                  /\ lockedValue' := lockedValue
                  /\ lockedRound' := lockedRound
                  /\ validValue' := validValue
                  /\ validRound' := validRound)
                /\ (msgsPropose' := msgsPropose
                  /\ msgsPrevote' := msgsPrevote
                  /\ proposalReceptionTime' := proposalReceptionTime)
                /\ action' = "UponQuorumOfPrevotesAny"))
          \/ (\E v$23 \in { "v0", "v1" }:
            \E t$22 \in 0 .. 7:
              \E vr$8 \in 0 .. 3 \union {(-1)}:
                LET (*@type: (() => Int); *) r_si_54 == round[p$17] IN
                LET (*
                  @type: (() => <<Str, Int, Int>>);
                *)
                prop_si_17 == <<v$23, t$22, (r_si_54)>>
                IN
                step[p$17] \in { "PREVOTE", "PRECOMMIT" }
                  /\ LET (*
                    @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
                  *)
                  msg_si_17 ==
                    [type |-> "PROPOSAL",
                      src |-> Proposer[(r_si_54)],
                      round |-> r_si_54,
                      proposal |-> prop_si_17,
                      validRound |-> vr$8]
                  IN
                  msg_si_17 \in msgsPropose[(r_si_54)]
                    /\ LET (*
                      @type: (() => Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]));
                    *)
                    PV_si_10 ==
                      {
                        m$35 \in msgsPrevote[(r_si_54)]:
                          m$35["id"] = prop_si_17
                      }
                    IN
                    Cardinality((PV_si_10)) >= 2 * 1 + 1
                      /\ evidence'
                        = (PV_si_10 \union {(msg_si_17)}) \union evidence
                  /\ (IF step[p$17] = "PREVOTE"
                  THEN lockedValue' = [ lockedValue EXCEPT ![p$17] = v$23 ]
                    /\ lockedRound' = [ lockedRound EXCEPT ![p$17] = r_si_54 ]
                    /\ LET (*
                      @type: (() => [id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]);
                    *)
                    newMsg_si_22 ==
                      [type |-> "PRECOMMIT",
                        src |-> p$17,
                        round |-> r_si_54,
                        id |-> prop_si_17]
                    IN
                    msgsPrecommit'
                        = [
                          msgsPrecommit EXCEPT
                            ![r_si_54] =
                              msgsPrecommit[(r_si_54)] \union {(newMsg_si_22)}
                        ]
                    /\ step' = [ step EXCEPT ![p$17] = "PRECOMMIT" ]
                  ELSE lockedValue' := lockedValue
                    /\ lockedRound' := lockedRound
                    /\ msgsPrecommit' := msgsPrecommit
                    /\ step' := step)
                  /\ validValue' = [ validValue EXCEPT ![p$17] = prop_si_17 ]
                  /\ validRound' = [ validRound EXCEPT ![p$17] = r_si_54 ]
                  /\ (<<localClock, realTime>>' = <<localClock, realTime>>
                    /\ beginRound' := beginRound)
                  /\ (round' := round /\ decision' := decision)
                  /\ (msgsPropose' := msgsPropose
                    /\ msgsPrevote' := msgsPrevote
                    /\ proposalReceptionTime' := proposalReceptionTime)
                  /\ action' = "UponProposalInPrevoteOrCommitAndPrevote")
          \/ ((\E MyEvidence$8 \in SUBSET (msgsPrecommit[round[p$17]]):
              LET (*
                @type: (() => Set(Str));
              *)
              Committers_si_3 == { m$36["src"]: m$36 \in MyEvidence$8 }
              IN
              Cardinality((Committers_si_3)) >= 2 * 1 + 1
                /\ evidence' = MyEvidence$8 \union evidence
                /\ round[p$17] + 1 \in 0 .. 3
                /\ (step[p$17] /= "DECIDED"
                  /\ round' = [ round EXCEPT ![p$17] = round[p$17] + 1 ]
                  /\ step' = [ step EXCEPT ![p$17] = "PROPOSE" ]
                  /\ beginRound'
                    = [
                      beginRound EXCEPT
                        ![<<(round[p$17] + 1), p$17>>] = localClock[p$17]
                    ])
                /\ <<localClock, realTime>>' = <<localClock, realTime>>
                /\ (decision' := decision
                  /\ lockedValue' := lockedValue
                  /\ lockedRound' := lockedRound
                  /\ validValue' := validValue
                  /\ validRound' := validRound)
                /\ (msgsPropose' := msgsPropose
                  /\ msgsPrevote' := msgsPrevote
                  /\ msgsPrecommit' := msgsPrecommit
                  /\ proposalReceptionTime' := proposalReceptionTime)
                /\ action' = "UponQuorumOfPrecommitsAny"))
          \/ (decision[p$17] = <<<<"None", (-1), (-1)>>, (-1)>>
            /\ (\E v$24 \in { "v0", "v1" }:
              \E t$23 \in 0 .. 7:
                \E r$55 \in 0 .. 3:
                  \E pr$10 \in 0 .. 3:
                    \E vr$9 \in 0 .. 3 \union {(-1)}:
                      LET (*
                        @type: (() => <<Str, Int, Int>>);
                      *)
                      prop_si_18 == <<v$24, t$23, pr$10>>
                      IN
                      LET (*
                          @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
                        *)
                        msg_si_18 ==
                          [type |-> "PROPOSAL",
                            src |-> Proposer[r$55],
                            round |-> r$55,
                            proposal |-> prop_si_18,
                            validRound |-> vr$9]
                        IN
                        msg_si_18 \in msgsPropose[r$55]
                          /\ proposalReceptionTime[r$55, p$17] /= -1
                          /\ LET (*
                            @type: (() => Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]));
                          *)
                          PV_si_11 ==
                            {
                              m$37 \in msgsPrecommit[r$55]:
                                m$37["id"] = prop_si_18
                            }
                          IN
                          Cardinality((PV_si_11)) >= 2 * 1 + 1
                            /\ evidence'
                              = (PV_si_11 \union {(msg_si_18)}) \union evidence
                            /\ decision'
                              = [
                                decision EXCEPT
                                  ![p$17] = <<(prop_si_18), r$55>>
                              ]
                            /\ step' = [ step EXCEPT ![p$17] = "DECIDED" ]
                            /\ <<localClock, realTime>>'
                              = <<localClock, realTime>>
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
                            /\ action' = "UponProposalInPrecommitNoDecision"))
          \/ (step[p$17] = "PROPOSE"
            /\ p$17 /= Proposer[round[p$17]]
            /\ LET (*
              @type: (() => [id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]);
            *)
            newMsg_si_23 ==
              [type |-> "PREVOTE",
                src |-> p$17,
                round |-> round[p$17],
                id |-> <<"None", (-1), (-1)>>]
            IN
            msgsPrevote'
                = [
                  msgsPrevote EXCEPT
                    ![round[p$17]] =
                      msgsPrevote[round[p$17]] \union {(newMsg_si_23)}
                ]
            /\ step' = [ step EXCEPT ![p$17] = "PREVOTE" ]
            /\ (<<localClock, realTime>>' = <<localClock, realTime>>
              /\ beginRound' := beginRound)
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
            /\ action' = "OnTimeoutPropose")
          \/ (step[p$17] = "PREVOTE"
            /\ LET (*
              @type: (() => Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]));
            *)
            PV_si_12 ==
              {
                m$38 \in msgsPrevote[round[p$17]]:
                  m$38["id"] = <<"None", (-1), (-1)>>
              }
            IN
            Cardinality((PV_si_12)) >= 2 * 1 + 1
              /\ evidence' = PV_si_12 \union evidence
              /\ LET (*
                @type: (() => [id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]);
              *)
              newMsg_si_24 ==
                [type |-> "PRECOMMIT",
                  src |-> p$17,
                  round |-> round[p$17],
                  id |-> <<"None", (-1), (-1)>>]
              IN
              msgsPrecommit'
                  = [
                    msgsPrecommit EXCEPT
                      ![round[p$17]] =
                        msgsPrecommit[round[p$17]] \union {(newMsg_si_24)}
                  ]
              /\ step' = [ step EXCEPT ![p$17] = "PRECOMMIT" ]
              /\ (<<localClock, realTime>>' = <<localClock, realTime>>
                /\ beginRound' := beginRound)
              /\ (round' := round
                /\ decision' := decision
                /\ lockedValue' := lockedValue
                /\ lockedRound' := lockedRound
                /\ validValue' := validValue
                /\ validRound' := validRound)
              /\ (msgsPropose' := msgsPropose
                /\ msgsPrevote' := msgsPrevote
                /\ proposalReceptionTime' := proposalReceptionTime)
              /\ action' = "OnQuorumOfNilPrevotes")
          \/ (\E r$56 \in 0 .. 3:
            r$56 > round[p$17]
              /\ LET (*
                @type: (() => Set([id: <<Str, Int, Int>>, proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]));
              *)
              RoundMsgs_si_3 ==
                (msgsPropose[r$56] \union msgsPrevote[r$56])
                  \union msgsPrecommit[r$56]
              IN
              \E MyEvidence$9 \in SUBSET RoundMsgs_si_3:
                LET (*
                  @type: (() => Set(Str));
                *)
                Faster_si_3 == { m$39["src"]: m$39 \in MyEvidence$9 }
                IN
                Cardinality((Faster_si_3)) >= 1 + 1
                  /\ evidence' = MyEvidence$9 \union evidence
                  /\ (step[p$17] /= "DECIDED"
                    /\ round' = [ round EXCEPT ![p$17] = r$56 ]
                    /\ step' = [ step EXCEPT ![p$17] = "PROPOSE" ]
                    /\ beginRound'
                      = [
                        beginRound EXCEPT
                          ![<<r$56, p$17>>] = localClock[p$17]
                      ])
                  /\ <<localClock, realTime>>' = <<localClock, realTime>>
                  /\ (decision' := decision
                    /\ lockedValue' := lockedValue
                    /\ lockedRound' := lockedRound
                    /\ validValue' := validValue
                    /\ validRound' := validRound)
                  /\ (msgsPropose' := msgsPropose
                    /\ msgsPrevote' := msgsPrevote
                    /\ msgsPrecommit' := msgsPrecommit
                    /\ proposalReceptionTime' := proposalReceptionTime)
                  /\ action' = "OnRoundCatchup")))

CInitPrimed ==
  Proposer' \in [(0 .. 3) -> ({ "c1", "c2" } \union { "f3", "f4" })]

InitPrimed ==
  round' = [ p$10 \in { "c1", "c2" } |-> 0 ]
    /\ localClock' \in [{ "c1", "c2" } -> (2 .. 2 + 2)]
    /\ realTime' = 0
    /\ step' = [ p$11 \in { "c1", "c2" } |-> "PROPOSE" ]
    /\ decision'
      = [ p$12 \in { "c1", "c2" } |-> <<<<"None", (-1), (-1)>>, (-1)>> ]
    /\ lockedValue' = [ p$13 \in { "c1", "c2" } |-> "None" ]
    /\ lockedRound' = [ p$14 \in { "c1", "c2" } |-> -1 ]
    /\ validValue' = [ p$15 \in { "c1", "c2" } |-> <<"None", (-1), (-1)>> ]
    /\ validRound' = [ p$16 \in { "c1", "c2" } |-> -1 ]
    /\ ((TRUE
        /\ (msgsPropose' \in [(0 .. 3) -> Gen(5)]
          /\ msgsPrevote' \in [(0 .. 3) -> Gen(5)]
          /\ msgsPrecommit' \in [(0 .. 3) -> Gen(5)]
          /\ ((\A r$43 \in 0 .. 3:
              msgsPropose'[r$43]
                  \subseteq [type: {"PROPOSAL"},
                    src: { "f3", "f4" },
                    round: 0 .. 3,
                    proposal:
                      ({ "v0", "v1" } \union {"v2"}) \X (0 .. 7) \X (0 .. 3),
                    validRound: 0 .. 3 \union {(-1)}]
                /\ (\A m$29 \in msgsPropose'[r$43]: r$43 = m$29["round"])))
          /\ ((\A r$44 \in 0 .. 3:
              msgsPrevote'[r$44]
                  \subseteq [type: {"PREVOTE"},
                    src: { "f3", "f4" },
                    round: 0 .. 3,
                    id: ({ "v0", "v1" } \union {"v2"}) \X (0 .. 7) \X (0 .. 3)]
                /\ (\A m$30 \in msgsPrevote'[r$44]: r$44 = m$30["round"])))
          /\ ((\A r$45 \in 0 .. 3:
              msgsPrecommit'[r$45]
                  \subseteq [type: {"PRECOMMIT"},
                    src: { "f3", "f4" },
                    round: 0 .. 3,
                    id: ({ "v0", "v1" } \union {"v2"}) \X (0 .. 7) \X (0 .. 3)]
                /\ (\A m$31 \in msgsPrecommit'[r$45]: r$45 = m$31["round"])))))
      \/ (~TRUE
        /\ msgsPropose' = [ r$46 \in 0 .. 3 |-> {} ]
        /\ msgsPrevote' = [ r$47 \in 0 .. 3 |-> {} ]
        /\ msgsPrecommit' = [ r$48 \in 0 .. 3 |-> {} ]))
    /\ proposalReceptionTime' = [ r_p$1 \in (0 .. 3) \X { "c1", "c2" } |-> -1 ]
    /\ evidence' = {}
    /\ action' = "Init"
    /\ beginRound'
      = [
        r_c$1 \in (0 .. 3) \X { "c1", "c2" } |->
          IF r_c$1[1] = 0 THEN localClock'[r_c$1[2]] ELSE 7
      ]

(*
  @type: Bool;
*)
VCInv_si_0 ==
  \A p$22 \in { "c1", "c2" }:
    \A q$6 \in { "c1", "c2" }:
      decision[p$22] /= <<<<"None", (-1), (-1)>>, (-1)>>
        /\ decision[q$6] /= <<<<"None", (-1), (-1)>>, (-1)>>
        => (\E v$10 \in { "v0", "v1" }:
          \E t$10 \in 0 .. 7:
            \E pr$5 \in 0 .. 3:
              \E r1$3 \in 0 .. 3:
                \E r2$3 \in 0 .. 3:
                  LET (*
                    @type: (() => <<Str, Int, Int>>);
                  *)
                  prop_si_8 == <<v$10, t$10, pr$5>>
                  IN
                  decision[p$22] = <<(prop_si_8), r1$3>>
                    /\ decision[q$6] = <<(prop_si_8), r2$3>>)

(*
  @type: Bool;
*)
VCNotInv_si_0 ==
  ~(\A p$22 \in { "c1", "c2" }:
    \A q$6 \in { "c1", "c2" }:
      decision[p$22] /= <<<<"None", (-1), (-1)>>, (-1)>>
        /\ decision[q$6] /= <<<<"None", (-1), (-1)>>, (-1)>>
        => (\E v$10 \in { "v0", "v1" }:
          \E t$10 \in 0 .. 7:
            \E pr$5 \in 0 .. 3:
              \E r1$3 \in 0 .. 3:
                \E r2$3 \in 0 .. 3:
                  LET (*
                    @type: (() => <<Str, Int, Int>>);
                  *)
                  prop_si_8 == <<v$10, t$10, pr$5>>
                  IN
                  decision[p$22] = <<(prop_si_8), r1$3>>
                    /\ decision[q$6] = <<(prop_si_8), r2$3>>))

(*
  @type: Bool;
*)
VCInv_si_1 ==
  \A p$23 \in { "c1", "c2" }:
    LET (*@type: (() => <<Str, Int, Int>>); *) prop_si_9 == decision[p$23][1] IN
    prop_si_9 /= <<"None", (-1), (-1)>> => (prop_si_9)[1] \in { "v0", "v1" }

(*
  @type: Bool;
*)
VCNotInv_si_1 ==
  ~(\A p$23 \in { "c1", "c2" }:
    LET (*@type: (() => <<Str, Int, Int>>); *) prop_si_9 == decision[p$23][1] IN
    prop_si_9 /= <<"None", (-1), (-1)>> => (prop_si_9)[1] \in { "v0", "v1" })

(*
  @type: Bool;
*)
VCInv_si_2 ==
  \A p$24 \in { "c1", "c2" }:
    \E v$11 \in { "v0", "v1" }:
      \E t$11 \in 0 .. 7:
        \E pr$6 \in 0 .. 3:
          \E dr$2 \in 0 .. 3:
            decision[p$24] = <<<<v$11, t$11, pr$6>>, dr$2>>
              => (\E q$7 \in { "c1", "c2" }:
                LET (*
                  @type: (() => Int);
                *)
                propRecvTime_si_2 == proposalReceptionTime[pr$6, q$7]
                IN
                beginRound[pr$6, q$7] <= propRecvTime_si_2
                  /\ beginRound[(pr$6 + 1), q$7] >= propRecvTime_si_2
                  /\ (propRecvTime_si_2 >= t$11 - 2
                    /\ propRecvTime_si_2 <= (t$11 + 2) + 2))

(*
  @type: Bool;
*)
VCNotInv_si_2 ==
  ~(\A p$24 \in { "c1", "c2" }:
    \E v$11 \in { "v0", "v1" }:
      \E t$11 \in 0 .. 7:
        \E pr$6 \in 0 .. 3:
          \E dr$2 \in 0 .. 3:
            decision[p$24] = <<<<v$11, t$11, pr$6>>, dr$2>>
              => (\E q$7 \in { "c1", "c2" }:
                LET (*
                  @type: (() => Int);
                *)
                propRecvTime_si_2 == proposalReceptionTime[pr$6, q$7]
                IN
                beginRound[pr$6, q$7] <= propRecvTime_si_2
                  /\ beginRound[(pr$6 + 1), q$7] >= propRecvTime_si_2
                  /\ (propRecvTime_si_2 >= t$11 - 2
                    /\ propRecvTime_si_2 <= (t$11 + 2) + 2)))

(*
  @type: Bool;
*)
VCInv_si_3 ==
  \A r$31 \in 0 .. 3 \ {3}:
    \A v$12 \in { "v0", "v1" }:
      LET (*@type: (() => Str); *) p_si_25 == Proposer[r$31] IN
      p_si_25 \in { "c1", "c2" }
        => (\E t$12 \in 0 .. 7:
          LET (*
            @type: (() => <<Str, Int, Int>>);
          *)
          prop_si_10 == <<v$12, t$12, r$31>>
          IN
          (\E msg$8 \in msgsPropose[r$31]:
              msg$8["proposal"] = prop_si_10
                /\ msg$8["src"] = p_si_25
                /\ msg$8["validRound"] = -1)
            /\ LET (*@type: (() => Int); *) tOffset_si_2 == (t$12 + 2) + 2 IN
            [
                r$32 \in 0 .. 3 |->
                  ApaFoldSet(LET (*
                    @type: ((Int, Int) => Int);
                  *)
                  Min2_si_4(a_si_7, b_si_7) == IF a$7 <= b$7 THEN a$7 ELSE b$7
                  IN
                  Min2$4, 7, { beginRound[r$32, p$26]: p$26 \in { "c1", "c2" } })
              ][
                r$31
              ]
                <= t$12
              /\ t$12
                <= [
                  r$33 \in 0 .. 3 |->
                    ApaFoldSet(LET (*
                      @type: ((Int, Int) => Int);
                    *)
                    Max2_si_4(a_si_8, b_si_8) == IF a$8 >= b$8 THEN a$8 ELSE b$8
                    IN
                    Max2$4, (-1), {
                      beginRound[r$33, p$27]:
                        p$27 \in { "c1", "c2" }
                    })
                ][
                  r$31
                ]
              /\ [
                r$34 \in 0 .. 3 |->
                  ApaFoldSet(LET (*
                    @type: ((Int, Int) => Int);
                  *)
                  Max2_si_5(a_si_9, b_si_9) == IF a$9 >= b$9 THEN a$9 ELSE b$9
                  IN
                  Max2$5, (-1), {
                    beginRound[r$34, p$28]:
                      p$28 \in { "c1", "c2" }
                  })
              ][
                r$31
              ]
                <= tOffset_si_2
              /\ tOffset_si_2
                <= [
                  r$35 \in 0 .. 3 |->
                    ApaFoldSet(LET (*
                      @type: ((Int, Int) => Int);
                    *)
                    Min2_si_5(a_si_10, b_si_10) ==
                      IF a$10 <= b$10 THEN a$10 ELSE b$10
                    IN
                    Min2$5, 7, {
                      beginRound[r$35, p$29]:
                        p$29 \in { "c1", "c2" }
                    })
                ][
                  (r$31 + 1)
                ]
            => (\A q$8 \in { "c1", "c2" }:
              LET (*
                @type: (() => <<<<Str, Int, Int>>, Int>>);
              *)
              dq_si_2 == decision[q$8]
              IN
              dq_si_2 /= <<<<"None", (-1), (-1)>>, (-1)>>
                => (dq_si_2)[1] = prop_si_10))

(*
  @type: Bool;
*)
VCNotInv_si_3 ==
  ~(\A r$31 \in 0 .. 3 \ {3}:
    \A v$12 \in { "v0", "v1" }:
      LET (*@type: (() => Str); *) p_si_25 == Proposer[r$31] IN
      p_si_25 \in { "c1", "c2" }
        => (\E t$12 \in 0 .. 7:
          LET (*
            @type: (() => <<Str, Int, Int>>);
          *)
          prop_si_10 == <<v$12, t$12, r$31>>
          IN
          (\E msg$8 \in msgsPropose[r$31]:
              msg$8["proposal"] = prop_si_10
                /\ msg$8["src"] = p_si_25
                /\ msg$8["validRound"] = -1)
            /\ LET (*@type: (() => Int); *) tOffset_si_2 == (t$12 + 2) + 2 IN
            [
                r$32 \in 0 .. 3 |->
                  ApaFoldSet(LET (*
                    @type: ((Int, Int) => Int);
                  *)
                  Min2_si_4(a_si_7, b_si_7) == IF a$7 <= b$7 THEN a$7 ELSE b$7
                  IN
                  Min2$4, 7, { beginRound[r$32, p$26]: p$26 \in { "c1", "c2" } })
              ][
                r$31
              ]
                <= t$12
              /\ t$12
                <= [
                  r$33 \in 0 .. 3 |->
                    ApaFoldSet(LET (*
                      @type: ((Int, Int) => Int);
                    *)
                    Max2_si_4(a_si_8, b_si_8) == IF a$8 >= b$8 THEN a$8 ELSE b$8
                    IN
                    Max2$4, (-1), {
                      beginRound[r$33, p$27]:
                        p$27 \in { "c1", "c2" }
                    })
                ][
                  r$31
                ]
              /\ [
                r$34 \in 0 .. 3 |->
                  ApaFoldSet(LET (*
                    @type: ((Int, Int) => Int);
                  *)
                  Max2_si_5(a_si_9, b_si_9) == IF a$9 >= b$9 THEN a$9 ELSE b$9
                  IN
                  Max2$5, (-1), {
                    beginRound[r$34, p$28]:
                      p$28 \in { "c1", "c2" }
                  })
              ][
                r$31
              ]
                <= tOffset_si_2
              /\ tOffset_si_2
                <= [
                  r$35 \in 0 .. 3 |->
                    ApaFoldSet(LET (*
                      @type: ((Int, Int) => Int);
                    *)
                    Min2_si_5(a_si_10, b_si_10) ==
                      IF a$10 <= b$10 THEN a$10 ELSE b$10
                    IN
                    Min2$5, 7, {
                      beginRound[r$35, p$29]:
                        p$29 \in { "c1", "c2" }
                    })
                ][
                  (r$31 + 1)
                ]
            => (\A q$8 \in { "c1", "c2" }:
              LET (*
                @type: (() => <<<<Str, Int, Int>>, Int>>);
              *)
              dq_si_2 == decision[q$8]
              IN
              dq_si_2 /= <<<<"None", (-1), (-1)>>, (-1)>>
                => (dq_si_2)[1] = prop_si_10)))

================================================================================
