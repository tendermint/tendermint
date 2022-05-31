------------------------ MODULE 07_OutPreprocessingPass ------------------------

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
CInit == Proposer \in [(0 .. 3) -> ({ "c1", "c2", "c3" } \union {"f4"})]

(*
  @type: (() => Bool);
*)
Inv ==
  (\A p$1 \in { "c1", "c2", "c3" }:
      \A q$1 \in { "c1", "c2", "c3" }:
        (decision[p$1] = <<<<"None", (-1), (-1)>>, (-1)>>
            \/ decision[q$1] = <<<<"None", (-1), (-1)>>, (-1)>>)
          \/ (\E v$1 \in { "v0", "v1" }:
            \E t$1 \in 0 .. 7:
              \E pr$1 \in 0 .. 3:
                \E r1$1 \in 0 .. 3:
                  \E r2$1 \in 0 .. 3:
                    LET (*
                      @type: (() => <<Str, Int, Int>>);
                    *)
                    prop_si_1 == <<v$1, t$1, pr$1>>
                    IN
                    decision[p$1] = <<(prop_si_1), r1$1>>
                      /\ decision[q$1] = <<(prop_si_1), r2$1>>))
    /\ (\A p$2 \in { "c1", "c2", "c3" }:
      LET (*
        @type: (() => <<Str, Int, Int>>);
      *)
      prop_si_2 == decision[p$2][1]
      IN
      prop_si_2 = <<"None", (-1), (-1)>> \/ (prop_si_2)[1] \in { "v0", "v1" })
    /\ (\A p$3 \in { "c1", "c2", "c3" }:
      \E v$2 \in { "v0", "v1" }:
        \E t$2 \in 0 .. 7:
          \E pr$2 \in 0 .. 3:
            \E dr$1 \in 0 .. 3:
              ~(decision[p$3] = <<<<v$2, t$2, pr$2>>, dr$1>>)
                \/ (\E q$2 \in { "c1", "c2", "c3" }:
                  LET (*
                    @type: (() => Int);
                  *)
                  propRecvTime_si_1 == proposalReceptionTime[pr$2, q$2]
                  IN
                  beginRound[pr$2, q$2] <= propRecvTime_si_1
                    /\ beginRound[(pr$2 + 1), q$2] >= propRecvTime_si_1
                    /\ (propRecvTime_si_1 >= t$2 - 2
                      /\ propRecvTime_si_1 <= (t$2 + 2) + 2)))
    /\ (\A r$1 \in { t_1$1 \in 0 .. 3: ~(t_1$1 \in {3}) }:
      \A v$3 \in { "v0", "v1" }:
        LET (*@type: (() => Str); *) p_si_4 == Proposer[r$1] IN
        ~(p_si_4 \in { "c1", "c2", "c3" })
          \/ (\E t$3 \in 0 .. 7:
            LET (*
              @type: (() => <<Str, Int, Int>>);
            *)
            prop_si_3 == <<v$3, t$3, r$1>>
            IN
            ((\A msg$1 \in msgsPropose[r$1]:
                  ~(msg$1["proposal"] = prop_si_3)
                    \/ ~(msg$1["src"] = p_si_4)
                    \/ ~(msg$1["validRound"] = -1))
                \/ LET (*@type: (() => Int); *) tOffset_si_1 == (t$3 + 2) + 2 IN
                [
                    r$2 \in 0 .. 3 |->
                      ApaFoldSet(LET (*
                        @type: ((Int, Int) => Int);
                      *)
                      Min2_si_1(a_si_1, b_si_1) ==
                        IF a$1 <= b$1 THEN a$1 ELSE b$1
                      IN
                      Min2$1, 7, {
                        beginRound[r$2, p$5]:
                          p$5 \in { "c1", "c2", "c3" }
                      })
                  ][
                    r$1
                  ]
                    > t$3
                  \/ t$3
                    > [
                      r$3 \in 0 .. 3 |->
                        ApaFoldSet(LET (*
                          @type: ((Int, Int) => Int);
                        *)
                        Max2_si_1(a_si_2, b_si_2) ==
                          IF a$2 >= b$2 THEN a$2 ELSE b$2
                        IN
                        Max2$1, (-1), {
                          beginRound[r$3, p$6]:
                            p$6 \in { "c1", "c2", "c3" }
                        })
                    ][
                      r$1
                    ]
                  \/ [
                    r$4 \in 0 .. 3 |->
                      ApaFoldSet(LET (*
                        @type: ((Int, Int) => Int);
                      *)
                      Max2_si_2(a_si_3, b_si_3) ==
                        IF a$3 >= b$3 THEN a$3 ELSE b$3
                      IN
                      Max2$2, (-1), {
                        beginRound[r$4, p$7]:
                          p$7 \in { "c1", "c2", "c3" }
                      })
                  ][
                    r$1
                  ]
                    > tOffset_si_1
                  \/ tOffset_si_1
                    > [
                      r$5 \in 0 .. 3 |->
                        ApaFoldSet(LET (*
                          @type: ((Int, Int) => Int);
                        *)
                        Min2_si_2(a_si_4, b_si_4) ==
                          IF a$4 <= b$4 THEN a$4 ELSE b$4
                        IN
                        Min2$2, 7, {
                          beginRound[r$5, p$8]:
                            p$8 \in { "c1", "c2", "c3" }
                        })
                    ][
                      (r$1 + 1)
                    ])
              \/ (\A q$3 \in { "c1", "c2", "c3" }:
                LET (*
                  @type: (() => <<<<Str, Int, Int>>, Int>>);
                *)
                dq_si_1 == decision[q$3]
                IN
                dq_si_1 = <<<<"None", (-1), (-1)>>, (-1)>>
                  \/ (dq_si_1)[1] = prop_si_3)))

