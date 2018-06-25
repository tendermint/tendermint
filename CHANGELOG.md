# Changelog

## 0.9.0

*June 24th, 2018*

BREAKING:
 - [events, pubsub] Removed - moved to github.com/tendermint/tendermint
 - [merkle] Use 20-bytes of SHA256 instead of RIPEMD160. NOTE: this package is
   moving to github.com/tendermint/go-crypto !
 - [common] Remove gogoproto from KVPair types
 - [common] Error simplification, #220

FEATURES:

 - [db/remotedb] New DB type using an external CLevelDB process via
   GRPC
 - [autofile] logjack command for piping stdin to a rotating file
 - [bech32] New package. NOTE: should move out of here - it's just two small
   functions
 - [common] ColoredBytes([]byte) string for printing mixed ascii and bytes
 - [db] DebugDB uses ColoredBytes()

## 0.8.4

*June 5, 2018*

IMPROVEMENTS:

 - [autofile] Flush on Stop; Close() method to Flush and close file

## 0.8.3

*May 21, 2018*

FEATURES:

 - [common] ASCIITrim()

## 0.8.2 (April 23rd, 2018)

FEATURES:

 - [pubsub] TagMap, NewTagMap
 - [merkle] SimpleProofsFromMap()
 - [common] IsASCIIText()
 - [common] PrefixEndBytes // e.g. increment or nil
 - [common] BitArray.MarshalJSON/.UnmarshalJSON
 - [common] BitArray uses 'x' not 'X' for String() and above.
 - [db] DebugDB shows better colorized output

BUG FIXES:

 - [common] Fix TestParallelAbort nondeterministic failure #201/#202
 - [db] PrefixDB Iterator/ReverseIterator fixes
 - [db] DebugDB fixes

## 0.8.1 (April 5th, 2018)

FEATURES:

 - [common] Error.Error() includes cause
 - [common] IsEmpty() for 0 length

## 0.8.0 (April 4th, 2018)

BREAKING:

 - [merkle] `PutVarint->PutUvarint` in encodeByteSlice
 - [db] batch.WriteSync()
 - [common] Refactored and fixed `Parallel` function
 - [common] Refactored `Rand` functionality
 - [common] Remove unused `Right/LeftPadString` functions
 - [common] Remove StackError, introduce Error interface (to replace use of pkg/errors)

FEATURES:

 - [db] NewPrefixDB for a DB with all keys prefixed
 - [db] NewDebugDB prints everything during operation
 - [common] SplitAndTrim func
 - [common] rand.Float64(), rand.Int63n(n), rand.Int31n(n) and global equivalents
 - [common] HexBytes Format()

BUG FIXES:

 - [pubsub] Fix unsubscribing
 - [cli] Return config errors
 - [common] Fix WriteFileAtomic Windows bug

## 0.7.1 (March 22, 2018)

IMPROVEMENTS:

 - glide -> dep

BUG FIXES:

 - [common] Fix panic in NewBitArray for negative bits
 - [common] Fix and simplify WriteFileAtomic so it cleans up properly

## 0.7.0 (February 20, 2018)

BREAKING:

 - [db] Major API upgrade. See `db/types.go`.
 - [common] added `Quit() <-chan struct{}` to Service interface.
   The returned channel is closed when service is stopped.
 - [common] Remove HTTP functions
 - [common] Heap.Push takes an `int`, new Heap.PushComparable takes the comparable.
 - [logger] Removed. Use `log`
 - [merkle] Major API updade - uses cmn.KVPairs.
 - [cli] WriteDemoConfig -> WriteConfigValues
 - [all] Remove go-wire dependency!

FEATURES:

 - [db] New FSDB that uses the filesystem directly
 - [common] HexBytes
 - [common] KVPair and KI64Pair (protobuf based key-value pair objects)

IMPROVEMENTS:

 - [clist] add WaitChan() to CList, NextWaitChan() and PrevWaitChan()
   to CElement. These can be used instead of blocking `*Wait()` methods
   if you need to be able to send quit signal and not block forever
 - [common] IsHex handles 0x-prefix

