# Changelog

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





































































































































































