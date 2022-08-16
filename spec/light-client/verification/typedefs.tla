----------------------------- MODULE typedefs ---------------------------------

(*
 @typeAlias: blockHeader = {
   height: Int,
   time: Int,
   lastCommit: Set(NODE),
   VS: Set(NODE),
   NextVS: Set(NODE)
 };

 @typeAlias: lightBlock = {
   header: $blockHeader,
   Commits: Set(NODE)
 };

 @typeAlias: blockchain = Int -> $blockHeader;

 @typeAlias: lightBlockMap = Int -> $lightBlock;

 @typeAlias: lightBlockStatus = Int -> Str;
 *)
typedefs_aliases == TRUE
===============================================================================
