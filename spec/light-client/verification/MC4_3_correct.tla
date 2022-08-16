---------------------------- MODULE MC4_3_correct ---------------------------
EXTENDS MC_vars

AllNodes == {"1_OF_NODE", "2_OF_NODE", "3_OF_NODE", "4_OF_NODE"}
TRUSTED_HEIGHT == 1
TARGET_HEIGHT == 3
TRUSTING_PERIOD == 1400 \* two weeks, one day is 100 time units :-)
CLOCK_DRIFT == 10       \* how much we assume the local clock is drifting
REAL_CLOCK_DRIFT == 3   \* how much the local clock is actually drifting
IS_PRIMARY_CORRECT == TRUE
\* @type: <<Int, Int>>;
FAULTY_RATIO == <<1, 3>>    \* < 1 / 3 faulty validators

INSTANCE Lightclient_003_draft
==============================================================================
