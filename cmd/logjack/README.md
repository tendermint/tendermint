A custom version of logrotate that doesn't rely on sudo access to /etc/logrotate.d.

This will be the second process aside from "tendermint" managed by "debora/barak".

```bash
logjack -chopSize="10M" -limitSize="1G" $HOME/.tendermint/logs/tendermint.log
```

The above example chops the log file and moves it, e.g. to $HOME/.tendermint/logs/tendermint.log.000,
when the base file (tendermint.log) exceeds 10M in size.  If the total size of tendermint.log.XXX exceeds 1G in size,
the older files are removed.