(*
  @type: (() => Bool);
*)
Init ==
  round = [ p$9 \in { "c1", "c2", "c3" } |-> 0 ]
    /\ localClock \in [{ "c1", "c2", "c3" } -> (2 .. 2 + 2)]
    /\ realTime = 0
    /\ step = [ p$10 \in { "c1", "c2", "c3" } |-> "PROPOSE" ]
    /\ decision
      = [ p$11 \in { "c1", "c2", "c3" } |-> <<<<"None", (-1), (-1)>>, (-1)>> ]
    /\ lockedValue = [ p$12 \in { "c1", "c2", "c3" } |-> "None" ]
    /\ lockedRound = [ p$13 \in { "c1", "c2", "c3" } |-> -1 ]
    /\ validValue = [ p$14 \in { "c1", "c2", "c3" } |-> <<"None", (-1), (-1)>> ]
    /\ validRound = [ p$15 \in { "c1", "c2", "c3" } |-> -1 ]
    /\ ((TRUE
        /\ (msgsPropose \in [(0 .. 3) -> Gen(5)]
          /\ msgsPrevote \in [(0 .. 3) -> Gen(5)]
          /\ msgsPrecommit \in [(0 .. 3) -> Gen(5)]
          /\ ((\A r$6 \in 0 .. 3:
              (\A t_a$1 \in msgsPropose[r$6]:
                  t_a$1
                    \in {
                      [type |-> t_5$1,
                        src |-> t_6$1,
                        round |-> t_7$1,
                        proposal |-> t_8$1,
                        validRound |-> t_9$1]:
                        t_5$1 \in {"PROPOSAL"},
                        t_6$1 \in {"f4"},
                        t_7$1 \in 0 .. 3,
                        t_8$1 \in
                          {
                            <<t_2$1, t_3$1, t_4$1>>:
                              t_2$1 \in { "v0", "v1" } \union {"v2"},
                              t_3$1 \in 0 .. 7,
                              t_4$1 \in 0 .. 3
                          },
                        t_9$1 \in 0 .. 3 \union {(-1)}
                    })
                /\ (\A m$1 \in msgsPropose[r$6]: r$6 = m$1["round"])))
          /\ ((\A r$7 \in 0 .. 3:
              (\A t_i$1 \in msgsPrevote[r$7]:
                  t_i$1
                    \in {
                      [type |-> t_e$1,
                        src |-> t_f$1,
                        round |-> t_g$1,
                        id |-> t_h$1]:
                        t_e$1 \in {"PREVOTE"},
                        t_f$1 \in {"f4"},
                        t_g$1 \in 0 .. 3,
                        t_h$1 \in
                          {
                            <<t_b$1, t_c$1, t_d$1>>:
                              t_b$1 \in { "v0", "v1" } \union {"v2"},
                              t_c$1 \in 0 .. 7,
                              t_d$1 \in 0 .. 3
                          }
                    })
                /\ (\A m$2 \in msgsPrevote[r$7]: r$7 = m$2["round"])))
          /\ ((\A r$8 \in 0 .. 3:
              (\A t_q$1 \in msgsPrecommit[r$8]:
                  t_q$1
                    \in {
                      [type |-> t_m$1,
                        src |-> t_n$1,
                        round |-> t_o$1,
                        id |-> t_p$1]:
                        t_m$1 \in {"PRECOMMIT"},
                        t_n$1 \in {"f4"},
                        t_o$1 \in 0 .. 3,
                        t_p$1 \in
                          {
                            <<t_j$1, t_k$1, t_l$1>>:
                              t_j$1 \in { "v0", "v1" } \union {"v2"},
                              t_k$1 \in 0 .. 7,
                              t_l$1 \in 0 .. 3
                          }
                    })
                /\ (\A m$3 \in msgsPrecommit[r$8]: r$8 = m$3["round"])))))
      \/ (FALSE
        /\ msgsPropose = [ r$9 \in 0 .. 3 |-> {} ]
        /\ msgsPrevote = [ r$10 \in 0 .. 3 |-> {} ]
        /\ msgsPrecommit = [ r$11 \in 0 .. 3 |-> {} ]))
    /\ proposalReceptionTime
      = [
        r_p$1 \in
          { <<t_r$1, t_s$1>>: t_r$1 \in 0 .. 3, t_s$1 \in { "c1", "c2", "c3" } } |->
          -1
      ]
    /\ evidence = {}
    /\ action = "Init"
    /\ beginRound
      = [
        r_c$1 \in
          { <<t_t$1, t_u$1>>: t_t$1 \in 0 .. 3, t_u$1 \in { "c1", "c2", "c3" } } |->
          IF r_c$1[1] = 0 THEN localClock[r_c$1[2]] ELSE 7
      ]

