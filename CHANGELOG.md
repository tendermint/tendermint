# Changelog

## 0.3.0 (August 25, 2017)

BREAKING CHANGES:

- [common] NewBaseService takes the new logger
- [cli] RunCaptureWithArgs now captures stderr and stdout
  - +func RunCaptureWithArgs(cmd Executable, args []string, env map[string]string) (stdout, stderr string, err error)
  - -func RunCaptureWithArgs(cmd Executable, args []string, env map[string]string) (output string, err error) 

FEATURES:

- [common] various common HTTP functionality
- [common] Date range parsing from string (ex. "2015-12-31:2017-12-31") 
- [pubsub] New package for publish-subscribe with more advanced filtering

BUG FIXES:

- [common] fix atomicity of WriteFileAtomic by calling fsync
- [db] fix memDb iteration index out of range

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





































































































































































