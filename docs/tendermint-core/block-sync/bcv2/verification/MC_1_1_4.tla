--------------------------- MODULE MC_1_1_4 ------------------------------------

VALIDATOR_SETS == {"vs1", "vs2"}
NIL_VS == "NilVS"
CORRECT == {"c1"}
FAULTY == {"f2"}
MAX_HEIGHT == 4
PEER_MAX_REQUESTS == 2
TARGET_PENDING == 3

VARIABLES
    state, blockPool, peersState, chain, turn, inMsg, outMsg

INSTANCE fastsync_apalache    
================================================================================
