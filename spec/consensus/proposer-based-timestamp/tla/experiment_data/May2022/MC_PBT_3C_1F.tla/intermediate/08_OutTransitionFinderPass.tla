---------------------- MODULE 08_OutTransitionFinderPass ----------------------

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
CInit_si_0 ==
  \E t_1n$1 \in [(0 .. 3) -> ({ "c1", "c2", "c3" } \union {"f4"})]:
    Proposer' := t_1n$1

(*
  @type: (() => Bool);
*)
Init_si_0000 ==
  round' := [ p$1 \in { "c1", "c2", "c3" } |-> 0 ]
    /\ (\E t_1o$1 \in [{ "c1", "c2", "c3" } -> (2 .. 2 + 2)]:
      localClock' := t_1o$1)
    /\ realTime' := 0
    /\ step' := [ p$2 \in { "c1", "c2", "c3" } |-> "PROPOSE" ]
    /\ decision'
      := [ p$3 \in { "c1", "c2", "c3" } |-> <<<<"None", (-1), (-1)>>, (-1)>> ]
    /\ lockedValue' := [ p$4 \in { "c1", "c2", "c3" } |-> "None" ]
    /\ lockedRound' := [ p$5 \in { "c1", "c2", "c3" } |-> -1 ]
    /\ validValue'
      := [ p$6 \in { "c1", "c2", "c3" } |-> <<"None", (-1), (-1)>> ]
    /\ validRound' := [ p$7 \in { "c1", "c2", "c3" } |-> -1 ]
    /\ (TRUE
      /\ ((\E t_1p$1 \in [(0 .. 3) -> Gen(5)]: msgsPropose' := t_1p$1)
        /\ (\E t_1q$1 \in [(0 .. 3) -> Gen(5)]: msgsPrevote' := t_1q$1)
        /\ (\E t_1r$1 \in [(0 .. 3) -> Gen(5)]: msgsPrecommit' := t_1r$1)
        /\ ((\A r$1 \in 0 .. 3:
            (\A t_20$1 \in msgsPropose'[r$1]:
                t_20$1
                  \in {
                    [type |-> t_1v$1,
                      src |-> t_1w$1,
                      round |-> t_1x$1,
                      proposal |-> t_1y$1,
                      validRound |-> t_1z$1]:
                      t_1v$1 \in {"PROPOSAL"},
                      t_1w$1 \in {"f4"},
                      t_1x$1 \in 0 .. 3,
                      t_1y$1 \in
                        {
                          <<t_1s$1, t_1t$1, t_1u$1>>:
                            t_1s$1 \in { "v0", "v1" } \union {"v2"},
                            t_1t$1 \in 0 .. 7,
                            t_1u$1 \in 0 .. 3
                        },
                      t_1z$1 \in 0 .. 3 \union {(-1)}
                  })
              /\ (\A m$1 \in msgsPropose'[r$1]: r$1 = m$1["round"])))
        /\ ((\A r$2 \in 0 .. 3:
            (\A t_28$1 \in msgsPrevote'[r$2]:
                t_28$1
                  \in {
                    [type |-> t_24$1,
                      src |-> t_25$1,
                      round |-> t_26$1,
                      id |-> t_27$1]:
                      t_24$1 \in {"PREVOTE"},
                      t_25$1 \in {"f4"},
                      t_26$1 \in 0 .. 3,
                      t_27$1 \in
                        {
                          <<t_21$1, t_22$1, t_23$1>>:
                            t_21$1 \in { "v0", "v1" } \union {"v2"},
                            t_22$1 \in 0 .. 7,
                            t_23$1 \in 0 .. 3
                        }
                  })
              /\ (\A m$2 \in msgsPrevote'[r$2]: r$2 = m$2["round"])))
        /\ ((\A r$3 \in 0 .. 3:
            (\A t_2g$1 \in msgsPrecommit'[r$3]:
                t_2g$1
                  \in {
                    [type |-> t_2c$1,
                      src |-> t_2d$1,
                      round |-> t_2e$1,
                      id |-> t_2f$1]:
                      t_2c$1 \in {"PRECOMMIT"},
                      t_2d$1 \in {"f4"},
                      t_2e$1 \in 0 .. 3,
                      t_2f$1 \in
                        {
                          <<t_29$1, t_2a$1, t_2b$1>>:
                            t_29$1 \in { "v0", "v1" } \union {"v2"},
                            t_2a$1 \in 0 .. 7,
                            t_2b$1 \in 0 .. 3
                        }
                  })
              /\ (\A m$3 \in msgsPrecommit'[r$3]: r$3 = m$3["round"])))))
    /\ proposalReceptionTime'
      := [
        r_p$1 \in
          {
            <<t_2h$1, t_2i$1>>:
              t_2h$1 \in 0 .. 3,
              t_2i$1 \in { "c1", "c2", "c3" }
          } |->
          -1
      ]
    /\ evidence' := {}
    /\ action' := "Init"
    /\ beginRound'
      := [
        r_c$1 \in
          {
            <<t_2j$1, t_2k$1>>:
              t_2j$1 \in 0 .. 3,
              t_2k$1 \in { "c1", "c2", "c3" }
          } |->
          IF r_c$1[1] = 0 THEN localClock'[r_c$1[2]] ELSE 7
      ]

(*
  @type: (() => Bool);
*)
Init_si_0001 ==
  round' := [ p$8 \in { "c1", "c2", "c3" } |-> 0 ]
    /\ (\E t_1o$2 \in [{ "c1", "c2", "c3" } -> (2 .. 2 + 2)]:
      localClock' := t_1o$2)
    /\ realTime' := 0
    /\ step' := [ p$9 \in { "c1", "c2", "c3" } |-> "PROPOSE" ]
    /\ decision'
      := [ p$10 \in { "c1", "c2", "c3" } |-> <<<<"None", (-1), (-1)>>, (-1)>> ]
    /\ lockedValue' := [ p$11 \in { "c1", "c2", "c3" } |-> "None" ]
    /\ lockedRound' := [ p$12 \in { "c1", "c2", "c3" } |-> -1 ]
    /\ validValue'
      := [ p$13 \in { "c1", "c2", "c3" } |-> <<"None", (-1), (-1)>> ]
    /\ validRound' := [ p$14 \in { "c1", "c2", "c3" } |-> -1 ]
    /\ (FALSE
      /\ msgsPropose' := [ r$4 \in 0 .. 3 |-> {} ]
      /\ msgsPrevote' := [ r$5 \in 0 .. 3 |-> {} ]
      /\ msgsPrecommit' := [ r$6 \in 0 .. 3 |-> {} ])
    /\ proposalReceptionTime'
      := [
        r_p$2 \in
          {
            <<t_2h$2, t_2i$2>>:
              t_2h$2 \in 0 .. 3,
              t_2i$2 \in { "c1", "c2", "c3" }
          } |->
          -1
      ]
    /\ evidence' := {}
    /\ action' := "Init"
    /\ beginRound'
      := [
        r_c$2 \in
          {
            <<t_2j$2, t_2k$2>>:
              t_2j$2 \in 0 .. 3,
              t_2k$2 \in { "c1", "c2", "c3" }
          } |->
          IF r_c$2[1] = 0 THEN localClock'[r_c$2[2]] ELSE 7
      ]

(*
  @type: (() => Bool);
*)
Next_si_0000 ==
  realTime < 7
    /\ (\E t$1 \in 0 .. 7:
      t$1 > realTime
        /\ realTime' := t$1
        /\ localClock'
          := [
            p$15 \in { "c1", "c2", "c3" } |->
              localClock[p$15] + t$1 - realTime
          ])
    /\ ((round' := round
        /\ step' := step
        /\ decision' := decision
        /\ lockedValue' := lockedValue
        /\ lockedRound' := lockedRound
        /\ validValue' := validValue
        /\ validRound' := validRound)
      /\ (msgsPropose' := msgsPropose
        /\ msgsPrevote' := msgsPrevote
        /\ msgsPrecommit' := msgsPrecommit
        /\ evidence' := evidence
        /\ proposalReceptionTime' := proposalReceptionTime)
      /\ beginRound' := beginRound)
    /\ action' := "AdvanceRealTime"

(*
  @type: (() => Bool);
*)
Next_si_0001 ==
  (\A p$16 \in { "c1", "c2", "c3" }:
      \A q$1 \in { "c1", "c2", "c3" }:
        p$16 = q$1
          \/ ((localClock[p$16] >= localClock[q$1]
              /\ localClock[p$16] - localClock[q$1] < 2)
            \/ (localClock[p$16] < localClock[q$1]
              /\ localClock[q$1] - localClock[p$16] < 2)))
    /\ (\E p$17 \in { "c1", "c2", "c3" }:
      step[p$17] = "PREVOTE"
        /\ LET (*
          @type: (() => Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]));
        *)
        PV_si_1 ==
          {
            m$4 \in msgsPrevote[round[p$17]]:
              m$4["id"] = <<"None", (-1), (-1)>>
          }
        IN
        Cardinality((PV_si_1)) >= 2 * 1 + 1
          /\ evidence' := (PV_si_1 \union evidence)
          /\ LET (*
            @type: (() => [id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]);
          *)
          newMsg_si_1 ==
            [type |-> "PRECOMMIT",
              src |-> p$17,
              round |-> round[p$17],
              id |-> <<"None", (-1), (-1)>>]
          IN
          msgsPrecommit'
              := [
                msgsPrecommit EXCEPT
                  ![round[p$17]] =
                    msgsPrecommit[round[p$17]] \union {(newMsg_si_1)}
              ]
          /\ step' := [ step EXCEPT ![p$17] = "PRECOMMIT" ]
          /\ ((localClock' := localClock /\ realTime' := realTime)
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
          /\ action' := "OnQuorumOfNilPrevotes")

(*
  @type: (() => Bool);
*)
Next_si_0002 ==
  FALSE
    /\ action' := "FaultyBroadcast"
    /\ (\E r$7 \in 0 .. 3:
      \E msgs$1 \in SUBSET ({
        [type |-> t_16$1, src |-> t_17$1, round |-> t_18$1, id |-> t_19$1]:
          t_16$1 \in {"PREVOTE"},
          t_17$1 \in {"f4"},
          t_18$1 \in {r$7},
          t_19$1 \in
            {
              <<t_13$1, t_14$1, t_15$1>>:
                t_13$1 \in { "v0", "v1" } \union {"v2"},
                t_14$1 \in 0 .. 7,
                t_15$1 \in 0 .. 3
            }
      }):
        msgsPrevote'
            := [ msgsPrevote EXCEPT ![r$7] = msgsPrevote[r$7] \union msgs$1 ]
          /\ ((round' := round
              /\ step' := step
              /\ decision' := decision
              /\ lockedValue' := lockedValue
              /\ lockedRound' := lockedRound
              /\ validValue' := validValue
              /\ validRound' := validRound)
            /\ (localClock' := localClock /\ realTime' := realTime)
            /\ beginRound' := beginRound)
          /\ (msgsPropose' := msgsPropose
            /\ msgsPrecommit' := msgsPrecommit
            /\ evidence' := evidence
            /\ proposalReceptionTime' := proposalReceptionTime))

(*
  @type: (() => Bool);
*)
Next_si_0003 ==
  (\A p$18 \in { "c1", "c2", "c3" }:
      \A q$2 \in { "c1", "c2", "c3" }:
        p$18 = q$2
          \/ ((localClock[p$18] >= localClock[q$2]
              /\ localClock[p$18] - localClock[q$2] < 2)
            \/ (localClock[p$18] < localClock[q$2]
              /\ localClock[q$2] - localClock[p$18] < 2)))
    /\ (\E p$19 \in { "c1", "c2", "c3" }:
      step[p$19] = "PROPOSE"
        /\ ~(p$19 = Proposer[round[p$19]])
        /\ LET (*
          @type: (() => [id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]);
        *)
        newMsg_si_2 ==
          [type |-> "PREVOTE",
            src |-> p$19,
            round |-> round[p$19],
            id |-> <<"None", (-1), (-1)>>]
        IN
        msgsPrevote'
            := [
              msgsPrevote EXCEPT
                ![round[p$19]] = msgsPrevote[round[p$19]] \union {(newMsg_si_2)}
            ]
        /\ step' := [ step EXCEPT ![p$19] = "PREVOTE" ]
        /\ ((localClock' := localClock /\ realTime' := realTime)
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
        /\ action' := "OnTimeoutPropose")

(*
  @type: (() => Bool);
*)
Next_si_0004 ==
  FALSE
    /\ action' := "FaultyBroadcast"
    /\ (\E r$8 \in 0 .. 3:
      \E msgs$2 \in SUBSET ({
        [type |-> t_1d$1, src |-> t_1e$1, round |-> t_1f$1, id |-> t_1g$1]:
          t_1d$1 \in {"PRECOMMIT"},
          t_1e$1 \in {"f4"},
          t_1f$1 \in {r$8},
          t_1g$1 \in
            {
              <<t_1a$1, t_1b$1, t_1c$1>>:
                t_1a$1 \in { "v0", "v1" } \union {"v2"},
                t_1b$1 \in 0 .. 7,
                t_1c$1 \in 0 .. 3
            }
      }):
        msgsPrecommit'
            := [
              msgsPrecommit EXCEPT
                ![r$8] = msgsPrecommit[r$8] \union msgs$2
            ]
          /\ ((round' := round
              /\ step' := step
              /\ decision' := decision
              /\ lockedValue' := lockedValue
              /\ lockedRound' := lockedRound
              /\ validValue' := validValue
              /\ validRound' := validRound)
            /\ (localClock' := localClock /\ realTime' := realTime)
            /\ beginRound' := beginRound)
          /\ (msgsPropose' := msgsPropose
            /\ msgsPrevote' := msgsPrevote
            /\ evidence' := evidence
            /\ proposalReceptionTime' := proposalReceptionTime))

(*
  @type: (() => Bool);
*)
Next_si_0005 ==
  (\A p$20 \in { "c1", "c2", "c3" }:
      \A q$3 \in { "c1", "c2", "c3" }:
        p$20 = q$3
          \/ ((localClock[p$20] >= localClock[q$3]
              /\ localClock[p$20] - localClock[q$3] < 2)
            \/ (localClock[p$20] < localClock[q$3]
              /\ localClock[q$3] - localClock[p$20] < 2)))
    /\ (\E p$21 \in { "c1", "c2", "c3" }:
      \E v$1 \in { "v0", "v1" }:
        \E t$2 \in 0 .. 7:
          \E vr$1 \in 0 .. 3 \union {(-1)}:
            LET (*@type: (() => Int); *) r_si_9 == round[p$21] IN
            LET (*
              @type: (() => <<Str, Int, Int>>);
            *)
            prop_si_1 == <<v$1, t$2, (r_si_9)>>
            IN
            step[p$21] \in { "PREVOTE", "PRECOMMIT" }
              /\ LET (*
                @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
              *)
              msg_si_1 ==
                [type |-> "PROPOSAL",
                  src |-> Proposer[(r_si_9)],
                  round |-> r_si_9,
                  proposal |-> prop_si_1,
                  validRound |-> vr$1]
              IN
              msg_si_1 \in msgsPropose[(r_si_9)]
                /\ LET (*
                  @type: (() => Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]));
                *)
                PV_si_2 ==
                  { m$5 \in msgsPrevote[(r_si_9)]: m$5["id"] = prop_si_1 }
                IN
                Cardinality((PV_si_2)) >= 2 * 1 + 1
                  /\ evidence'
                    := ((PV_si_2 \union {(msg_si_1)}) \union evidence)
              /\ (step[p$21] = "PREVOTE"
                /\ (lockedValue' := [ lockedValue EXCEPT ![p$21] = v$1 ]
                  /\ lockedRound' := [ lockedRound EXCEPT ![p$21] = r_si_9 ]
                  /\ LET (*
                    @type: (() => [id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]);
                  *)
                  newMsg_si_3 ==
                    [type |-> "PRECOMMIT",
                      src |-> p$21,
                      round |-> r_si_9,
                      id |-> prop_si_1]
                  IN
                  msgsPrecommit'
                      := [
                        msgsPrecommit EXCEPT
                          ![r_si_9] =
                            msgsPrecommit[(r_si_9)] \union {(newMsg_si_3)}
                      ]
                  /\ step' := [ step EXCEPT ![p$21] = "PRECOMMIT" ]))
              /\ validValue' := [ validValue EXCEPT ![p$21] = prop_si_1 ]
              /\ validRound' := [ validRound EXCEPT ![p$21] = r_si_9 ]
              /\ ((localClock' := localClock /\ realTime' := realTime)
                /\ beginRound' := beginRound)
              /\ (round' := round /\ decision' := decision)
              /\ (msgsPropose' := msgsPropose
                /\ msgsPrevote' := msgsPrevote
                /\ proposalReceptionTime' := proposalReceptionTime)
              /\ action' := "UponProposalInPrevoteOrCommitAndPrevote")

(*
  @type: (() => Bool);
*)
Next_si_0006 ==
  (\A p$22 \in { "c1", "c2", "c3" }:
      \A q$4 \in { "c1", "c2", "c3" }:
        p$22 = q$4
          \/ ((localClock[p$22] >= localClock[q$4]
              /\ localClock[p$22] - localClock[q$4] < 2)
            \/ (localClock[p$22] < localClock[q$4]
              /\ localClock[q$4] - localClock[p$22] < 2)))
    /\ (\E p$23 \in { "c1", "c2", "c3" }:
      \E r$10 \in 0 .. 3:
        r$10 > round[p$23]
          /\ LET (*
            @type: (() => Set([id: <<Str, Int, Int>>, proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]));
          *)
          RoundMsgs_si_1 ==
            (msgsPropose[r$10] \union msgsPrevote[r$10])
              \union msgsPrecommit[r$10]
          IN
          \E MyEvidence$1 \in SUBSET RoundMsgs_si_1:
            LET (*
              @type: (() => Set(Str));
            *)
            Faster_si_1 == { m$6["src"]: m$6 \in MyEvidence$1 }
            IN
            Cardinality((Faster_si_1)) >= 1 + 1
              /\ evidence' := (MyEvidence$1 \union evidence)
              /\ (~(step[p$23] = "DECIDED")
                /\ round' := [ round EXCEPT ![p$23] = r$10 ]
                /\ step' := [ step EXCEPT ![p$23] = "PROPOSE" ]
                /\ beginRound'
                  := [ beginRound EXCEPT ![<<r$10, p$23>>] = localClock[p$23] ])
              /\ (localClock' := localClock /\ realTime' := realTime)
              /\ (decision' := decision
                /\ lockedValue' := lockedValue
                /\ lockedRound' := lockedRound
                /\ validValue' := validValue
                /\ validRound' := validRound)
              /\ (msgsPropose' := msgsPropose
                /\ msgsPrevote' := msgsPrevote
                /\ msgsPrecommit' := msgsPrecommit
                /\ proposalReceptionTime' := proposalReceptionTime)
              /\ action' := "OnRoundCatchup")

(*
  @type: (() => Bool);
*)
Next_si_0007 ==
  (\A p$24 \in { "c1", "c2", "c3" }:
      \A q$5 \in { "c1", "c2", "c3" }:
        p$24 = q$5
          \/ ((localClock[p$24] >= localClock[q$5]
              /\ localClock[p$24] - localClock[q$5] < 2)
            \/ (localClock[p$24] < localClock[q$5]
              /\ localClock[q$5] - localClock[p$24] < 2)))
    /\ (\E p$25 \in { "c1", "c2", "c3" }:
      decision[p$25] = <<<<"None", (-1), (-1)>>, (-1)>>
        /\ (\E v$2 \in { "v0", "v1" }:
          \E t$3 \in 0 .. 7:
            \E r$11 \in 0 .. 3:
              \E pr$1 \in 0 .. 3:
                \E vr$2 \in 0 .. 3 \union {(-1)}:
                  LET (*
                    @type: (() => <<Str, Int, Int>>);
                  *)
                  prop_si_2 == <<v$2, t$3, pr$1>>
                  IN
                  LET (*
                      @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
                    *)
                    msg_si_2 ==
                      [type |-> "PROPOSAL",
                        src |-> Proposer[r$11],
                        round |-> r$11,
                        proposal |-> prop_si_2,
                        validRound |-> vr$2]
                    IN
                    msg_si_2 \in msgsPropose[r$11]
                      /\ ~(proposalReceptionTime[r$11, p$25] = -1)
                      /\ LET (*
                        @type: (() => Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]));
                      *)
                      PV_si_3 ==
                        { m$7 \in msgsPrecommit[r$11]: m$7["id"] = prop_si_2 }
                      IN
                      Cardinality((PV_si_3)) >= 2 * 1 + 1
                        /\ evidence'
                          := ((PV_si_3 \union {(msg_si_2)}) \union evidence)
                        /\ decision'
                          := [ decision EXCEPT ![p$25] = <<(prop_si_2), r$11>> ]
                        /\ step' := [ step EXCEPT ![p$25] = "DECIDED" ]
                        /\ (localClock' := localClock /\ realTime' := realTime)
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
                        /\ action' := "UponProposalInPrecommitNoDecision"))

(*
  @type: (() => Bool);
*)
Next_si_0008 ==
  (\A p$26 \in { "c1", "c2", "c3" }:
      \A q$6 \in { "c1", "c2", "c3" }:
        p$26 = q$6
          \/ ((localClock[p$26] >= localClock[q$6]
              /\ localClock[p$26] - localClock[q$6] < 2)
            \/ (localClock[p$26] < localClock[q$6]
              /\ localClock[q$6] - localClock[p$26] < 2)))
    /\ (\E p$27 \in { "c1", "c2", "c3" }:
      \E v$3 \in { "v0", "v1" } \union {"v2"}:
        \E t$4 \in 0 .. 7:
          \E vr$3 \in 0 .. 3:
            \E pr$2 \in 0 .. 3:
              LET (*@type: (() => Int); *) r_si_12 == round[p$27] IN
              LET (*
                @type: (() => <<Str, Int, Int>>);
              *)
              prop_si_3 == <<v$3, t$4, pr$2>>
              IN
              ((step[p$27] = "PROPOSE" /\ 0 <= vr$3) /\ vr$3 < r_si_12)
                /\ pr$2 <= vr$3
                /\ LET (*
                  @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
                *)
                msg_si_3 ==
                  [type |-> "PROPOSAL",
                    src |-> Proposer[(r_si_12)],
                    round |-> r_si_12,
                    proposal |-> prop_si_3,
                    validRound |-> vr$3]
                IN
                msg_si_3 \in msgsPropose[(r_si_12)]
                  /\ LET (*
                    @type: (() => Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]));
                  *)
                  PV_si_4 ==
                    { m$8 \in msgsPrevote[vr$3]: m$8["id"] = prop_si_3 }
                  IN
                  Cardinality((PV_si_4)) >= 2 * 1 + 1
                    /\ evidence'
                      := ((PV_si_4 \union {(msg_si_3)}) \union evidence)
                /\ LET (*
                  @type: (() => <<Str, Int, Int>>);
                *)
                mid_si_1 ==
                  IF prop_si_3
                      \in {
                        <<t_1k$1, t_1l$1, t_1m$1>>:
                          t_1k$1 \in { "v0", "v1" },
                          t_1l$1 \in 2 .. 7,
                          t_1m$1 \in 0 .. 3
                      }
                    /\ (lockedRound[p$27] <= vr$3 \/ lockedValue[p$27] = v$3)
                  THEN prop_si_3
                  ELSE <<"None", (-1), (-1)>>
                IN
                LET (*
                  @type: (() => [id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]);
                *)
                newMsg_si_4 ==
                  [type |-> "PREVOTE",
                    src |-> p$27,
                    round |-> r_si_12,
                    id |-> mid_si_1]
                IN
                msgsPrevote'
                    := [
                      msgsPrevote EXCEPT
                        ![r_si_12] =
                          msgsPrevote[(r_si_12)] \union {(newMsg_si_4)}
                    ]
                /\ step' := [ step EXCEPT ![p$27] = "PREVOTE" ]
                /\ ((localClock' := localClock /\ realTime' := realTime)
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
                /\ action' := "UponProposalInProposeAndPrevote")

(*
  @type: (() => Bool);
*)
Next_si_0009 ==
  (\A p$28 \in { "c1", "c2", "c3" }:
      \A q$7 \in { "c1", "c2", "c3" }:
        p$28 = q$7
          \/ ((localClock[p$28] >= localClock[q$7]
              /\ localClock[p$28] - localClock[q$7] < 2)
            \/ (localClock[p$28] < localClock[q$7]
              /\ localClock[q$7] - localClock[p$28] < 2)))
    /\ (\E p$29 \in { "c1", "c2", "c3" }:
      step[p$29] = "PREVOTE"
        /\ (\E MyEvidence$2 \in SUBSET (msgsPrevote[round[p$29]]):
          LET (*
            @type: (() => Set(Str));
          *)
          Voters_si_1 == { m$9["src"]: m$9 \in MyEvidence$2 }
          IN
          Cardinality((Voters_si_1)) >= 2 * 1 + 1
            /\ evidence' := (MyEvidence$2 \union evidence)
            /\ LET (*
              @type: (() => [id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]);
            *)
            newMsg_si_5 ==
              [type |-> "PRECOMMIT",
                src |-> p$29,
                round |-> round[p$29],
                id |-> <<"None", (-1), (-1)>>]
            IN
            msgsPrecommit'
                := [
                  msgsPrecommit EXCEPT
                    ![round[p$29]] =
                      msgsPrecommit[round[p$29]] \union {(newMsg_si_5)}
                ]
            /\ step' := [ step EXCEPT ![p$29] = "PRECOMMIT" ]
            /\ ((localClock' := localClock /\ realTime' := realTime)
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
            /\ action' := "UponQuorumOfPrevotesAny"))

(*
  @type: (() => Bool);
*)
Next_si_0010 ==
  (\A p$30 \in { "c1", "c2", "c3" }:
      \A q$8 \in { "c1", "c2", "c3" }:
        p$30 = q$8
          \/ ((localClock[p$30] >= localClock[q$8]
              /\ localClock[p$30] - localClock[q$8] < 2)
            \/ (localClock[p$30] < localClock[q$8]
              /\ localClock[q$8] - localClock[p$30] < 2)))
    /\ (\E p$31 \in { "c1", "c2", "c3" }:
      \E v$4 \in { "v0", "v1" } \union {"v2"}:
        \E t$5 \in 0 .. 7:
          LET (*@type: (() => Int); *) r_si_13 == round[p$31] IN
            LET (*
              @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
            *)
            msg_si_4 ==
              [type |-> "PROPOSAL",
                src |-> Proposer[round[p$31]],
                round |-> round[p$31],
                proposal |-> <<v$4, t$5, (r_si_13)>>,
                validRound |-> -1]
            IN
            msg_si_4 \in msgsPropose[round[p$31]]
              /\ proposalReceptionTime[(r_si_13), p$31] = -1
              /\ proposalReceptionTime'
                := [
                  proposalReceptionTime EXCEPT
                    ![<<(r_si_13), p$31>>] = localClock[p$31]
                ]
              /\ ((localClock' := localClock /\ realTime' := realTime)
                /\ (round' := round
                  /\ step' := step
                  /\ decision' := decision
                  /\ lockedValue' := lockedValue
                  /\ lockedRound' := lockedRound
                  /\ validValue' := validValue
                  /\ validRound' := validRound))
              /\ (msgsPropose' := msgsPropose
                /\ msgsPrevote' := msgsPrevote
                /\ msgsPrecommit' := msgsPrecommit
                /\ evidence' := evidence)
              /\ beginRound' := beginRound
              /\ action' := "ReceiveProposal")

(*
  @type: (() => Bool);
*)
Next_si_0011 ==
  (\A p$32 \in { "c1", "c2", "c3" }:
      \A q$9 \in { "c1", "c2", "c3" }:
        p$32 = q$9
          \/ ((localClock[p$32] >= localClock[q$9]
              /\ localClock[p$32] - localClock[q$9] < 2)
            \/ (localClock[p$32] < localClock[q$9]
              /\ localClock[q$9] - localClock[p$32] < 2)))
    /\ (\E p$33 \in { "c1", "c2", "c3" }:
      (\E MyEvidence$3 \in SUBSET (msgsPrecommit[round[p$33]]):
          LET (*
            @type: (() => Set(Str));
          *)
          Committers_si_1 == { m$10["src"]: m$10 \in MyEvidence$3 }
          IN
          Cardinality((Committers_si_1)) >= 2 * 1 + 1
            /\ evidence' := (MyEvidence$3 \union evidence)
            /\ round[p$33] + 1 \in 0 .. 3
            /\ (~(step[p$33] = "DECIDED")
              /\ round' := [ round EXCEPT ![p$33] = round[p$33] + 1 ]
              /\ step' := [ step EXCEPT ![p$33] = "PROPOSE" ]
              /\ beginRound'
                := [
                  beginRound EXCEPT
                    ![<<(round[p$33] + 1), p$33>>] = localClock[p$33]
                ])
            /\ (localClock' := localClock /\ realTime' := realTime)
            /\ (decision' := decision
              /\ lockedValue' := lockedValue
              /\ lockedRound' := lockedRound
              /\ validValue' := validValue
              /\ validRound' := validRound)
            /\ (msgsPropose' := msgsPropose
              /\ msgsPrevote' := msgsPrevote
              /\ msgsPrecommit' := msgsPrecommit
              /\ proposalReceptionTime' := proposalReceptionTime)
            /\ action' := "UponQuorumOfPrecommitsAny"))

(*
  @type: (() => Bool);
*)
Next_si_0012 ==
  (\A p$34 \in { "c1", "c2", "c3" }:
      \A q$10 \in { "c1", "c2", "c3" }:
        p$34 = q$10
          \/ ((localClock[p$34] >= localClock[q$10]
              /\ localClock[p$34] - localClock[q$10] < 2)
            \/ (localClock[p$34] < localClock[q$10]
              /\ localClock[q$10] - localClock[p$34] < 2)))
    /\ (\E p$35 \in { "c1", "c2", "c3" }:
      \E v$5 \in { "v0", "v1" } \union {"v2"}:
        \E t$6 \in 0 .. 7:
          LET (*@type: (() => Int); *) r_si_14 == round[p$35] IN
          LET (*
            @type: (() => <<Str, Int, Int>>);
          *)
          prop_si_4 == <<v$5, t$6, (r_si_14)>>
          IN
          step[p$35] = "PROPOSE"
            /\ LET (*
              @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
            *)
            msg_si_5 ==
              [type |-> "PROPOSAL",
                src |-> Proposer[(r_si_14)],
                round |-> r_si_14,
                proposal |-> prop_si_4,
                validRound |-> -1]
            IN
            evidence' := ({(msg_si_5)} \union evidence)
            /\ LET (*
              @type: (() => <<Str, Int, Int>>);
            *)
            mid_si_2 ==
              IF (proposalReceptionTime[(r_si_14), p$35] >= t$6 - 2
                  /\ proposalReceptionTime[(r_si_14), p$35] <= (t$6 + 2) + 2)
                /\ prop_si_4
                  \in {
                    <<t_1h$1, t_1i$1, t_1j$1>>:
                      t_1h$1 \in { "v0", "v1" },
                      t_1i$1 \in 2 .. 7,
                      t_1j$1 \in 0 .. 3
                  }
                /\ (lockedRound[p$35] = -1 \/ lockedValue[p$35] = v$5)
              THEN prop_si_4
              ELSE <<"None", (-1), (-1)>>
            IN
            LET (*
              @type: (() => [id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]);
            *)
            newMsg_si_6 ==
              [type |-> "PREVOTE",
                src |-> p$35,
                round |-> r_si_14,
                id |-> mid_si_2]
            IN
            msgsPrevote'
                := [
                  msgsPrevote EXCEPT
                    ![r_si_14] = msgsPrevote[(r_si_14)] \union {(newMsg_si_6)}
                ]
            /\ step' := [ step EXCEPT ![p$35] = "PREVOTE" ]
            /\ ((localClock' := localClock /\ realTime' := realTime)
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
            /\ action' := "UponProposalInPropose")

(*
  @type: (() => Bool);
*)
Next_si_0013 ==
  FALSE
    /\ action' := "FaultyBroadcast"
    /\ (\E r$15 \in 0 .. 3:
      \E msgs$3 \in SUBSET ({
        [type |-> t_y$1,
          src |-> t_z$1,
          round |-> t_10$1,
          proposal |-> t_11$1,
          validRound |-> t_12$1]:
          t_y$1 \in {"PROPOSAL"},
          t_z$1 \in {"f4"},
          t_10$1 \in {r$15},
          t_11$1 \in
            {
              <<t_v$1, t_w$1, t_x$1>>:
                t_v$1 \in { "v0", "v1" } \union {"v2"},
                t_w$1 \in 0 .. 7,
                t_x$1 \in 0 .. 3
            },
          t_12$1 \in 0 .. 3 \union {(-1)}
      }):
        msgsPropose'
            := [ msgsPropose EXCEPT ![r$15] = msgsPropose[r$15] \union msgs$3 ]
          /\ ((round' := round
              /\ step' := step
              /\ decision' := decision
              /\ lockedValue' := lockedValue
              /\ lockedRound' := lockedRound
              /\ validValue' := validValue
              /\ validRound' := validRound)
            /\ (localClock' := localClock /\ realTime' := realTime)
            /\ beginRound' := beginRound)
          /\ (msgsPrevote' := msgsPrevote
            /\ msgsPrecommit' := msgsPrecommit
            /\ evidence' := evidence
            /\ proposalReceptionTime' := proposalReceptionTime))

(*
  @type: (() => Bool);
*)
Next_si_0014 ==
  (\A p$36 \in { "c1", "c2", "c3" }:
      \A q$11 \in { "c1", "c2", "c3" }:
        p$36 = q$11
          \/ ((localClock[p$36] >= localClock[q$11]
              /\ localClock[p$36] - localClock[q$11] < 2)
            \/ (localClock[p$36] < localClock[q$11]
              /\ localClock[q$11] - localClock[p$36] < 2)))
    /\ (\E p$37 \in { "c1", "c2", "c3" }:
      LET (*@type: (() => Int); *) r_si_16 == round[p$37] IN
      p$37 = Proposer[(r_si_16)]
        /\ step[p$37] = "PROPOSE"
        /\ (\A m$11 \in msgsPropose[(r_si_16)]: ~(m$11["src"] = p$37))
        /\ (\E v$6 \in { "v0", "v1" }:
          LET (*
            @type: (() => <<Str, Int, Int>>);
          *)
          proposal_si_1 ==
            IF ~(validValue[p$37] = <<"None", (-1), (-1)>>)
            THEN validValue[p$37]
            ELSE <<v$6, localClock[p$37], (r_si_16)>>
          IN
          LET (*
              @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
            *)
            newMsg_si_7 ==
              [type |-> "PROPOSAL",
                src |-> p$37,
                round |-> r_si_16,
                proposal |-> proposal_si_1,
                validRound |-> validRound[p$37]]
            IN
            msgsPropose'
                := [
                  msgsPropose EXCEPT
                    ![r_si_16] = msgsPropose[(r_si_16)] \union {(newMsg_si_7)}
                ])
        /\ ((localClock' := localClock /\ realTime' := realTime)
          /\ (round' := round
            /\ step' := step
            /\ decision' := decision
            /\ lockedValue' := lockedValue
            /\ lockedRound' := lockedRound
            /\ validValue' := validValue
            /\ validRound' := validRound))
        /\ (msgsPrevote' := msgsPrevote
          /\ msgsPrecommit' := msgsPrecommit
          /\ evidence' := evidence
          /\ proposalReceptionTime' := proposalReceptionTime)
        /\ beginRound' := beginRound
        /\ action' := "InsertProposal")

(*
  @type: (() => Bool);
*)
Next_si_0015 ==
  (\A p$38 \in { "c1", "c2", "c3" }:
      \A q$12 \in { "c1", "c2", "c3" }:
        p$38 = q$12
          \/ ((localClock[p$38] >= localClock[q$12]
              /\ localClock[p$38] - localClock[q$12] < 2)
            \/ (localClock[p$38] < localClock[q$12]
              /\ localClock[q$12] - localClock[p$38] < 2)))
    /\ (\E p$39 \in { "c1", "c2", "c3" }:
      \E v$7 \in { "v0", "v1" }:
        \E t$7 \in 0 .. 7:
          \E vr$4 \in 0 .. 3 \union {(-1)}:
            LET (*@type: (() => Int); *) r_si_17 == round[p$39] IN
            LET (*
              @type: (() => <<Str, Int, Int>>);
            *)
            prop_si_5 == <<v$7, t$7, (r_si_17)>>
            IN
            step[p$39] \in { "PREVOTE", "PRECOMMIT" }
              /\ LET (*
                @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
              *)
              msg_si_6 ==
                [type |-> "PROPOSAL",
                  src |-> Proposer[(r_si_17)],
                  round |-> r_si_17,
                  proposal |-> prop_si_5,
                  validRound |-> vr$4]
              IN
              msg_si_6 \in msgsPropose[(r_si_17)]
                /\ LET (*
                  @type: (() => Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]));
                *)
                PV_si_5 ==
                  { m$12 \in msgsPrevote[(r_si_17)]: m$12["id"] = prop_si_5 }
                IN
                Cardinality((PV_si_5)) >= 2 * 1 + 1
                  /\ evidence'
                    := ((PV_si_5 \union {(msg_si_6)}) \union evidence)
              /\ (~(step[p$39] = "PREVOTE")
                /\ (lockedValue' := lockedValue
                  /\ lockedRound' := lockedRound
                  /\ msgsPrecommit' := msgsPrecommit
                  /\ step' := step))
              /\ validValue' := [ validValue EXCEPT ![p$39] = prop_si_5 ]
              /\ validRound' := [ validRound EXCEPT ![p$39] = r_si_17 ]
              /\ ((localClock' := localClock /\ realTime' := realTime)
                /\ beginRound' := beginRound)
              /\ (round' := round /\ decision' := decision)
              /\ (msgsPropose' := msgsPropose
                /\ msgsPrevote' := msgsPrevote
                /\ proposalReceptionTime' := proposalReceptionTime)
              /\ action' := "UponProposalInPrevoteOrCommitAndPrevote")

(*
  @type: Bool;
*)
VCInv_si_0 ==
  \A p$40 \in { "c1", "c2", "c3" }:
    \A q$13 \in { "c1", "c2", "c3" }:
      (decision[p$40] = <<<<"None", (-1), (-1)>>, (-1)>>
          \/ decision[q$13] = <<<<"None", (-1), (-1)>>, (-1)>>)
        \/ (\E v$8 \in { "v0", "v1" }:
          \E t$8 \in 0 .. 7:
            \E pr$3 \in 0 .. 3:
              \E r1$1 \in 0 .. 3:
                \E r2$1 \in 0 .. 3:
                  LET (*
                    @type: (() => <<Str, Int, Int>>);
                  *)
                  prop_si_6 == <<v$8, t$8, pr$3>>
                  IN
                  decision[p$40] = <<(prop_si_6), r1$1>>
                    /\ decision[q$13] = <<(prop_si_6), r2$1>>)

(*
  @type: Bool;
*)
VCNotInv_si_0 ==
  \E p$41 \in { "c1", "c2", "c3" }:
    \E q$14 \in { "c1", "c2", "c3" }:
      (~(decision[p$41] = <<<<"None", (-1), (-1)>>, (-1)>>)
          /\ ~(decision[q$14] = <<<<"None", (-1), (-1)>>, (-1)>>))
        /\ (\A v$9 \in { "v0", "v1" }:
          \A t$9 \in 0 .. 7:
            \A pr$4 \in 0 .. 3:
              \A r1$2 \in 0 .. 3:
                \A r2$2 \in 0 .. 3:
                  LET (*
                    @type: (() => <<Str, Int, Int>>);
                  *)
                  prop_si_7 == <<v$9, t$9, pr$4>>
                  IN
                  ~(decision[p$41] = <<(prop_si_7), r1$2>>)
                    \/ ~(decision[q$14] = <<(prop_si_7), r2$2>>))

(*
  @type: Bool;
*)
VCInv_si_1 ==
  \A p$42 \in { "c1", "c2", "c3" }:
    LET (*@type: (() => <<Str, Int, Int>>); *) prop_si_8 == decision[p$42][1] IN
    prop_si_8 = <<"None", (-1), (-1)>> \/ (prop_si_8)[1] \in { "v0", "v1" }

(*
  @type: Bool;
*)
VCNotInv_si_1 ==
  \E p$43 \in { "c1", "c2", "c3" }:
    LET (*@type: (() => <<Str, Int, Int>>); *) prop_si_9 == decision[p$43][1] IN
    ~(prop_si_9 = <<"None", (-1), (-1)>>)
      /\ ~((prop_si_9)[1] \in { "v0", "v1" })

(*
  @type: Bool;
*)
VCInv_si_2 ==
  \A p$44 \in { "c1", "c2", "c3" }:
    \E v$10 \in { "v0", "v1" }:
      \E t$10 \in 0 .. 7:
        \E pr$5 \in 0 .. 3:
          \E dr$1 \in 0 .. 3:
            ~(decision[p$44] = <<<<v$10, t$10, pr$5>>, dr$1>>)
              \/ (\E q$15 \in { "c1", "c2", "c3" }:
                LET (*
                  @type: (() => Int);
                *)
                propRecvTime_si_1 == proposalReceptionTime[pr$5, q$15]
                IN
                beginRound[pr$5, q$15] <= propRecvTime_si_1
                  /\ beginRound[(pr$5 + 1), q$15] >= propRecvTime_si_1
                  /\ (propRecvTime_si_1 >= t$10 - 2
                    /\ propRecvTime_si_1 <= (t$10 + 2) + 2))

(*
  @type: Bool;
*)
VCNotInv_si_2 ==
  \E p$45 \in { "c1", "c2", "c3" }:
    \A v$11 \in { "v0", "v1" }:
      \A t$11 \in 0 .. 7:
        \A pr$6 \in 0 .. 3:
          \A dr$2 \in 0 .. 3:
            decision[p$45] = <<<<v$11, t$11, pr$6>>, dr$2>>
              /\ (\A q$16 \in { "c1", "c2", "c3" }:
                LET (*
                  @type: (() => Int);
                *)
                propRecvTime_si_2 == proposalReceptionTime[pr$6, q$16]
                IN
                beginRound[pr$6, q$16] > propRecvTime_si_2
                  \/ beginRound[(pr$6 + 1), q$16] < propRecvTime_si_2
                  \/ (propRecvTime_si_2 < t$11 - 2
                    \/ propRecvTime_si_2 > (t$11 + 2) + 2))

(*
  @type: Bool;
*)
VCInv_si_3 ==
  \A r$18 \in { t_2l$1 \in 0 .. 3: ~(t_2l$1 \in {3}) }:
    \A v$12 \in { "v0", "v1" }:
      LET (*@type: (() => Str); *) p_si_46 == Proposer[r$18] IN
      ~(p_si_46 \in { "c1", "c2", "c3" })
        \/ (\E t$12 \in 0 .. 7:
          LET (*
            @type: (() => <<Str, Int, Int>>);
          *)
          prop_si_10 == <<v$12, t$12, r$18>>
          IN
          ((\A msg$7 \in msgsPropose[r$18]:
                ~(msg$7["proposal"] = prop_si_10)
                  \/ ~(msg$7["src"] = p_si_46)
                  \/ ~(msg$7["validRound"] = -1))
              \/ LET (*@type: (() => Int); *) tOffset_si_1 == (t$12 + 2) + 2 IN
              [
                  r$19 \in 0 .. 3 |->
                    ApaFoldSet(LET (*
                      @type: ((Int, Int) => Int);
                    *)
                    Min2_si_1(a_si_1, b_si_1) == IF a$1 <= b$1 THEN a$1 ELSE b$1
                    IN
                    Min2$1, 7, {
                      beginRound[r$19, p$47]:
                        p$47 \in { "c1", "c2", "c3" }
                    })
                ][
                  r$18
                ]
                  > t$12
                \/ t$12
                  > [
                    r$20 \in 0 .. 3 |->
                      ApaFoldSet(LET (*
                        @type: ((Int, Int) => Int);
                      *)
                      Max2_si_1(a_si_2, b_si_2) ==
                        IF a$2 >= b$2 THEN a$2 ELSE b$2
                      IN
                      Max2$1, (-1), {
                        beginRound[r$20, p$48]:
                          p$48 \in { "c1", "c2", "c3" }
                      })
                  ][
                    r$18
                  ]
                \/ [
                  r$21 \in 0 .. 3 |->
                    ApaFoldSet(LET (*
                      @type: ((Int, Int) => Int);
                    *)
                    Max2_si_2(a_si_3, b_si_3) == IF a$3 >= b$3 THEN a$3 ELSE b$3
                    IN
                    Max2$2, (-1), {
                      beginRound[r$21, p$49]:
                        p$49 \in { "c1", "c2", "c3" }
                    })
                ][
                  r$18
                ]
                  > tOffset_si_1
                \/ tOffset_si_1
                  > [
                    r$22 \in 0 .. 3 |->
                      ApaFoldSet(LET (*
                        @type: ((Int, Int) => Int);
                      *)
                      Min2_si_2(a_si_4, b_si_4) ==
                        IF a$4 <= b$4 THEN a$4 ELSE b$4
                      IN
                      Min2$2, 7, {
                        beginRound[r$22, p$50]:
                          p$50 \in { "c1", "c2", "c3" }
                      })
                  ][
                    (r$18 + 1)
                  ])
            \/ (\A q$17 \in { "c1", "c2", "c3" }:
              LET (*
                @type: (() => <<<<Str, Int, Int>>, Int>>);
              *)
              dq_si_1 == decision[q$17]
              IN
              dq_si_1 = <<<<"None", (-1), (-1)>>, (-1)>>
                \/ (dq_si_1)[1] = prop_si_10))

(*
  @type: Bool;
*)
VCNotInv_si_3 ==
  \E r$23 \in { t_2m$1 \in 0 .. 3: ~(t_2m$1 \in {3}) }:
    \E v$13 \in { "v0", "v1" }:
      LET (*@type: (() => Str); *) p_si_51 == Proposer[r$23] IN
      p_si_51 \in { "c1", "c2", "c3" }
        /\ (\A t$13 \in 0 .. 7:
          LET (*
            @type: (() => <<Str, Int, Int>>);
          *)
          prop_si_11 == <<v$13, t$13, r$23>>
          IN
          ((\E msg$8 \in msgsPropose[r$23]:
                msg$8["proposal"] = prop_si_11
                  /\ msg$8["src"] = p_si_51
                  /\ msg$8["validRound"] = -1)
              /\ LET (*@type: (() => Int); *) tOffset_si_2 == (t$13 + 2) + 2 IN
              [
                  r$24 \in 0 .. 3 |->
                    ApaFoldSet(LET (*
                      @type: ((Int, Int) => Int);
                    *)
                    Min2_si_3(a_si_5, b_si_5) == IF a$5 <= b$5 THEN a$5 ELSE b$5
                    IN
                    Min2$3, 7, {
                      beginRound[r$24, p$52]:
                        p$52 \in { "c1", "c2", "c3" }
                    })
                ][
                  r$23
                ]
                  <= t$13
                /\ t$13
                  <= [
                    r$25 \in 0 .. 3 |->
                      ApaFoldSet(LET (*
                        @type: ((Int, Int) => Int);
                      *)
                      Max2_si_3(a_si_6, b_si_6) ==
                        IF a$6 >= b$6 THEN a$6 ELSE b$6
                      IN
                      Max2$3, (-1), {
                        beginRound[r$25, p$53]:
                          p$53 \in { "c1", "c2", "c3" }
                      })
                  ][
                    r$23
                  ]
                /\ [
                  r$26 \in 0 .. 3 |->
                    ApaFoldSet(LET (*
                      @type: ((Int, Int) => Int);
                    *)
                    Max2_si_4(a_si_7, b_si_7) == IF a$7 >= b$7 THEN a$7 ELSE b$7
                    IN
                    Max2$4, (-1), {
                      beginRound[r$26, p$54]:
                        p$54 \in { "c1", "c2", "c3" }
                    })
                ][
                  r$23
                ]
                  <= tOffset_si_2
                /\ tOffset_si_2
                  <= [
                    r$27 \in 0 .. 3 |->
                      ApaFoldSet(LET (*
                        @type: ((Int, Int) => Int);
                      *)
                      Min2_si_4(a_si_8, b_si_8) ==
                        IF a$8 <= b$8 THEN a$8 ELSE b$8
                      IN
                      Min2$4, 7, {
                        beginRound[r$27, p$55]:
                          p$55 \in { "c1", "c2", "c3" }
                      })
                  ][
                    (r$23 + 1)
                  ])
            /\ (\E q$18 \in { "c1", "c2", "c3" }:
              LET (*
                @type: (() => <<<<Str, Int, Int>>, Int>>);
              *)
              dq_si_2 == decision[q$18]
              IN
              ~(dq_si_2 = <<<<"None", (-1), (-1)>>, (-1)>>)
                /\ ~((dq_si_2)[1] = prop_si_11)))

================================================================================
