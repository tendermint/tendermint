# RFC 007: Don't Panic

## Changelog

- 2021-12-17: initial draft (@tychoish)

## Abstract

Today, the Tendermint core codebase has panics in a number of cases as
a response to exceptional situations. These panics complicate testing,
and might make tendermint components difficult to use as a library in
some circumstances. This document outlines a project of converting
panics to errors and describes the situations where its safe to
panic. 

## Background

Panics in Go are a great mechanism for aborting the current execution
for truly exceptional situations (e.g. memory errors, data corruption,
processes initialization); however, because they resemble exceptions
in other languages, it can be easy to over use them in the
implementation of software architectures. This certainly happened in
the history of Tendermint, and as we embark on the project of
stabilizing the package, we find ourselves in the right moment to
reexamine our use of panics, and largely where panics happen in the
code base.

There are still some situations where panics are acceptable and
desireable, but it's important that Tendermint, as a project, comes to
consensus--perhaps in the text of this document--on the situations
where it is acceptable to panic. 

### References

- [Defer Panic and Recover](https://go.dev/blog/defer-panic-and-recover)
- [Why Go gets exceptions right](https://dave.cheney.net/tag/panic)
- [Don't panic](https://dave.cheney.net/practical-go/presentations/gophercon-singapore-2019.html#_dont_panic)

## Discussion

### Acceptable Panics 

#### Initialization 

It is unambiguously safe (and desireable) to panic in `init()`
functions in response to any kind of error. These errors are caught by
tests, and occur early enough in process initialization that they
won't cause unexpected runtime crashes. 

Other code that is called early in process initialization MAY panic,
in some situations if it's not possible to return an error or cause
the process to abort early, although these situations should be
vanishingly slim.

#### Data Corruption 

If Tendermint code encounters an inconsistency that could be
attributed to data corruption or a logical impossibility it is safer
to panic and crash the process than continue to attempt to make
progress in these situations. 

Examples including reading data out of the storage engine that
is invalid or corrupt, or encountering an ambiguous situation where
the process should halt.

TODO: Add more and better examples here.

#### Request Handling

Code that handles responses to incoming/external requests
(e.g. `http.Handler`) should avoid panics, but practice this isn't
totally possible, and it makes sense that request handlers have some
kind of default recovery mechanism that will prevent one request from
terminating a service. 

### Unacceptable Panics

In **no** other situation is it acceptable for the code to panic: 

- ensuring that arbitrary go routines can't panic. 
- ensuring that there are no arbitrary panics in core production code,
  espically code that can run at any time during the. 
- all test code and fixture signal errors with a mechanism like
  testify's `require` assertion rather than calling panic.
