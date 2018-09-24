# Pending

Special thanks to external contributors on this release:

BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API

* Blockchain Protocol
  * [types] \#2459 `Vote`/`Proposal`/`Heartbeat` use amino encoding instead of JSON in `SignBytes`.
  * [privval] \#2459 Split `SocketPVMsg`s implementations into Request and Response, where the Response may contain a error message (returned by the remote signer). 

* P2P Protocol

FEATURES:

IMPROVEMENTS:

BUG FIXES:
