--------------------------- MODULE MC_1_0_4 ------------------------------------

\*a <: b == a \* type annotation

VALIDATOR_SETS == {"vs1", "vs2"}
NIL_VS == "NilVS"
CORRECT == {"c1"}
FAULTY == {"f2"} \ {"f2"}
MAX_HEIGHT == 4
PEER_MAX_REQUESTS == 2
TARGET_PENDING == 3

VARIABLES
    state, blockPool, peersState, chain, turn, inMsg, outMsg

INSTANCE fastsync_apalache    
================================================================================
