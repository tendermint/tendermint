# Changelog

## 0.5.0 (April 21, 2017)

BREAKING CHANGES: 

- Remove or unexport methods from FuzzedConnection: Active, Mode, ProbDropRW, ProbDropConn, ProbSleep, MaxDelayMilliseconds, Fuzz
- switch.AddPeerWithConnection is unexported and replaced by switch.AddPeer
- switch.DialPeerWithAddress takes a bool, setting the peer as persistent or not

FEATURES:

- Persistent peers: any peer considered a "seed" will be reconnected to when the connection is dropped


IMPROVEMENTS:

- Many more tests and comments
- Refactor configurations for less dependence on go-config. Introduces new structs PeerConfig, MConnConfig, FuzzConnConfig
- New methods on peer: CloseConn, HandshakeTimeout, IsPersistent, Addr, PubKey
- NewNetAddress supports a testing mode where the address defaults to 0.0.0.0:0


## 0.4.0 (March 6, 2017)

BREAKING CHANGES: 

- DialSeeds now takes an AddrBook and returns an error: `DialSeeds(*AddrBook, []string) error`
- NewNetAddressString now returns an error: `NewNetAddressString(string) (*NetAddress, error)`

FEATURES:

- `NewNetAddressStrings([]string) ([]*NetAddress, error)`
- `AddrBook.Save()`

IMPROVEMENTS:

- PexReactor responsible for starting and stopping the AddrBook

BUG FIXES:

- DialSeeds returns an error instead of panicking on bad addresses

## 0.3.5 (January 12, 2017)

FEATURES

- Toggle strict routability in the AddrBook 

BUG FIXES

- Close filtered out connections
- Fixes for MakeConnectedSwitches and Connect2Switches

## 0.3.4 (August 10, 2016)

FEATURES:

- Optionally filter connections by address or public key

## 0.3.3 (May 12, 2016)

FEATURES:

- FuzzConn

## 0.3.2 (March 12, 2016)

IMPROVEMENTS:

- Memory optimizations

## 0.3.1 ()

FEATURES: 

- Configurable parameters

