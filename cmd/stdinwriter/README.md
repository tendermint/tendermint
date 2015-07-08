stdinwriter reads from stdin and writes to the specified file, in a way compatible for logrotate to move around.
(see tendermint/common/os#AutoFile)

```bash
some_command arg1 arg2 2>&1 | stdinwriter -o path_to_log.log
```
