# Pending

BREAKING CHANGES:
- [types] CanonicalTime uses nanoseconds instead of clipping to ms
    - breaks serialization/signing of all messages with a timestamp
- [abci] Removed Fee from ResponseDeliverTx and ResponseCheckTx

IMPROVEMENTS:
- [blockchain] Improve fast-sync logic
    - tweak params
    - only process one block at a time to avoid starving
