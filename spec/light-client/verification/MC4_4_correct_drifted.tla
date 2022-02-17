---------------------- MODULE MC4_4_correct_drifted ---------------------------

AllNodes == {"n1", "n2", "n3", "n4"}
TRUSTED_HEIGHT == 1
TARGET_HEIGHT == 4
TRUSTING_PERIOD == 1400 \* two weeks, one day is 100 time units :-)
CLOCK_DRIFT == 10       \* how much we assume the local clock is drifting
REAL_CLOCK_DRIFT == 30  \* how much the local clock is actually drifting
IS_PRIMARY_CORRECT == TRUE
FAULTY_RATIO == <<1, 3>>    \* < 1 / 3 faulty validators

VARIABLES
  state, nextHeight, fetchedLightBlocks, lightBlockStatus, latestVerified,
  nprobes,
  localClock,
  refClock, blockchain, Faulty

(* the light client previous state components, used for monitoring *)
VARIABLES
  prevVerified,
  prevCurrent,
  prevLocalClock,
  prevVerdict

INSTANCE Lightclient_003_draft
==============================================================================