(*
  @type: (() => Bool);
*)
Next ==
  (realTime < 7
      /\ (\E t$4 \in 0 .. 7:
        t$4 > realTime
          /\ realTime' = t$4
          /\ localClock'
            = [
              p$16 \in { "c1", "c2", "c3" } |->
                localClock[p$16] + t$4 - realTime
            ])
      /\ ((round' = round
          /\ step' = step
          /\ decision' = decision
          /\ lockedValue' = lockedValue
          /\ lockedRound' = lockedRound
          /\ validValue' = validValue
          /\ validRound' = validRound)
        /\ (msgsPropose' = msgsPropose
          /\ msgsPrevote' = msgsPrevote
          /\ msgsPrecommit' = msgsPrecommit
          /\ evidence' = evidence
          /\ proposalReceptionTime' = proposalReceptionTime)
        /\ beginRound' := beginRound)
      /\ action' = "AdvanceRealTime")
    \/ (FALSE
      /\ action' = "FaultyBroadcast"
      /\ (\E r$12 \in 0 .. 3:
        (\E msgs$1 \in SUBSET ({
            [type |-> t_y$1,
              src |-> t_z$1,
              round |-> t_10$1,
              proposal |-> t_11$1,
              validRound |-> t_12$1]:
              t_y$1 \in {"PROPOSAL"},
              t_z$1 \in {"f4"},
              t_10$1 \in {r$12},
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
                = [
                  msgsPropose EXCEPT
                    ![r$12] = msgsPropose[r$12] \union msgs$1
                ]
              /\ ((round' = round
                  /\ step' = step
                  /\ decision' = decision
                  /\ lockedValue' = lockedValue
                  /\ lockedRound' = lockedRound
                  /\ validValue' = validValue
                  /\ validRound' = validRound)
                /\ (localClock' = localClock /\ realTime' = realTime)
                /\ beginRound' := beginRound)
              /\ (msgsPrevote' := msgsPrevote
                /\ msgsPrecommit' := msgsPrecommit
                /\ evidence' := evidence
                /\ proposalReceptionTime' := proposalReceptionTime))
          \/ (\E msgs$2 \in SUBSET ({
            [type |-> t_16$1, src |-> t_17$1, round |-> t_18$1, id |-> t_19$1]:
              t_16$1 \in {"PREVOTE"},
              t_17$1 \in {"f4"},
              t_18$1 \in {r$12},
              t_19$1 \in
                {
                  <<t_13$1, t_14$1, t_15$1>>:
                    t_13$1 \in { "v0", "v1" } \union {"v2"},
                    t_14$1 \in 0 .. 7,
                    t_15$1 \in 0 .. 3
                }
          }):
            msgsPrevote'
                = [
                  msgsPrevote EXCEPT
                    ![r$12] = msgsPrevote[r$12] \union msgs$2
                ]
              /\ ((round' = round
                  /\ step' = step
                  /\ decision' = decision
                  /\ lockedValue' = lockedValue
                  /\ lockedRound' = lockedRound
                  /\ validValue' = validValue
                  /\ validRound' = validRound)
                /\ (localClock' = localClock /\ realTime' = realTime)
                /\ beginRound' := beginRound)
              /\ (msgsPropose' := msgsPropose
                /\ msgsPrecommit' := msgsPrecommit
                /\ evidence' := evidence
                /\ proposalReceptionTime' := proposalReceptionTime))
          \/ (\E msgs$3 \in SUBSET ({
            [type |-> t_1d$1, src |-> t_1e$1, round |-> t_1f$1, id |-> t_1g$1]:
              t_1d$1 \in {"PRECOMMIT"},
              t_1e$1 \in {"f4"},
              t_1f$1 \in {r$12},
              t_1g$1 \in
                {
                  <<t_1a$1, t_1b$1, t_1c$1>>:
                    t_1a$1 \in { "v0", "v1" } \union {"v2"},
                    t_1b$1 \in 0 .. 7,
                    t_1c$1 \in 0 .. 3
                }
          }):
            msgsPrecommit'
                = [
                  msgsPrecommit EXCEPT
                    ![r$12] = msgsPrecommit[r$12] \union msgs$3
                ]
              /\ ((round' = round
                  /\ step' = step
                  /\ decision' = decision
                  /\ lockedValue' = lockedValue
                  /\ lockedRound' = lockedRound
                  /\ validValue' = validValue
                  /\ validRound' = validRound)
                /\ (localClock' = localClock /\ realTime' = realTime)
                /\ beginRound' := beginRound)
              /\ (msgsPropose' := msgsPropose
                /\ msgsPrevote' := msgsPrevote
                /\ evidence' := evidence
                /\ proposalReceptionTime' := proposalReceptionTime))))
    \/ ((\A p$17 \in { "c1", "c2", "c3" }:
        \A q$4 \in { "c1", "c2", "c3" }:
          p$17 = q$4
            \/ ((localClock[p$17] >= localClock[q$4]
                /\ localClock[p$17] - localClock[q$4] < 2)
              \/ (localClock[p$17] < localClock[q$4]
                /\ localClock[q$4] - localClock[p$17] < 2)))
      /\ (\E p$18 \in { "c1", "c2", "c3" }:
        LET (*@type: (() => Int); *) r_si_13 == round[p$18] IN
          p$18 = Proposer[(r_si_13)]
            /\ step[p$18] = "PROPOSE"
            /\ (\A m$4 \in msgsPropose[(r_si_13)]: ~(m$4["src"] = p$18))
            /\ (\E v$4 \in { "v0", "v1" }:
              LET (*
                @type: (() => <<Str, Int, Int>>);
              *)
              proposal_si_1 ==
                IF ~(validValue[p$18] = <<"None", (-1), (-1)>>)
                THEN validValue[p$18]
                ELSE <<v$4, localClock[p$18], (r_si_13)>>
              IN
              LET (*
                  @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
                *)
                newMsg_si_1 ==
                  [type |-> "PROPOSAL",
                    src |-> p$18,
                    round |-> r_si_13,
                    proposal |-> proposal_si_1,
                    validRound |-> validRound[p$18]]
                IN
                msgsPropose'
                    = [
                      msgsPropose EXCEPT
                        ![r_si_13] =
                          msgsPropose[(r_si_13)] \union {(newMsg_si_1)}
                    ])
            /\ ((localClock' = localClock /\ realTime' = realTime)
              /\ (round' = round
                /\ step' = step
                /\ decision' = decision
                /\ lockedValue' = lockedValue
                /\ lockedRound' = lockedRound
                /\ validValue' = validValue
                /\ validRound' = validRound))
            /\ (msgsPrevote' := msgsPrevote
              /\ msgsPrecommit' := msgsPrecommit
              /\ evidence' := evidence
              /\ proposalReceptionTime' := proposalReceptionTime)
            /\ beginRound' := beginRound
            /\ action' = "InsertProposal"
          \/ (\E v$5 \in { "v0", "v1" } \union {"v2"}:
            \E t$5 \in 0 .. 7:
              LET (*@type: (() => Int); *) r_si_14 == round[p$18] IN
                LET (*
                  @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
                *)
                msg_si_2 ==
                  [type |-> "PROPOSAL",
                    src |-> Proposer[round[p$18]],
                    round |-> round[p$18],
                    proposal |-> <<v$5, t$5, (r_si_14)>>,
                    validRound |-> -1]
                IN
                msg_si_2 \in msgsPropose[round[p$18]]
                  /\ proposalReceptionTime[(r_si_14), p$18] = -1
                  /\ proposalReceptionTime'
                    = [
                      proposalReceptionTime EXCEPT
                        ![<<(r_si_14), p$18>>] = localClock[p$18]
                    ]
                  /\ ((localClock' = localClock /\ realTime' = realTime)
                    /\ (round' = round
                      /\ step' = step
                      /\ decision' = decision
                      /\ lockedValue' = lockedValue
                      /\ lockedRound' = lockedRound
                      /\ validValue' = validValue
                      /\ validRound' = validRound))
                  /\ (msgsPropose' := msgsPropose
                    /\ msgsPrevote' := msgsPrevote
                    /\ msgsPrecommit' := msgsPrecommit
                    /\ evidence' := evidence)
                  /\ beginRound' := beginRound
                  /\ action' = "ReceiveProposal")
          \/ (\E v$6 \in { "v0", "v1" } \union {"v2"}:
            \E t$6 \in 0 .. 7:
              LET (*@type: (() => Int); *) r_si_15 == round[p$18] IN
              LET (*
                @type: (() => <<Str, Int, Int>>);
              *)
              prop_si_4 == <<v$6, t$6, (r_si_15)>>
              IN
              step[p$18] = "PROPOSE"
                /\ LET (*
                  @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
                *)
                msg_si_3 ==
                  [type |-> "PROPOSAL",
                    src |-> Proposer[(r_si_15)],
                    round |-> r_si_15,
                    proposal |-> prop_si_4,
                    validRound |-> -1]
                IN
                evidence' = {(msg_si_3)} \union evidence
                /\ LET (*
                  @type: (() => <<Str, Int, Int>>);
                *)
                mid_si_1 ==
                  IF (proposalReceptionTime[(r_si_15), p$18] >= t$6 - 2
                      /\ proposalReceptionTime[(r_si_15), p$18] <= (t$6 + 2) + 2)
                    /\ prop_si_4
                      \in {
                        <<t_1h$1, t_1i$1, t_1j$1>>:
                          t_1h$1 \in { "v0", "v1" },
                          t_1i$1 \in 2 .. 7,
                          t_1j$1 \in 0 .. 3
                      }
                    /\ (lockedRound[p$18] = -1 \/ lockedValue[p$18] = v$6)
                  THEN prop_si_4
                  ELSE <<"None", (-1), (-1)>>
                IN
                LET (*
                  @type: (() => [id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]);
                *)
                newMsg_si_2 ==
                  [type |-> "PREVOTE",
                    src |-> p$18,
                    round |-> r_si_15,
                    id |-> mid_si_1]
                IN
                msgsPrevote'
                    = [
                      msgsPrevote EXCEPT
                        ![r_si_15] =
                          msgsPrevote[(r_si_15)] \union {(newMsg_si_2)}
                    ]
                /\ step' = [ step EXCEPT ![p$18] = "PREVOTE" ]
                /\ ((localClock' = localClock /\ realTime' = realTime)
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
          \/ (\E v$7 \in { "v0", "v1" } \union {"v2"}:
            \E t$7 \in 0 .. 7:
              \E vr$1 \in 0 .. 3:
                \E pr$3 \in 0 .. 3:
                  LET (*@type: (() => Int); *) r_si_16 == round[p$18] IN
                  LET (*
                    @type: (() => <<Str, Int, Int>>);
                  *)
                  prop_si_5 == <<v$7, t$7, pr$3>>
                  IN
                  ((step[p$18] = "PROPOSE" /\ 0 <= vr$1) /\ vr$1 < r_si_16)
                    /\ pr$3 <= vr$1
                    /\ LET (*
                      @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
                    *)
                    msg_si_4 ==
                      [type |-> "PROPOSAL",
                        src |-> Proposer[(r_si_16)],
                        round |-> r_si_16,
                        proposal |-> prop_si_5,
                        validRound |-> vr$1]
                    IN
                    msg_si_4 \in msgsPropose[(r_si_16)]
                      /\ LET (*
                        @type: (() => Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]));
                      *)
                      PV_si_1 ==
                        { m$5 \in msgsPrevote[vr$1]: m$5["id"] = prop_si_5 }
                      IN
                      Cardinality((PV_si_1)) >= 2 * 1 + 1
                        /\ evidence'
                          = (PV_si_1 \union {(msg_si_4)}) \union evidence
                    /\ LET (*
                      @type: (() => <<Str, Int, Int>>);
                    *)
                    mid_si_2 ==
                      IF prop_si_5
                          \in {
                            <<t_1k$1, t_1l$1, t_1m$1>>:
                              t_1k$1 \in { "v0", "v1" },
                              t_1l$1 \in 2 .. 7,
                              t_1m$1 \in 0 .. 3
                          }
                        /\ (lockedRound[p$18] <= vr$1 \/ lockedValue[p$18] = v$7)
                      THEN prop_si_5
                      ELSE <<"None", (-1), (-1)>>
                    IN
                    LET (*
                      @type: (() => [id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]);
                    *)
                    newMsg_si_3 ==
                      [type |-> "PREVOTE",
                        src |-> p$18,
                        round |-> r_si_16,
                        id |-> mid_si_2]
                    IN
                    msgsPrevote'
                        = [
                          msgsPrevote EXCEPT
                            ![r_si_16] =
                              msgsPrevote[(r_si_16)] \union {(newMsg_si_3)}
                        ]
                    /\ step' = [ step EXCEPT ![p$18] = "PREVOTE" ]
                    /\ ((localClock' = localClock /\ realTime' = realTime)
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
          \/ (step[p$18] = "PREVOTE"
            /\ (\E MyEvidence$1 \in SUBSET (msgsPrevote[round[p$18]]):
              LET (*
                @type: (() => Set(Str));
              *)
              Voters_si_1 == { m$6["src"]: m$6 \in MyEvidence$1 }
              IN
              Cardinality((Voters_si_1)) >= 2 * 1 + 1
                /\ evidence' = MyEvidence$1 \union evidence
                /\ LET (*
                  @type: (() => [id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]);
                *)
                newMsg_si_4 ==
                  [type |-> "PRECOMMIT",
                    src |-> p$18,
                    round |-> round[p$18],
                    id |-> <<"None", (-1), (-1)>>]
                IN
                msgsPrecommit'
                    = [
                      msgsPrecommit EXCEPT
                        ![round[p$18]] =
                          msgsPrecommit[round[p$18]] \union {(newMsg_si_4)}
                    ]
                /\ step' = [ step EXCEPT ![p$18] = "PRECOMMIT" ]
                /\ ((localClock' = localClock /\ realTime' = realTime)
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
          \/ (\E v$8 \in { "v0", "v1" }:
            \E t$8 \in 0 .. 7:
              \E vr$2 \in 0 .. 3 \union {(-1)}:
                LET (*@type: (() => Int); *) r_si_17 == round[p$18] IN
                LET (*
                  @type: (() => <<Str, Int, Int>>);
                *)
                prop_si_6 == <<v$8, t$8, (r_si_17)>>
                IN
                step[p$18] \in { "PREVOTE", "PRECOMMIT" }
                  /\ LET (*
                    @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
                  *)
                  msg_si_5 ==
                    [type |-> "PROPOSAL",
                      src |-> Proposer[(r_si_17)],
                      round |-> r_si_17,
                      proposal |-> prop_si_6,
                      validRound |-> vr$2]
                  IN
                  msg_si_5 \in msgsPropose[(r_si_17)]
                    /\ LET (*
                      @type: (() => Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]));
                    *)
                    PV_si_2 ==
                      { m$7 \in msgsPrevote[(r_si_17)]: m$7["id"] = prop_si_6 }
                    IN
                    Cardinality((PV_si_2)) >= 2 * 1 + 1
                      /\ evidence'
                        = (PV_si_2 \union {(msg_si_5)}) \union evidence
                  /\ (IF step[p$18] = "PREVOTE"
                  THEN lockedValue' = [ lockedValue EXCEPT ![p$18] = v$8 ]
                    /\ lockedRound' = [ lockedRound EXCEPT ![p$18] = r_si_17 ]
                    /\ LET (*
                      @type: (() => [id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]);
                    *)
                    newMsg_si_5 ==
                      [type |-> "PRECOMMIT",
                        src |-> p$18,
                        round |-> r_si_17,
                        id |-> prop_si_6]
                    IN
                    msgsPrecommit'
                        = [
                          msgsPrecommit EXCEPT
                            ![r_si_17] =
                              msgsPrecommit[(r_si_17)] \union {(newMsg_si_5)}
                        ]
                    /\ step' = [ step EXCEPT ![p$18] = "PRECOMMIT" ]
                  ELSE lockedValue' := lockedValue
                    /\ lockedRound' := lockedRound
                    /\ msgsPrecommit' := msgsPrecommit
                    /\ step' := step)
                  /\ validValue' = [ validValue EXCEPT ![p$18] = prop_si_6 ]
                  /\ validRound' = [ validRound EXCEPT ![p$18] = r_si_17 ]
                  /\ ((localClock' = localClock /\ realTime' = realTime)
                    /\ beginRound' := beginRound)
                  /\ (round' := round /\ decision' := decision)
                  /\ (msgsPropose' := msgsPropose
                    /\ msgsPrevote' := msgsPrevote
                    /\ proposalReceptionTime' := proposalReceptionTime)
                  /\ action' = "UponProposalInPrevoteOrCommitAndPrevote")
          \/ ((\E MyEvidence$2 \in SUBSET (msgsPrecommit[round[p$18]]):
              LET (*
                @type: (() => Set(Str));
              *)
              Committers_si_1 == { m$8["src"]: m$8 \in MyEvidence$2 }
              IN
              Cardinality((Committers_si_1)) >= 2 * 1 + 1
                /\ evidence' = MyEvidence$2 \union evidence
                /\ round[p$18] + 1 \in 0 .. 3
                /\ (~(step[p$18] = "DECIDED")
                  /\ round' = [ round EXCEPT ![p$18] = round[p$18] + 1 ]
                  /\ step' = [ step EXCEPT ![p$18] = "PROPOSE" ]
                  /\ beginRound'
                    = [
                      beginRound EXCEPT
                        ![<<(round[p$18] + 1), p$18>>] = localClock[p$18]
                    ])
                /\ (localClock' = localClock /\ realTime' = realTime)
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
          \/ (decision[p$18] = <<<<"None", (-1), (-1)>>, (-1)>>
            /\ (\E v$9 \in { "v0", "v1" }:
              \E t$9 \in 0 .. 7:
                \E r$18 \in 0 .. 3:
                  \E pr$4 \in 0 .. 3:
                    \E vr$3 \in 0 .. 3 \union {(-1)}:
                      LET (*
                        @type: (() => <<Str, Int, Int>>);
                      *)
                      prop_si_7 == <<v$9, t$9, pr$4>>
                      IN
                      LET (*
                          @type: (() => [proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]);
                        *)
                        msg_si_6 ==
                          [type |-> "PROPOSAL",
                            src |-> Proposer[r$18],
                            round |-> r$18,
                            proposal |-> prop_si_7,
                            validRound |-> vr$3]
                        IN
                        msg_si_6 \in msgsPropose[r$18]
                          /\ ~(proposalReceptionTime[r$18, p$18] = -1)
                          /\ LET (*
                            @type: (() => Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]));
                          *)
                          PV_si_3 ==
                            {
                              m$9 \in msgsPrecommit[r$18]:
                                m$9["id"] = prop_si_7
                            }
                          IN
                          Cardinality((PV_si_3)) >= 2 * 1 + 1
                            /\ evidence'
                              = (PV_si_3 \union {(msg_si_6)}) \union evidence
                            /\ decision'
                              = [
                                decision EXCEPT
                                  ![p$18] = <<(prop_si_7), r$18>>
                              ]
                            /\ step' = [ step EXCEPT ![p$18] = "DECIDED" ]
                            /\ (localClock' = localClock /\ realTime' = realTime)
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
          \/ (step[p$18] = "PROPOSE"
            /\ ~(p$18 = Proposer[round[p$18]])
            /\ LET (*
              @type: (() => [id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]);
            *)
            newMsg_si_6 ==
              [type |-> "PREVOTE",
                src |-> p$18,
                round |-> round[p$18],
                id |-> <<"None", (-1), (-1)>>]
            IN
            msgsPrevote'
                = [
                  msgsPrevote EXCEPT
                    ![round[p$18]] =
                      msgsPrevote[round[p$18]] \union {(newMsg_si_6)}
                ]
            /\ step' = [ step EXCEPT ![p$18] = "PREVOTE" ]
            /\ ((localClock' = localClock /\ realTime' = realTime)
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
          \/ (step[p$18] = "PREVOTE"
            /\ LET (*
              @type: (() => Set([id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]));
            *)
            PV_si_4 ==
              {
                m$10 \in msgsPrevote[round[p$18]]:
                  m$10["id"] = <<"None", (-1), (-1)>>
              }
            IN
            Cardinality((PV_si_4)) >= 2 * 1 + 1
              /\ evidence' = PV_si_4 \union evidence
              /\ LET (*
                @type: (() => [id: <<Str, Int, Int>>, round: Int, src: Str, type: Str]);
              *)
              newMsg_si_7 ==
                [type |-> "PRECOMMIT",
                  src |-> p$18,
                  round |-> round[p$18],
                  id |-> <<"None", (-1), (-1)>>]
              IN
              msgsPrecommit'
                  = [
                    msgsPrecommit EXCEPT
                      ![round[p$18]] =
                        msgsPrecommit[round[p$18]] \union {(newMsg_si_7)}
                  ]
              /\ step' = [ step EXCEPT ![p$18] = "PRECOMMIT" ]
              /\ ((localClock' = localClock /\ realTime' = realTime)
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
          \/ (\E r$19 \in 0 .. 3:
            r$19 > round[p$18]
              /\ LET (*
                @type: (() => Set([id: <<Str, Int, Int>>, proposal: <<Str, Int, Int>>, round: Int, src: Str, type: Str, validRound: Int]));
              *)
              RoundMsgs_si_1 ==
                (msgsPropose[r$19] \union msgsPrevote[r$19])
                  \union msgsPrecommit[r$19]
              IN
              \E MyEvidence$3 \in SUBSET RoundMsgs_si_1:
                LET (*
                  @type: (() => Set(Str));
                *)
                Faster_si_1 == { m$11["src"]: m$11 \in MyEvidence$3 }
                IN
                Cardinality((Faster_si_1)) >= 1 + 1
                  /\ evidence' = MyEvidence$3 \union evidence
                  /\ (~(step[p$18] = "DECIDED")
                    /\ round' = [ round EXCEPT ![p$18] = r$19 ]
                    /\ step' = [ step EXCEPT ![p$18] = "PROPOSE" ]
                    /\ beginRound'
                      = [
                        beginRound EXCEPT
                          ![<<r$19, p$18>>] = localClock[p$18]
                      ])
                  /\ (localClock' = localClock /\ realTime' = realTime)
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
  \E t_1n$1 \in [(0 .. 3) -> ({ "c1", "c2", "c3" } \union {"f4"})]:
    Proposer' = t_1n$1

InitPrimed ==
  round' = [ p$19 \in { "c1", "c2", "c3" } |-> 0 ]
    /\ (\E t_1o$1 \in [{ "c1", "c2", "c3" } -> (2 .. 2 + 2)]:
      localClock' = t_1o$1)
    /\ realTime' = 0
    /\ step' = [ p$20 \in { "c1", "c2", "c3" } |-> "PROPOSE" ]
    /\ decision'
      = [ p$21 \in { "c1", "c2", "c3" } |-> <<<<"None", (-1), (-1)>>, (-1)>> ]
    /\ lockedValue' = [ p$22 \in { "c1", "c2", "c3" } |-> "None" ]
    /\ lockedRound' = [ p$23 \in { "c1", "c2", "c3" } |-> -1 ]
    /\ validValue'
      = [ p$24 \in { "c1", "c2", "c3" } |-> <<"None", (-1), (-1)>> ]
    /\ validRound' = [ p$25 \in { "c1", "c2", "c3" } |-> -1 ]
    /\ ((TRUE
        /\ ((\E t_1p$1 \in [(0 .. 3) -> Gen(5)]: msgsPropose' = t_1p$1)
          /\ (\E t_1q$1 \in [(0 .. 3) -> Gen(5)]: msgsPrevote' = t_1q$1)
          /\ (\E t_1r$1 \in [(0 .. 3) -> Gen(5)]: msgsPrecommit' = t_1r$1)
          /\ ((\A r$20 \in 0 .. 3:
              (\A t_20$1 \in msgsPropose'[r$20]:
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
                /\ (\A m$12 \in msgsPropose'[r$20]: r$20 = m$12["round"])))
          /\ ((\A r$21 \in 0 .. 3:
              (\A t_28$1 \in msgsPrevote'[r$21]:
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
                /\ (\A m$13 \in msgsPrevote'[r$21]: r$21 = m$13["round"])))
          /\ ((\A r$22 \in 0 .. 3:
              (\A t_2g$1 \in msgsPrecommit'[r$22]:
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
                /\ (\A m$14 \in msgsPrecommit'[r$22]: r$22 = m$14["round"])))))
      \/ (FALSE
        /\ msgsPropose' = [ r$23 \in 0 .. 3 |-> {} ]
        /\ msgsPrevote' = [ r$24 \in 0 .. 3 |-> {} ]
        /\ msgsPrecommit' = [ r$25 \in 0 .. 3 |-> {} ]))
    /\ proposalReceptionTime'
      = [
        r_p$2 \in
          {
            <<t_2h$1, t_2i$1>>:
              t_2h$1 \in 0 .. 3,
              t_2i$1 \in { "c1", "c2", "c3" }
          } |->
          -1
      ]
    /\ evidence' = {}
    /\ action' = "Init"
    /\ beginRound'
      = [
        r_c$2 \in
          {
            <<t_2j$1, t_2k$1>>:
              t_2j$1 \in 0 .. 3,
              t_2k$1 \in { "c1", "c2", "c3" }
          } |->
          IF r_c$2[1] = 0 THEN localClock'[r_c$2[2]] ELSE 7
      ]

(*
  @type: Bool;
*)
VCInv_si_0 ==
  \A p$26 \in { "c1", "c2", "c3" }:
    \A q$5 \in { "c1", "c2", "c3" }:
      (decision[p$26] = <<<<"None", (-1), (-1)>>, (-1)>>
          \/ decision[q$5] = <<<<"None", (-1), (-1)>>, (-1)>>)
        \/ (\E v$10 \in { "v0", "v1" }:
          \E t$10 \in 0 .. 7:
            \E pr$5 \in 0 .. 3:
              \E r1$2 \in 0 .. 3:
                \E r2$2 \in 0 .. 3:
                  LET (*
                    @type: (() => <<Str, Int, Int>>);
                  *)
                  prop_si_8 == <<v$10, t$10, pr$5>>
                  IN
                  decision[p$26] = <<(prop_si_8), r1$2>>
                    /\ decision[q$5] = <<(prop_si_8), r2$2>>)

(*
  @type: Bool;
*)
VCNotInv_si_0 ==
  \E p$27 \in { "c1", "c2", "c3" }:
    \E q$6 \in { "c1", "c2", "c3" }:
      (~(decision[p$27] = <<<<"None", (-1), (-1)>>, (-1)>>)
          /\ ~(decision[q$6] = <<<<"None", (-1), (-1)>>, (-1)>>))
        /\ (\A v$11 \in { "v0", "v1" }:
          \A t$11 \in 0 .. 7:
            \A pr$6 \in 0 .. 3:
              \A r1$3 \in 0 .. 3:
                \A r2$3 \in 0 .. 3:
                  LET (*
                    @type: (() => <<Str, Int, Int>>);
                  *)
                  prop_si_9 == <<v$11, t$11, pr$6>>
                  IN
                  ~(decision[p$27] = <<(prop_si_9), r1$3>>)
                    \/ ~(decision[q$6] = <<(prop_si_9), r2$3>>))

(*
  @type: Bool;
*)
VCInv_si_1 ==
  \A p$28 \in { "c1", "c2", "c3" }:
    LET (*
      @type: (() => <<Str, Int, Int>>);
    *)
    prop_si_10 == decision[p$28][1]
    IN
    prop_si_10 = <<"None", (-1), (-1)>> \/ (prop_si_10)[1] \in { "v0", "v1" }

(*
  @type: Bool;
*)
VCNotInv_si_1 ==
  \E p$29 \in { "c1", "c2", "c3" }:
    LET (*
      @type: (() => <<Str, Int, Int>>);
    *)
    prop_si_11 == decision[p$29][1]
    IN
    ~(prop_si_11 = <<"None", (-1), (-1)>>)
      /\ ~((prop_si_11)[1] \in { "v0", "v1" })

(*
  @type: Bool;
*)
VCInv_si_2 ==
  \A p$30 \in { "c1", "c2", "c3" }:
    \E v$12 \in { "v0", "v1" }:
      \E t$12 \in 0 .. 7:
        \E pr$7 \in 0 .. 3:
          \E dr$2 \in 0 .. 3:
            ~(decision[p$30] = <<<<v$12, t$12, pr$7>>, dr$2>>)
              \/ (\E q$7 \in { "c1", "c2", "c3" }:
                LET (*
                  @type: (() => Int);
                *)
                propRecvTime_si_2 == proposalReceptionTime[pr$7, q$7]
                IN
                beginRound[pr$7, q$7] <= propRecvTime_si_2
                  /\ beginRound[(pr$7 + 1), q$7] >= propRecvTime_si_2
                  /\ (propRecvTime_si_2 >= t$12 - 2
                    /\ propRecvTime_si_2 <= (t$12 + 2) + 2))

(*
  @type: Bool;
*)
VCNotInv_si_2 ==
  \E p$31 \in { "c1", "c2", "c3" }:
    \A v$13 \in { "v0", "v1" }:
      \A t$13 \in 0 .. 7:
        \A pr$8 \in 0 .. 3:
          \A dr$3 \in 0 .. 3:
            decision[p$31] = <<<<v$13, t$13, pr$8>>, dr$3>>
              /\ (\A q$8 \in { "c1", "c2", "c3" }:
                LET (*
                  @type: (() => Int);
                *)
                propRecvTime_si_3 == proposalReceptionTime[pr$8, q$8]
                IN
                beginRound[pr$8, q$8] > propRecvTime_si_3
                  \/ beginRound[(pr$8 + 1), q$8] < propRecvTime_si_3
                  \/ (propRecvTime_si_3 < t$13 - 2
                    \/ propRecvTime_si_3 > (t$13 + 2) + 2))

(*
  @type: Bool;
*)
VCInv_si_3 ==
  \A r$26 \in { t_2l$1 \in 0 .. 3: ~(t_2l$1 \in {3}) }:
    \A v$14 \in { "v0", "v1" }:
      LET (*@type: (() => Str); *) p_si_32 == Proposer[r$26] IN
      ~(p_si_32 \in { "c1", "c2", "c3" })
        \/ (\E t$14 \in 0 .. 7:
          LET (*
            @type: (() => <<Str, Int, Int>>);
          *)
          prop_si_12 == <<v$14, t$14, r$26>>
          IN
          ((\A msg$7 \in msgsPropose[r$26]:
                ~(msg$7["proposal"] = prop_si_12)
                  \/ ~(msg$7["src"] = p_si_32)
                  \/ ~(msg$7["validRound"] = -1))
              \/ LET (*@type: (() => Int); *) tOffset_si_2 == (t$14 + 2) + 2 IN
              [
                  r$27 \in 0 .. 3 |->
                    ApaFoldSet(LET (*
                      @type: ((Int, Int) => Int);
                    *)
                    Min2_si_3(a_si_5, b_si_5) == IF a$5 <= b$5 THEN a$5 ELSE b$5
                    IN
                    Min2$3, 7, {
                      beginRound[r$27, p$33]:
                        p$33 \in { "c1", "c2", "c3" }
                    })
                ][
                  r$26
                ]
                  > t$14
                \/ t$14
                  > [
                    r$28 \in 0 .. 3 |->
                      ApaFoldSet(LET (*
                        @type: ((Int, Int) => Int);
                      *)
                      Max2_si_3(a_si_6, b_si_6) ==
                        IF a$6 >= b$6 THEN a$6 ELSE b$6
                      IN
                      Max2$3, (-1), {
                        beginRound[r$28, p$34]:
                          p$34 \in { "c1", "c2", "c3" }
                      })
                  ][
                    r$26
                  ]
                \/ [
                  r$29 \in 0 .. 3 |->
                    ApaFoldSet(LET (*
                      @type: ((Int, Int) => Int);
                    *)
                    Max2_si_4(a_si_7, b_si_7) == IF a$7 >= b$7 THEN a$7 ELSE b$7
                    IN
                    Max2$4, (-1), {
                      beginRound[r$29, p$35]:
                        p$35 \in { "c1", "c2", "c3" }
                    })
                ][
                  r$26
                ]
                  > tOffset_si_2
                \/ tOffset_si_2
                  > [
                    r$30 \in 0 .. 3 |->
                      ApaFoldSet(LET (*
                        @type: ((Int, Int) => Int);
                      *)
                      Min2_si_4(a_si_8, b_si_8) ==
                        IF a$8 <= b$8 THEN a$8 ELSE b$8
                      IN
                      Min2$4, 7, {
                        beginRound[r$30, p$36]:
                          p$36 \in { "c1", "c2", "c3" }
                      })
                  ][
                    (r$26 + 1)
                  ])
            \/ (\A q$9 \in { "c1", "c2", "c3" }:
              LET (*
                @type: (() => <<<<Str, Int, Int>>, Int>>);
              *)
              dq_si_2 == decision[q$9]
              IN
              dq_si_2 = <<<<"None", (-1), (-1)>>, (-1)>>
                \/ (dq_si_2)[1] = prop_si_12))

(*
  @type: Bool;
*)
VCNotInv_si_3 ==
  \E r$31 \in { t_2m$1 \in 0 .. 3: ~(t_2m$1 \in {3}) }:
    \E v$15 \in { "v0", "v1" }:
      LET (*@type: (() => Str); *) p_si_37 == Proposer[r$31] IN
      p_si_37 \in { "c1", "c2", "c3" }
        /\ (\A t$15 \in 0 .. 7:
          LET (*
            @type: (() => <<Str, Int, Int>>);
          *)
          prop_si_13 == <<v$15, t$15, r$31>>
          IN
          ((\E msg$8 \in msgsPropose[r$31]:
                msg$8["proposal"] = prop_si_13
                  /\ msg$8["src"] = p_si_37
                  /\ msg$8["validRound"] = -1)
              /\ LET (*@type: (() => Int); *) tOffset_si_3 == (t$15 + 2) + 2 IN
              [
                  r$32 \in 0 .. 3 |->
                    ApaFoldSet(LET (*
                      @type: ((Int, Int) => Int);
                    *)
                    Min2_si_5(a_si_9, b_si_9) == IF a$9 <= b$9 THEN a$9 ELSE b$9
                    IN
                    Min2$5, 7, {
                      beginRound[r$32, p$38]:
                        p$38 \in { "c1", "c2", "c3" }
                    })
                ][
                  r$31
                ]
                  <= t$15
                /\ t$15
                  <= [
                    r$33 \in 0 .. 3 |->
                      ApaFoldSet(LET (*
                        @type: ((Int, Int) => Int);
                      *)
                      Max2_si_5(a_si_10, b_si_10) ==
                        IF a$10 >= b$10 THEN a$10 ELSE b$10
                      IN
                      Max2$5, (-1), {
                        beginRound[r$33, p$39]:
                          p$39 \in { "c1", "c2", "c3" }
                      })
                  ][
                    r$31
                  ]
                /\ [
                  r$34 \in 0 .. 3 |->
                    ApaFoldSet(LET (*
                      @type: ((Int, Int) => Int);
                    *)
                    Max2_si_6(a_si_11, b_si_11) ==
                      IF a$11 >= b$11 THEN a$11 ELSE b$11
                    IN
                    Max2$6, (-1), {
                      beginRound[r$34, p$40]:
                        p$40 \in { "c1", "c2", "c3" }
                    })
                ][
                  r$31
                ]
                  <= tOffset_si_3
                /\ tOffset_si_3
                  <= [
                    r$35 \in 0 .. 3 |->
                      ApaFoldSet(LET (*
                        @type: ((Int, Int) => Int);
                      *)
                      Min2_si_6(a_si_12, b_si_12) ==
                        IF a$12 <= b$12 THEN a$12 ELSE b$12
                      IN
                      Min2$6, 7, {
                        beginRound[r$35, p$41]:
                          p$41 \in { "c1", "c2", "c3" }
                      })
                  ][
                    (r$31 + 1)
                  ])
            /\ (\E q$10 \in { "c1", "c2", "c3" }:
              LET (*
                @type: (() => <<<<Str, Int, Int>>, Int>>);
              *)
              dq_si_3 == decision[q$10]
              IN
              ~(dq_si_3 = <<<<"None", (-1), (-1)>>, (-1)>>)
                /\ ~((dq_si_3)[1] = prop_si_13)))

================================================================================
