# How to run tests

The script `runApalache.sh` runs Apalache against one of the model files in this repository. This document describes how to use it.

1. Get Apalache, by following [these](https://apalache.informal.systems/docs/apalache/installation/index.html) instructions. Summarized:

    1. `git clone https://github.com/informalsystems/apalache.git`
    2. `make package`

2. Define an environment variable `APALACHE_HOME` and set it to the directory where you cloned the repository (resp. unpacked the prebuilt release). `$APALACHE_HOME/bin/apalache-mc` should point to the run script.

3. Call `runApalache.sh CMD N CM DD` where:

    1. `CMD` is the command. Either "check" or "typecheck". Default: "typecheck"
    2. `N` is the number of steps if `CMD=check`. Ignored if `CMD=typecheck`. Default: 10
    3. `MC` is a Boolean flag that controls whether the model cehcked has a majoricty of correct processes. Default: true
    4. `DD` is a Boolean flag that controls Apalache's `--discard-disabled` flag (See [here](https://apalache.informal.systems/docs/apalache/running.html)). Ignored if `CMD=typecheck`. Default: false

The results will be written to `_apalache-out` (see the [Apalache documentation](https://apalache.informal.systems/docs/adr/009adr-outputs.html)).

Example:
```sh
runApalache.sh check 2
```
Checks 2 steps of `MC_PBT_3C_1F.tla` and
```sh
runApalache.sh check 10 false
```
Checks 10 steps of `MC_PBT_2C_2F.tla`

# Updating the experiments log

After running a particularly significant test, copy the raw outputs from 
`_apalache-out` to `experiment_data` and update `experiment_log.md` accordingly.
See `experiment_data/May2022` for a suggested directory layout. 

Make sure to copy at least the `detailed.log` and `run.txt` files, as well as any counterexample files, if present.
