------------------------- MODULE MC5_5_correct ---------------------------

AllNodes == {"n1", "n2", "n3", "n4", "n5"}
TRUSTED_HEIGHT == 1
TARGET_HEIGHT == 5
TRUSTING_PERIOD == 1400 \* two weeks, one day is 100 time units :-)
IS_PRIMARY_CORRECT == TRUE

VARIABLES
  state, nextHeight, nprobes, fetchedLightBlocks, lightBlockStatus,
  latestVerified, nprobes,
  now, blockchain, Faulty

INSTANCE Lightclient_A_1
============================================================================
