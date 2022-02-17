----------------------------- MODULE MC_n5_f1 -------------------------------
CONSTANT 
  \* @type: ROUND -> PROCESS;
  Proposer

\* the variables declared in TendermintAcc3
VARIABLES
  \* @type: PROCESS -> ROUND;
  round,  
  \* @type: PROCESS -> STEP;
  step,   
  \* @type: PROCESS -> VALUE;
  decision,
  \* @type: PROCESS -> VALUE;
  lockedValue, 
  \* @type: PROCESS -> ROUND;
  lockedRound, 
  \* @type: PROCESS -> VALUE;
  validValue,  
  \* @type: PROCESS -> ROUND;
  validRound,   
  \* @type: ROUND -> Set(PROPMESSAGE);
  msgsPropose, 
  \* @type: ROUND -> Set(PREMESSAGE);
  msgsPrevote, 
  \* @type: ROUND -> Set(PREMESSAGE);
  msgsPrecommit, 
  \* @type: Set(MESSAGE);
  evidence, 
  \* @type: ACTION;
  action 

INSTANCE TendermintAccDebug_004_draft WITH
  Corr <- {"c1", "c2", "c3", "c4"},
  Faulty <- {"f5"},
  N <- 5,
  T <- 1,
  ValidValues <- { "v0", "v1" },
  InvalidValues <- {"v2"},
  MaxRound <- 2

\* run Apalache with --cinit=ConstInit
ConstInit == \* the proposer is arbitrary -- works for safety
  Proposer \in [Rounds -> AllProcs]

=============================================================================    