BUG FIXES:

 - [common] BitArray check for nil arguments
 - [common] Fix memory leak in RepeatTimer

## 0.6.0 (December 29, 2017)

BREAKING:
 - [cli] remove --root
 - [pubsub] add String() method to Query interface

IMPROVEMENTS:
 - [common] use a thread-safe and well seeded non-crypto rng

BUG FIXES
 - [clist] fix misuse of wait group
 - [common] introduce Ticker interface and logicalTicker for better testing of timers

## 0.5.0 (December 5, 2017)

BREAKING:
 - [common] replace Service#Start, Service#Stop first return value (bool) with an
   error (ErrAlreadyStarted, ErrAlreadyStopped)
 - [common] replace Service#Reset first return value (bool) with an error
 - [process] removed

FEATURES:
 - [common] IntInSlice and StringInSlice functions
 - [pubsub/query] introduce `Condition` struct, expose `Operator`, and add `query.Conditions()`

## 0.4.1 (November 27, 2017)

FEATURES:
 - [common] `Keys()` method on `CMap`

IMPROVEMENTS:
 - [log] complex types now encoded as "%+v" by default if `String()` method is undefined (previously resulted in error)
 - [log] logger logs its own errors

BUG FIXES:
 - [common] fixed `Kill()` to build on Windows (Windows does not have `syscall.Kill`)

## 0.4.0 (October 26, 2017)

BREAKING:
 - [common] GoPath is now a function
 - [db] `DB` and `Iterator` interfaces have new methods to better support iteration

FEATURES:
 - [autofile] `Read([]byte)` and `Write([]byte)` methods on `Group` to support binary WAL
 - [common] `Kill()` sends SIGTERM to the current process

IMPROVEMENTS:
 - comments and linting

BUG FIXES:
 - [events] fix allocation error prefixing cache with 1000 empty events

## 0.3.2 (October 2, 2017)

BUG FIXES:

- [autofile] fix AutoFile.Sync() to open file if it's been closed
- [db] fix MemDb.Close() to not empty the database (ie. its just a noop)


## 0.3.1 (September 22, 2017)

BUG FIXES:

- [common] fix WriteFileAtomic to not use /tmp, which can be on another device

## 0.3.0 (September 22, 2017)

BREAKING CHANGES:

- [log] logger functions no longer returns an error
- [common] NewBaseService takes the new logger
- [cli] RunCaptureWithArgs now captures stderr and stdout
  - +func RunCaptureWithArgs(cmd Executable, args []string, env map[string]string) (stdout, stderr string, err error)
  - -func RunCaptureWithArgs(cmd Executable, args []string, env map[string]string) (output string, err error)

FEATURES:

- [common] various common HTTP functionality
- [common] Date range parsing from string (ex. "2015-12-31:2017-12-31")
- [common] ProtocolAndAddress function
- [pubsub] New package for publish-subscribe with more advanced filtering

BUG FIXES:

- [common] fix atomicity of WriteFileAtomic by calling fsync
- [db] fix memDb iteration index out of range
- [autofile] fix Flush by calling fsync

## 0.2.2 (June 16, 2017)

FEATURES:

- [common] IsHex and StripHex for handling `0x` prefixed hex strings
- [log] NewTracingLogger returns a logger that output error traces, ala `github.com/pkg/errors`

IMPROVEMENTS:

- [cli] Error handling for tests
- [cli] Support dashes in ENV variables

BUG FIXES:

- [flowrate] Fix non-deterministic test failures

## 0.2.1 (June 2, 2017)

FEATURES:

- [cli] Log level parsing moved here from tendermint repo

## 0.2.0 (May 18, 2017)

BREAKING CHANGES:

- [common] NewBaseService takes the new logger


FEATURES:

- [cli] New library to standardize building command line tools
- [log] New logging library

BUG FIXES:

- [autofile] Close file before rotating

## 0.1.0 (May 1, 2017)

Initial release, combines what were previously independent repos:

- go-autofile
- go-clist
- go-common
- go-db
- go-events
- go-flowrate
- go-logger
- go-merkle
- go-process





































































































































































