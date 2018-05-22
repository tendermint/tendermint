Running in production
=====================

Logging
-------

Default logging level (``main:info,state:info,*:``) should suffice for normal
operation mode. Read `this post
<https://blog.cosmos.network/one-of-the-exciting-new-features-in-0-10-0-release-is-smart-log-level-flag-e2506b4ab756>__`
for details on how to configure ``log_level`` config variable. Some of the
modules can be found `here <./how-to-read-logs.html#list-of-modules>__`.

If you're trying to debug Tendermint or asked to provide logs with debug
logging level, you can do so by running tendermint with
``--log_level="*:debug"``.

Consensus WAL
-------------

Consensus module writes every message to the WAL (write-ahead log).
