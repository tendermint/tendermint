# ADR 046: Lite Client Implementation

## Changelog
* 13-02-2020: Initial draft

## Context

A `Client` struct represents a light client, connected to a single blockchain.
As soon as it's started (via `Start`), it tries to update to the latest header
(using bisection algorithm by default).

Cleaning routine is also started to remove headers outside of trusting period.
NOTE: since it's periodic, we still need to check header is not expired in
`TrustedHeader`, `TrustedValidatorSet` methods (and others which are using the
latest trusted header).

The user has an option to manually verify headers using `VerifyHeader` and
`VerifyHeaderAtHeight` methods. To avoid races, `UpdatePeriod(0)` needs to be
passed when initializing the light client (it turns off the auto update).

```go
type Client interface {
	// start and stop updating & cleaning goroutines
	Start() error
	Stop()
	Cleanup() error

	// get trusted headers & validators
	TrustedHeader(height int64, now time.Time) (*types.SignedHeader, error)
	TrustedValidatorSet(height int64, now time.Time) (*types.ValidatorSet, error)
	LastTrustedHeight() (int64, error)
	FirstTrustedHeight() (int64, error)

	// query configuration options
	ChainID() string
	Primary() provider.Provider
	Witnesses() []provider.Provider

	// verify new headers
	VerifyHeaderAtHeight(height int64, now time.Time) (*types.SignedHeader, error)
	VerifyHeader(newHeader *types.SignedHeader, newVals *types.ValidatorSet, now time.Time) error
}
```

A new light client can either be created from scratch (via `NewClient`) or
using the trusted store (via `NewClientFromTrustedStore`). When there's some
data in the trusted store and `NewClient` is called, the light client will a)
check if stored header is more recent b) optionally ask the user whenever it
should rollback (no confirmation required by default).

```go
func NewClient(
	chainID string,
	trustOptions TrustOptions,
	primary provider.Provider,
	witnesses []provider.Provider,
	trustedStore store.Store,
	options ...Option) (*Client, error) {
```

`witnesses` as argument (as opposite to `Option`) is an intentional choice,
made to increase security by default. At least one witness is required,
although, right now, the light client does not check that primary != witness.
When cross-checking a new header with witnesses, minimum number of witnesses
required to respond: 1.

Due to bisection algorithm nature, some headers might be skipped. If the light
client does not have a header for height `X` and `TrustedHeader(X)` or
`TrustedValidatorSet(X)` methods are called, it will download the header from
primary provider and perform a backwards verification.

```go
type Provider interface {
	ChainID() string

	SignedHeader(height int64) (*types.SignedHeader, error)
	ValidatorSet(height int64) (*types.ValidatorSet, error)
}
```

Provider is a full node usually, but can be another light client. The above
interface is thin and can accommodate many implementations.

If provider (primary or witness) becomes unavailable for a prolonged period of
time, it will be removed to ensure smooth operation.

Both `Client` and providers expose chain ID to track if there are on the same
chain. Note, when chain upgrades or intentionally forks, chain ID changes.

The light client stores headers & validators in the trusted store:

```go
type Store interface {
	SaveSignedHeaderAndNextValidatorSet(sh *types.SignedHeader, valSet *types.ValidatorSet) error
	DeleteSignedHeaderAndNextValidatorSet(height int64) error

	SignedHeader(height int64) (*types.SignedHeader, error)
	ValidatorSet(height int64) (*types.ValidatorSet, error)

	LastSignedHeaderHeight() (int64, error)
	FirstSignedHeaderHeight() (int64, error)

	SignedHeaderAfter(height int64) (*types.SignedHeader, error)
}
```

At the moment, the only implementation is the `db` store (wrapper around the KV
database, used in Tendermint). In the future, remote adapters are possible
(e.g. `Postgresql`).

```go
func Verify(
	chainID string,
	h1 *types.SignedHeader,
	h1NextVals *types.ValidatorSet,
	h2 *types.SignedHeader,
	h2Vals *types.ValidatorSet,
	trustingPeriod time.Duration,
	now time.Time,
	trustLevel tmmath.Fraction) error {
```

`Verify` pure function is exposed for a header verification. It handles both
cases of adjacent and non-adjacent headers. In the former case, it compares the
hashes directly (2/3+ signed transition). Otherwise, it verifies 1/3+
(`trustLevel`) of trusted validators are still present in new validators.

## Status

Accepted.

## Consequences

### Positive

* single `Client` struct, which is easy to use
* flexible interfaces for header providers and trusted storage

### Negative

* `Verify` needs to be aligned with the current spec

### Neutral

* `Verify` function might be misused (called with non-adjacent headers in
  incorrectly implemented sequential verification)
