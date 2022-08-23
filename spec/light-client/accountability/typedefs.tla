-------------------- MODULE typedefs ---------------------------
(*
  // the process type
  @typeAlias: process = Str;
  // the value type
  @typeAlias: value = Str;
  // the type of step labels
  @typeAlias: step = Str;
  // the type of round numbers
  @typeAlias: round = Int;
  // the type of action labels
  @typeAlias: action = Str;
  // the type of action traces
  @typeAlias: trace = Seq(Str);
  // the type of PROPOSE messages
  @typeAlias: proposeMsg =
  {
    type: $step,
    src: $process,
    round: $round,
    proposal: $value,
    validRound: $round
  };
  // the type of PRECOMMIT messages
  @typeAlias: preMsg =
  {
    type: $step,
    src: $process,
    round: $round,
    id: $value
  };
*)
TypeAliases == TRUE

=============================================================================
