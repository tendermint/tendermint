------------------------- MODULE MC_vars --------------------------------------
\* group variable definitions to avoid repetition in MC_*.tla files.

EXTENDS typedefs

\* see Lightclient_003_draft.tla for the detailed description

VARIABLES
  \* @type: Int;
  localClock,
  \* @type: Str;
  state,        
  \* @type: Int;
  nextHeight,   
  \* @type: Int;
  nprobes,
  \* @type: Int -> $lightBlock;
  fetchedLightBlocks,
  \* @type: Int -> Str;
  lightBlockStatus,
  \* @type: $lightBlock;
  latestVerified,
  \* @type: $lightBlock;
  prevVerified,
  \* @type: $lightBlock;
  prevCurrent,
  \* @type: Int;
  prevLocalClock,
  \* @type: Str;
  prevVerdict,
  \* @type: Int;
  refClock,
  \* @type: Int -> $blockHeader;
  blockchain,
  \* @type: Set(NODE);
  Faulty

===============================================================================
