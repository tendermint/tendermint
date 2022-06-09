-------------------- MODULE typedefs ---------------------------
(*
  @typeAlias: PROCESS = Str;
  @typeAlias: VALUE = Str;
  @typeAlias: STEP = Str;
  @typeAlias: ROUND = Int;
  @typeAlias: ACTION = Str;
  @typeAlias: TRACE = Seq(Str);
  @typeAlias: PROPMESSAGE = 
  [
    type: STEP, 
    src: PROCESS, 
    round: ROUND,
    proposal: VALUE, 
    validRound: ROUND
  ];
  @typeAlias: PREMESSAGE = 
  [
    type: STEP, 
    src: PROCESS, 
    round: ROUND,
    id: VALUE
  ];
  @typeAlias: MESSAGE = 
  [
    type: STEP, 
    src: PROCESS, 
    round: ROUND,
    proposal: VALUE, 
    validRound: ROUND,
    id: VALUE
  ];
*)
TypeAliases == TRUE

=============================================================================