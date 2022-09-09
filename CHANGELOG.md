## [0.10.0-dev.1] - 2022-09-09

### Bug Fixes

- Go lint issues (#455)

### Documentation

- Same-block execution docs and protobuf cleanup (#454)

### Features

- Add node's pro-tx-hash into a context (#416)
- Same-block execution (#418)

### Backport

- Tendermint v0.36 (#446)

## [0.9.0-dev.1] - 2022-09-01

### Bug Fixes

- Invalid initial height in e2e vote extensions test (#419)

### Miscellaneous Tasks

- Update changelog and version to 0.9.0-dev.1

### Refactor

- Allow set 0 for 'create-proof-block-range' to ignore proof block app hash
- Start test of proof-block range from 0 height

### Build

- Bump actions/setup-go from 3.2.0 to 3.2.1

## [0.8.0-dev.6] - 2022-07-15

### Bug Fixes

- Release script tries to use non-existing file
- Go link issues
- Data-race issue
- Applied changes according to PR feedback
- Make NewSignItem and MakeSignID exported, revert to precalculate hash for SignItem
- Quorum_sign_data_test.go
- Lint issue
- Check a receiver of ValidatorSet on nil

### Features

- Add missed fields (CoreChainLockedHeight, ProposerProTxHash and ProposedAppVersion) to RequestFinalizeBlock and PrepareProposal

### Miscellaneous Tasks

- Preallocate the list
- Fix unit tests
- Fix unit tests
- Some modification after self-review
- Remove ThresholdVoteExtension as redundant, use VoteExtension instead
- Update order fields initialization
- Update abci++ spec
- Update changelog and version to 0.8.0-dev.6

### Refactor

- Separate default and threshold-recover extensions between 2 independent list, persist threshold vote extensions with a commit
- Revert vote-extension protobuf structures to previous version
- The changes by PR feedback
- DashCoreSignerClient should return correct private key
- Modifications after merge
- Abci app expects tendermint.version.Consensus rather than proposed-app-version in RequestFinalizeBlock and RequestPrepareProposal
- Revert proposed_app_version

## [0.8.0-dev.5] - 2022-06-14

### Bug Fixes

- Consolidate all prerelease changes in latest full release changelog
- First part of modification after merge
- Mishandled pubkey read errors
- Eliminate compile level issues
- Unit tests in abci/example/kvstore
- Unit tests in dash/quorum package
- Deadlock at types.MockPV
- Blocksync package
- Evidence package
- Made some fixes/improvements
- Change a payload hash of a message vote
- Remove using a mutex in processPeerUpdate to fix a deadlock
- Remove double incrementing
- Some modifications for fixing unit tests
- Modify TestVoteString
- Some fixes / improvements
- Some fixes / improvements
- Override genesis time for pbst tests
- Pbst tests
- Disable checking duplicate votes
- Use the current time always when making proposal block
- Consensus state tests
- Consensus state tests
- Consensus state tests
- The tests inside state package
- Node tests
- Add custom marshalling/unmarshalling for coretypes.ResultValidators
- Add checking on nil in Vote.MarshalZerologObject
- Light client tests
- Rpc tests
- Remove duplicate test TestApp_Height
- Add mutex for transport_mconn.go
- Add required option "create-proof-block-range" in a config testdata
- Type error in generateDuplicateVoteEvidence
- Use thread safe way for interacting with consensus state
- Use a normal time ticker for some consensus unit tests
- E2e tests
- Lint issues
- Abci-cli
- Detected data race
- TestBlockProtoBuf
- Lint (proto and golang) modifications
- ProTxHash not correctly initialized
- Lint issue
- Proto lint
- Reuse setValSetUpdate to update validator index and validator-set-updates item in a storage
- Reuse setValSetUpdate to update validator index and validator-set-updates item in a storage
- Fix dependencies in e2e tests
- Install libpcap-dev before running go tests
- Install missing dependencies for linter
- Fix race conditions in reactor
- Specify alpine 3.15 in Dockerfile

### Documentation

- Abcidump documentation

### Features

- Abci protocol parser
- Abci protocol parser - packet capture
- Parse CBOR messages

### Miscellaneous Tasks

- Don't fail due to missing bodyclose in go 1.18
- Remove printing debug stacktrace for a duplicate vote
- Remove redundant mock cons_sync_reactor.go
- Remove github CI docs-toc.yml workflow
- Refactor e2e initialization
- Fix whitespace and comments
- Add unit tests for TestMakeBlockSignID, TestMakeStateSignID, TestMakeVoteExtensionSignIDs
- Some naming modifications
- Add verification for commit vote extension threshold signatures
- Modify a condition in VoteExtSigns2BytesSlices
- Remove recoverableVoteExtensionIndexes
- Some improvements
- Cleanup during self-review
- Remove duplicate test
- Update go.mod
- Update changelog and version to 0.8.0-dev.5
- Update changelog and version to 0.8.0-dev.5

### Refactor

- Single vote-extension field was modified on multiple ones. support default and threshold-recover types of extensions
- Simplify priv validator initialization code
- Add a centralized way for recovering threshold signatures, add a way of creating sign ids, refactor code to use one way of making sign data and recovering signs
- Standardize the naming of functions, variables
- Add some modifications by RP feedback
- Refactor cbor and apply review feedback
- Move abcidump from scripts/ to cmd/

### Security

- Merge result of tendermint/master with v0.8-dev (#376)

### Testing

- Use correct home path in TestRootConfig
- Add cbor test
- Add parse cmd test
- Test parser NewMessageType
- Test parser
- Replace hardcoded input data

### Backport

- Upgrade logging to v0.8
- Update for new logging

### Build

- Bump docker/build-push-action from 2.9.0 to 3.0.0
- Bump docker/login-action from 1.14.1 to 2.0.0
- Bump docker/setup-buildx-action from 1.6.0 to 2.0.0
- Use golang 1.18
- Upgrade golangci-lint to 1.46
- Bump actions/setup-go from 2 to 3.1.0
- Bump golangci/golangci-lint-action from 3.1.0 to 3.2.0
- Bump actions/setup-go from 3.1.0 to 3.2.0
- Bump github.com/golangci/golangci-lint

## [0.8.0-dev.4] - 2022-05-04

### Bug Fixes

- Add a missed "info" field to broadcast-tx-response (#369)

### Miscellaneous Tasks

- Update changelog and version to 0.8.0-dev.4 (#370)

### PBTS

- System model made more precise (#8096)

### Security

- Bump bufbuild/buf-setup-action from 1.3.1 to 1.4.0 (#8405)
- Bump codecov/codecov-action from 3.0.0 to 3.1.0 (#8406)
- Bump google.golang.org/grpc from 1.45.0 to 1.46.0 (#8408)
- Bump github.com/vektra/mockery/v2 from 2.12.0 to 2.12.1 (#8417)
- Bump github.com/google/go-cmp from 0.5.7 to 0.5.8 (#8422)
- Bump github.com/creachadair/tomledit from 0.0.18 to 0.0.19 (#8440)
- Bump github.com/btcsuite/btcd from 0.22.0-beta to 0.22.1 (#8439)
- Bump docker/setup-buildx-action from 1.6.0 to 1.7.0 (#8451)

### Abci

- Application type should take contexts (#8388)
- Application should return errors errors and nilable response objects (#8396)
- Remove redundant methods in client (#8401)
- Remove unneccessary implementations (#8403)
- Interface should take pointers to arguments (#8404)

### Abci++

- Remove intermediate protos (#8414)
- Vote extension cleanup (#8402)

### Backport

- V0.7.1 into v0.8-dev (#361)

### Blocksync

- Honor contexts supplied to BlockPool (#8447)

### Config

- Minor template infrastructure (#8411)

### Consensus

- Reduce size of validator set changes test (#8442)

### Crypto

- Remove unused code (#8412)
- Cleanup tmhash package (#8434)

### Fuzz

- Don't panic on expected errors (#8423)

### Node

- Start rpc service after reactors (#8426)

### P2p

- Remove support for multiple transports and endpoints (#8420)
- Use nodeinfo less often (#8427)
- Avoid using p2p.Channel internals (#8444)

### Privval/grpc

- Normalize signature (#8441)

### Rpc

- Fix byte string decoding for URL parameters (#8431)

## [0.8.0-dev.3] - 2022-04-22

### Miscellaneous Tasks

- Update changelog and version to 0.8.0-dev.3

### Build

- Bump github.com/vektra/mockery/v2 from 2.11.0 to 2.12.0 (#8393)

## [0.8.0-dev.2] - 2022-04-22

### Bug Fixes

- Network stuck due to outdated proposal block (#327)
- Don't process WAL logs for old rounds (#331)
- Use thread-safely way to get pro-tx-hash from peer-state (#344)
- Slightly modify a way of interacting with p2p channels in consensus reactor (#357)
- Remove select block to don't block sending a witness response (#336)
- Unsupported priv validator type - dashcore.RPCClient (#353)

### Miscellaneous Tasks

- Update changelog and version to 0.7.1
- If the tenderdash source code is not tracked by git then cloning "develop_0.1" branch as fallback scenario to build a project (#356)
- If the tenderdash source code is not tracked by git then cloning "develop_0.1" branch as fallback scenario to build a project (#355)
- Update changelog and version to 0.8.0-dev.2 (#333)

### Refactor

- Consolidate redundant code (#322)

### Security

- Bump github.com/lib/pq from 1.10.4 to 1.10.5 (#8283)
- Bump codecov/codecov-action from 2.1.0 to 3.0.0 (#8306)
- Bump actions/setup-go from 2 to 3 (#8305)
- Bump actions/stale from 4 to 5 (#8304)
- Bump actions/download-artifact from 2 to 3 (#8302)
- Bump actions/upload-artifact from 2 to 3 (#8303)
- Bump github.com/creachadair/tomledit from 0.0.11 to 0.0.13 (#8307)
- Bump github.com/vektra/mockery/v2 from 2.10.4 to 2.10.6 (#8346)
- Bump github.com/spf13/viper from 1.10.1 to 1.11.0 (#8344)
- Bump github.com/creachadair/atomicfile from 0.2.4 to 0.2.5 (#8365)
- Bump github.com/vektra/mockery/v2 from 2.10.6 to 2.11.0 (#8374)
- Bump github.com/creachadair/tomledit from 0.0.16 to 0.0.18 (#8392)

### Testing

- Update oss-fuzz build script to match reality (#8296)
- Convert to Go 1.18 native fuzzing (#8359)
- Remove debug logging statement (#8385)

### Abci

- Avoid having untracked requests in the channel (#8382)
- Streamline grpc application construction (#8383)

### Abci++

- Only include meaningful header fields in data passed-through to application (#8216)
- Sync implementation and spec for vote extensions (#8141)

### Build

- Implement full release workflow in the release script (#332)
- Use go install instead of go get. (#8299)
- Implement full release workflow in the release script (#332) (#345)
- Implement full release workflow in the release script (#332) (#345)
- Bump async from 2.6.3 to 2.6.4 in /docs (#8357)

### Cleanup

- Unused parameters (#8372)
- Pin get-diff-action uses to major version only, not minor/patch (#8368)

### Cli

- Add graceful catches to SIGINT (#8308)
- Simplify resetting commands (#8312)

### Confix

- Clean up and document transformations (#8301)
- Remove mempool.version in v0.36 (#8334)
- Convert tx-index.indexer from string to array (#8342)

### Consensus

- Add nil check to gossip routine (#8288)

### Eventbus

- Publish without contexts (#8369)

### Events

- Remove unused event code (#8313)

### Keymigrate

- Fix decoding of block-hash row keys (#8294)
- Fix conversion of transaction hash keys (#8352)

### Node

- Move handshake out of constructor (#8264)
- Use signals rather than ephemeral contexts (#8376)
- Cleanup setup for indexer and evidence components (#8378)

### Node+statesync

- Normalize initialization (#8275)

### P2p

- Fix setting in con-tracker (#8370)

### Pubsub

- [minor] remove unused stub method (#8316)

### Rpc

- Add more nil checks in the status end point (#8287)
- Avoid leaking threads (#8328)
- Reformat method signatures and use a context (#8377)

### Scmigrate

- Ensure target key is correctly renamed (#8276)

### Service

- Minor cleanup of comments (#8314)

### State

- Remove unused weighted time (#8315)

### Statesync+blocksync

- Move event publications into the sync operations (#8274)

## [0.7.1-dev.1] - 2022-04-07

### Bug Fixes

- Remove option c form linux build (#305)
- Cannot read properties of undefined
- Network stuck due to outdated proposal block (#327)
- Don't process WAL logs for old rounds (#331)

### Documentation

- Go tutorial fixed for 0.35.0 version (#7329) (#7330) (#7331)
- Update go ws code snippets (#7486) (#7487)
- Remove spec section from v0.35 docs (#7899)

### Miscellaneous Tasks

- Temporarily disable ARM build which is broken
- Backport Tendermint 0.35.1 to Tenderdash 0.8 (#309)
- Update CI e2e action workflow (#319)
- Change dockerhub build target
- Inspect context
- Bump golang version
- Remove debug
- Use gha cache from docker
- Revert dev changes
- Remove obsolete cache step

### Refactor

- [**breaking**] Replace is-masternode config with mode=validator (#308)
- Add MustPubKeyToProto helper function (#311)
- Implementing LLMQ generator (#310)
- Move bls CI code to a separate action and improve ARM build (#314)
- Persistent kvstore abci (#313)
- Improve statesync.backfill (#316)
- Small improvement in test four add four minus one genesis validators (#318)

### Security

- Bump github.com/golangci/golangci-lint from 1.45.0 to 1.45.2 (#8192)
- Bump github.com/adlio/schema from 1.2.3 to 1.3.0 (#8201)
- Bump github.com/vektra/mockery/v2 from 2.10.0 to 2.10.1 (#8226)
- Bump github.com/vektra/mockery/v2 from 2.10.1 to 2.10.2 (#8246)
- Bump github.com/vektra/mockery/v2 from 2.10.2 to 2.10.4 (#8250)
- Bump github.com/BurntSushi/toml from 1.0.0 to 1.1.0 (#8251)

### Testing

- Fix validator conn executor test backport
- Update mockery mocks
- Fix test test_abci_cli

### Abci++

- Correct max-size check to only operate on added and unmodified (#8242)

### Backport

- Add basic metrics to the indexer package. (#7250) (#7252)

### Build

- Bump google.golang.org/grpc from 1.41.0 to 1.42.0 (#7218)
- Bump github.com/lib/pq from 1.10.3 to 1.10.4
- Bump github.com/tendermint/tm-db from 0.6.4 to 0.6.6 (#7285)
- Bump minimist from 1.2.5 to 1.2.6 in /docs (#8196)
- Bump bufbuild/buf-setup-action from 1.1.0 to 1.3.0 (#8199)
- Bump github.com/spf13/viper from 1.9.0 to 1.10.0 (#7435)
- Bump github.com/adlio/schema from 1.2.2 to 1.2.3 (#7436)
- Bump github.com/spf13/cobra from 1.2.1 to 1.3.0 (#7457)
- Bump github.com/rs/zerolog from 1.26.0 to 1.26.1 (#7467)
- Downgrade tm-db from v0.6.7 to v0.6.6
- Use Go 1.18 to fix issue building curve25519-voi
- Bump bufbuild/buf-setup-action from 1.3.0 to 1.3.1 (#8245)
- Provide base branch to make as variable (#321)

### Ci

- Move test execution to makefile (#7372) (#7374)
- Update mergify for tenderdash 0.8
- Cleanup build/test targets (backport #7393) (#7395)
- Skip docker image builds during PRs (#7397) (#7398)
- Fix super-linter configuration settings (backport #7708) (#7710)
- Fixes for arm builds

### Cmd

- Cosmetic changes for errors and print statements (#7377) (#7408)
- Add integration test for rollback functionality (backport #7315) (#7369)

### Config

- Add a Deprecation annotation to P2PConfig.Seeds. (#7496) (#7497)
- Default indexer configuration to null (#8222)

### Consensus

- Add some more checks to vote counting (#7253) (#7262)
- Timeout params in toml used as overrides (#8186)
- Additional timing metrics (backport #7849) (#7875)
- Remove string indented function (#8257)
- Avoid panics during handshake (#8266)

### E2e

- Stabilize validator update form (#7340) (#7351)
- Clarify apphash reporting (#7348) (#7352)
- Generate keys for more stable load (#7344) (#7353)
- App hash test cleanup (0.35 backport) (#7350)
- Fix hashing for app + Fix logic of TestApp_Hash (#8229)

### Evidence

- Remove source of non-determinism from test (#7266) (#7268)

### Internal/libs/protoio

- Optimize MarshalDelimited by plain byteslice allocations+sync.Pool (#7325) (#7426)

### Internal/proxy

- Add initial set of abci metrics backport (#7342)

### Light

- Remove untracked close channel (#8228)

### Lint

- Remove lll check (#7346) (#7357)
- Bump linter version in ci (#8234)

### Migration

- Remove stale seen commits (#8205)

### Node

- Remove channel and peer update initialization from construction (#8238)
- Reorder service construction (#8262)

### P2p

- Reduce peer score for dial failures (backport #7265) (#7271)
- Plumb rudamentary service discovery to rectors and update statesync (backport #8030) (#8036)
- Update shim to transfer information about peers (#8047)
- Inject nodeinfo into router (#8261)

### Pubsub

- Report a non-nil error when shutting down. (#7310)

### Rpc

- Backport experimental buffer size control parameters from #7230 (tm v0.35.x) (#7276)
- Implement header and header_by_hash queries (backport #7270) (#7367)

### State

- Avoid premature genericism (#8224)

### Statesync

- Assert app version matches (backport #7856) (#7886)
- Avoid compounding retry logic for fetching consensus parameters (backport #8032) (#8041)
- Merge channel processing (#8240)
- Tweak test performance (#8267)

### Types

- Fix path handling in node key tests (#7493) (#7502)

## [0.8.0-dev.1] - 2022-03-24

### ABCI++

- Major refactor of spec's structure. Addressed Josef's comments. Merged ABCI's methods and data structs that didn't change. Added introductory paragraphs
- Found a solution to set the execution mode
- Update new protos to use enum instead of bool (#8158)

### ADR

- Synchronize PBTS ADR with spec (#7764)
- Protocol Buffers Management (#8029)

### ADR-74

- Migrate Timeout Parameters to Consensus Parameters (#7503)

### Bug Fixes

- Detect and fix data-race in MockPV (#262)
- Race condition when logging (#271)
- Decrease memory used by debug logs (#280)
- Tendermint stops when validator node id lookup fails (#279)
- Backport e2e tests (#248)

### Docs

- Abci++ typo (#8147)

### Documentation

- Fixup the builtin tutorial  (#7488)
- Fix some typos in ADR 075. (#7726)
- Drop v0.32 from the doc site configuration (#7741)
- Fix RPC output examples for GET queries (#7799)
- Fix ToC file extension for RFC 004. (#7813)
- Rename RFC 008 (#7841)
- Fix broken markdown links (cherry-pick of #7847) (#7848)
- Fix broken markdown links (#7847)
- Update spec links to point to tendermint/tendermint (#7851)
- Remove unnecessary os.Exit calls at the end of main (#7861)
- Fix misspelled file name (#7863)
- Remove spec section from v0.35 docs (#7899)
- Pin the RPC docs to v0.35 instead of master (#7909)
- Pin the RPC docs to v0.35 instead of master (backport #7909) (#7911)
- Update repo and spec readme's (#7907)
- Redirect master links to the latest release version (#7936)
- Redirect master links to the latest release version (backport #7936) (#7954)
- Fix cosmos theme version. (#7966)
- Point docs/master to the same content as the latest release (backport #7980) (#7998)
- Fix some broken markdown links (#8021)
- Update ADR template (#7789)
- Add an overview of the proposer-based timestamps algorithm (#8058)
- PBTS synchrony issues runbook (#8129)

### Miscellaneous Tasks

- Create only 1 proof block by default
- Release script and initial changelog (#250)
- [**breaking**] Bump ABCI version and update release.sh to change TMVersionDefault automatically (#253)
- Eliminate compile errors after backport of tendermint 0.35 (#238)
- Update changelog and version to 0.7.0
- Update unit tests after backport fo tendermint v0.35 (#245)
- Backport Tenderdash 0.7 to 0.8 (#246)
- Fix e2e tests and protxhash population (#273)
- Improve logging for debug purposes
- Stabilize consensus algorithm (#284)

### PBTS

- Spec reorganization, summary of changes on README.md (#399)

### RFC

- Add delete gas rfc (#7777)

### RFC-009

- Consensus Parameter Upgrades (#7524)

### Refactor

- Change node's proTxHash on slice from pointer of slice (#263)
- Some minor changes in validate-conn-executor and routerDashDialer (#277)
- Populate proTxHash in address-book (#274)
- Replace several functions with an identical body (processStateCh,processDataCh,processVoteCh,processVoteSetBitsCh) on one function processMsgCh (#296)

### Security

- Bump github.com/BurntSushi/toml from 0.4.1 to 1.0.0 (#7562)
- Bump docker/build-push-action from 2.7.0 to 2.8.0 (#7679)
- Bump github.com/vektra/mockery/v2 from 2.9.4 to 2.10.0 (#7685)
- Bump github.com/golangci/golangci-lint from 1.43.0 to 1.44.0 (#7692)
- Bump github.com/prometheus/client_golang from 1.12.0 to 1.12.1 (#7732)
- Bump github.com/gorilla/websocket from 1.4.2 to 1.5.0 (#7829)
- Bump github.com/golangci/golangci-lint from 1.44.0 to 1.44.2 (#7854)
- Bump golangci/golangci-lint-action from 2.5.2 to 3.1.0 (#8026)
- Bump actions/checkout from 2.4.0 to 3 (#8076)
- Bump docker/login-action from 1.13.0 to 1.14.1 (#8075)
- Bump golangci/golangci-lint-action from 2.5.2 to 3.1.0 (#8074)
- Bump google.golang.org/grpc from 1.44.0 to 1.45.0 (#8104)
- Bump github.com/spf13/cobra from 1.3.0 to 1.4.0 (#8109)
- Bump github.com/stretchr/testify from 1.7.0 to 1.7.1 (#8131)
- Bump gaurav-nelson/github-action-markdown-link-check from 1.0.13 to 1.0.14 (#8166)
- Bump docker/build-push-action from 2.9.0 to 2.10.0 (#8167)
- Bump github.com/golangci/golangci-lint from 1.44.2 to 1.45.0 (#8169)

### Testing

- Pass testing.T around rather than errors for test fixtures (#7518)
- Uniquify prom IDs (#7540)
- Remove in-test logging (#7558)
- Use noop loger with leakteset in more places (#7604)
- Update docker versions to match build version (#7646)
- Update cleanup opertunities (#7647)
- Reduce timeout to 4m from 8m (#7681)
- Reduce usage of the MustDefaultLogger constructor (#7960)
- Logger cleanup (#8153)
- KeepInvalidTxsInCache test is invalid

### Abci

- Socket server shutdown response handler (#7547)
- PrepareProposal (#6544)
- Vote Extension 1 (#6646)
- PrepareProposal-VoteExtension integration [2nd try] (#7821)
- Undo socket buffer limit (#7877)
- Make tendermint example+test clients manage a mutex (#7978)
- Remove lock protecting calls to the application interface (#7984)
- Use no-op loggers in the examples (#7996)
- Revert buffer limit change (#7990)
- Synchronize FinalizeBlock with the updated specification (#7983)

### Abci++

- Synchronize PrepareProposal with the newest version of the spec (#8094)
- Remove app_signed_updates (#8128)
- Remove CheckTx call from PrepareProposal flow (#8176)

### Abci/client

- Use a no-op logger in the test (#7633)
- Simplify client interface (#7607)
- Remove vestigially captured context (#7839)
- Remove waitgroup for requests (#7842)
- Remove client-level callback (#7845)
- Make flush operation sync (#7857)
- Remove lingering async client code (#7876)

### Abci/kvstore

- Test cleanup improvements (#7991)

### Adr

- Merge tendermint/spec repository into tendermint/tendermint (#7775)

### Autofile

- Ensure files are not reopened after closing (#7628)
- Avoid shutdown race (#7650)
- Reduce minor panic and docs changes (#8122)
- Remove vestigal close mechanism (#8150)

### Blocksync

- Standardize construction process (#7531)
- Shutdown cleanup (#7840)
- Drop redundant shutdown mechanisms (#8136)
- Remove intermediate channel (#8140)

### Build

- Bump technote-space/get-diff-action from 5 to 6.0.1 (#7535)
- Bump github.com/BurntSushi/toml from 0.4.1 to 1.0.0 (#7560)
- Make sure to test packages with external tests (#7608)
- Make sure to test packages with external tests (backport #7608) (#7635)
- Bump github.com/prometheus/client_golang (#7636)
- Bump github.com/prometheus/client_golang (#7637)
- Bump docker/build-push-action from 2.7.0 to 2.8.0 (#389)
- Bump github.com/prometheus/client_golang (#249)
- Bump github.com/BurntSushi/toml from 0.4.1 to 1.0.0
- Bump vuepress-theme-cosmos from 1.0.182 to 1.0.183 in /docs (#7680)
- Bump github.com/vektra/mockery/v2 from 2.9.4 to 2.10.0 (#7684)
- Bump google.golang.org/grpc from 1.43.0 to 1.44.0 (#7693)
- Bump github.com/golangci/golangci-lint (#7696)
- Bump google.golang.org/grpc from 1.43.0 to 1.44.0 (#7695)
- Bump github.com/prometheus/client_golang (#7731)
- Bump docker/build-push-action from 2.8.0 to 2.9.0 (#397)
- Bump docker/build-push-action from 2.8.0 to 2.9.0 (#7780)
- Bump github.com/gorilla/websocket from 1.4.2 to 1.5.0 (#7830)
- Bump docker/build-push-action from 2.7.0 to 2.9.0
- Bump github.com/gorilla/websocket from 1.4.2 to 1.5.0
- Bump github.com/rs/zerolog from 1.26.0 to 1.26.1
- Bump actions/github-script from 5 to 6
- Bump docker/login-action from 1.10.0 to 1.12.0
- Bump url-parse from 1.5.4 to 1.5.7 in /docs (#7855)
- Bump github.com/golangci/golangci-lint (#7853)
- Bump docker/login-action from 1.12.0 to 1.13.0
- Bump docker/login-action from 1.12.0 to 1.13.0 (#7890)
- Bump prismjs from 1.26.0 to 1.27.0 in /docs (#8022)
- Bump url-parse from 1.5.7 to 1.5.10 in /docs (#8023)
- Bump docker/login-action from 1.13.0 to 1.14.1
- Bump golangci/golangci-lint-action from 2.5.2 to 3.1.0

### Ci

- Fix super-linter configuration settings (#7708)
- Fix super-linter configuration settings (backport #7708) (#7710)

### Cleanup

- Remove commented code (#8123)

### Clist

- Reduce size of test workload for clist implementation (#7682)
- Remove unused waitgroup from clist implementation (#7843)

### Cmd

- Avoid package state in cli constructors (#7719)
- Make reset more safe (#8081)

### Cmd/debug

- Remove global variables and logging (#7957)

### Conensus

- Put timeouts on reactor tests (#7733)

### Config

- Add event subscription options and defaults (#7930)

### Consensus

- Use noop logger for WAL test (#7580)
- Explicit test timeout (#7585)
- Test shutdown to avoid hangs (#7603)
- Calculate prevote message delay metric (#7551)
- Check proposal non-nil in prevote message delay metric (#7625)
- Calculate prevote message delay metric (backport #7551) (#7618)
- Check proposal non-nil in prevote message delay metric (#7625) (#7632)
- Use delivertxsync (#7616)
- Fix height advances in test state (#7648)
- Use buffered channel in TestStateFullRound1 (#7668)
- Remove unused closer construct (#7734)
- Delay start of peer routines (#7753)
- Delay start of peer routines (backport of #7753) (#7760)
- Tie peer threads to peer lifecylce context (#7792)
- Refactor operations in consensus queryMaj23Routine (#7791)
- Refactor operations in consensus queryMaj23Routine (backport #7791) (#7793)
- Start the timeout ticker before replay (#7844)
- Additional timing metrics (#7849)
- Additional timing metrics (backport #7849) (#7875)
- Improve cleanup of wal tests (#7878)
- HasVoteMessage index boundary check (#7720)
- TestReactorValidatorSetChanges test fix (#7985)
- Make orchestration more reliable for invalid precommit test (#8013)
- Validator set changes test cleanup (#8035)
- Improve wal test cleanup (#8059)
- Fix TestInvalidState race and reporting (#8071)
- Ensure the node terminates on consensus failure (#8111)
- Avoid extra close channel (#8144)
- Avoid persistent kvstore in tests (#8148)
- Avoid race in accessing channel (#8149)
- Skip channel close during shutdown (#8155)
- Change lock handling in reactor and handleMsg for RoundState (forward-port #7994 #7992) (#8139)
- Reduce size of test fixtures and logging rate (#8172)
- Avoid panic during shutdown (#8170)
- Cleanup tempfile explictly (#8184)
- Add leaktest check to replay tests (#8185)
- Update state machine to use the new consensus params (#8181)

### Consensus/state

- Avert a data race with state update and tests (#7643)

### Context

- Cleaning up context dead ends (#7963)

### E2e

- Plumb logging instance (#7958)
- Change ci network configuration (#7988)

### Events

- Remove service aspects of event switch (#8146)

### Evidence

- Reactor constructor (#7533)
- Refactored the evidence message to process Evidence instead of EvidenceList (#7700)
- Manage and initialize state objects more clearly in the pool (#8080)

### Github

- Update e2e workflows (#7803)
- Add Informal code owners (#8042)

### Indexer

- Skip Docker tests when Docker is not available (#7814)

### Internal/libs

- Delete unused functionality (#7569)

### Jsontypes

- Improve tests and error diagnostics (#7669)

### Libs/cli

- Clean up package (#7806)

### Libs/clist

- Remove unused surface area (#8134)

### Libs/events

- Remove unused event cache (#7807)
- Remove unneccessary unsubscription code (#8135)

### Libs/log

- Remove Must constructor (#8120)

### Libs/service

- Regularize Stop semantics and concurrency primitives (#7809)

### Libs/strings

- Cleanup string helper function package (#7808)

### Light

- Avoid panic for integer underflow (#7589)
- Remove test panic (#7588)
- Convert validation panics to errors (#7597)
- Fix provider error plumbing (#7610)
- Return light client status on rpc /status  (#7536)
- Fix absence proof verification by light client (#7639)
- Fix absence proof verification by light client (backport #7639) (#7716)
- Remove legacy timeout scheme (#7776)
- Remove legacy timeout scheme (backport #7776) (#7786)
- Avert a data race (#7888)

### Log

- Remove support for traces (#7542)
- Avoid use of legacy test logging (#7583)

### Logging

- Remove reamining instances of SetLogger interface (#7572)
- Allow logging level override (#7873)

### Math

- Remove panics in safe math ops (#7962)

### Mempool

- Refactor mempool constructor (#7530)
- Reactor concurrency test tweaks (#7651)
- Return duplicate tx errors more consistently (#7714)
- Return duplicate tx errors more consistently (backport #7714) (#7718)
- IDs issue fixes (#7763)
- Remove duplicate tx message from reactor logs (#7795)
- Fix benchmark CheckTx for hitting the GetEvictableTxs call (#7796)
- Use checktx sync calls (#7868)
- Test harness should expose application (#8143)
- Reduce size of test (#8152)

### Mempool+evidence

- Simplify cleanup (#7794)

### Metrics

- Add metric for proposal timestamp difference  (#7709)

### Node

- New concrete type for seed node implementation (#7521)
- Move seed node implementation to its own file (#7566)
- Collapse initialization internals (#7567)
- Allow orderly shutdown if context is canceled and gensis is in the future (#7817)
- Clarify unneccessary logic in seed constructor (#7818)
- Hook up eventlog and eventlog metrics (#7981)
- Excise node handle within rpc env (#8063)
- Nodes should fetch state on startup (#8062)
- Pass eventbus at construction time (#8084)
- Cleanup evidence db (#8119)
- Always sync with the application at startup (#8159)

### Node+autofile

- Avoid leaks detected during WAL shutdown (#7599)

### Node+privval

- Refactor privval construction (#7574)

### Node+rpc

- Rpc environment should own it's creation (#7573)

### P2p

- Always advertise self, to enable mutual address discovery (#7620)
- Always advertise self, to enable mutual address discovery (#7594)
- Pass start time to flowrate and cleanup constructors (#7838)
- Make mconn transport test less flaky (#7973)
- Mconn track last message for pongs (#7995)
- Relax pong timeout (#8007)
- Backport changes in ping/pong tolerances (#8009)
- Retry failed connections slightly more aggressively (#8010)
- Retry failed connections slightly more aggressively (backport #8010) (#8012)
- Ignore transport close error during cleanup (#8011)
- Plumb rudamentary service discovery to rectors and update statesync (#8030)
- Plumb rudamentary service discovery to rectors and update statesync (backport #8030) (#8036)
- Re-enable tests previously disabled (#8049)
- Update shim to transfer information about peers (#8047)
- Update polling interval calculation for PEX requests (#8106)
- Remove unnecessary panic handling in PEX reactor (#8110)
- Adjust max non-persistent peer score (#8137)

### P2p+flowrate

- Rate control refactor (#7828)

### P2p/message

- Changed evidence message to contain evidence, not a list… (#394)

### Params

- Increase default synchrony params (#7704)

### Pex

- Regularize reactor constructor (#7532)
- Avert a data race on map access in the reactor (#7614)
- Do not send nil envelopes to the reactor (#7622)
- Improve handling of closed channels (#7623)

### Privval

- Improve client shutdown to prevent resource leak (#7544)
- Synchronize leak check with shutdown (#7629)
- Do not use old proposal timestamp (#7621)
- Avoid re-signing vote when RHS and signbytes are equal (#7592)

### Proto

- Merge the proposer-based timestamps parameters (#393)
- Abci++ changes (#348)
- Update proto generation to use buf (#7975)

### Protoio

- Fix incorrect test assertion (#7606)

### Proxy

- Fix endblock metric (#7989)
- Collapse triforcated abci.Client (#8067)

### Pubsub

- Use concrete queries instead of an interface (#7686)
- Check for termination in UnsubscribeAll (#7820)

### Reactors

- Skip log on some routine cancels (#7556)

### Readme

- Add vocdoni (#8117)

### Rfc

- P2p light client (#7672)
- RFC 015 ABCI++ Tx Mutation (#8033)

### Roadmap

- Update to better reflect v0.36 changes (#7774)

### Rollback

- Cleanup second node during test (#8175)

### Rpc

- Remove positional parameter encoding from clients (#7545)
- Collapse Caller and HTTPClient interfaces. (#7548)
- Simplify the JSON-RPC client Caller interface (#7549)
- Replace anonymous arguments with structured types (#7552)
- Refactor the HTTP POST handler (#7555)
- Replace custom context-like argument with context.Context (#7559)
- Remove cache control settings from the HTTP server (#7568)
- Fix mock test cases (#7571)
- Rework how responses are written back via HTTP (#7575)
- Simplify panic recovery in the server middleware (#7578)
- Consolidate RPC route map construction (#7582)
- Clean up the RPCFunc constructor signature (#7586)
- Check RPC service functions more carefully (#7587)
- Update fuzz criteria to match the implementation (#7595)
- Remove dependency of URL (GET) requests on tmjson (#7590)
- Simplify the encoding of interface-typed arguments in JSON (#7600)
- Paginate mempool /unconfirmed_txs endpoint (#7612)
- Use encoding/json rather than tmjson (#7670)
- Check error code for broadcast_tx_commit (#7683)
- Check error code for broadcast_tx_commit (#7683) (#7688)
- Add application info to `status` call (#7701)
- Remove unused websocket options (#7712)
- Clean up unused non-default websocket client options (#7713)
- Don't route websocket-only methods on GET requests (#7715)
- Clean up encoding of request and response messages (#7721)
- Simplify and consolidate response construction (#7725)
- Clean up unmarshaling of batch-valued responses (#7728)
- Simplify the handling of JSON-RPC request and response IDs (#7738)
- Fix layout of endpoint list (#7742)
- Fix layout of endpoint list (#7742) (#7744)
- Remove the placeholder RunState type. (#7749)
- Allow GET parameters that support encoding.TextUnmarshaler (#7800)
- Remove unused latency metric (#7810)
- Implement the eventlog defined by ADR 075 (#7825)
- Implement the ADR 075 /events method (#7965)
- Set a minimum long-polling interval for Events (#8050)

### Rpc/client

- Add Events method to the client interface (#7982)
- Rewrite the WaitForOneEvent helper (#7986)
- Add eventstream helper (#7987)

### Service

- Avoid debug logs before error (#7564)
- Change stop interface (#7816)
- Add NopService and use for PexReactor (#8100)

### Spec

- Merge spec repo into tendermint repo (#7804)
- Merge spec repo into tendermint repo (#7804)
- Minor updates to spec merge PR (#7835)

### State

- Synchronize the ProcessProposal implementation with the latest version of the spec (#7961)
- Avoid panics for marshaling errors (#8125)
- Panic on ResponsePrepareProposal validation error (#8145)
- Propogate error from state store (#8171)

### Statesync

- Reactor and channel construction (#7529)
- Use specific testing.T logger for tests (#7543)
- Clarify test cleanup (#7565)
- SyncAny test buffering (#7570)
- More orderly dispatcher shutdown (#7601)
- Relax timing (#7819)
- Assert app version matches (#7856)
- Assert app version matches (backport #7856) (#7886)
- Avoid compounding retry logic for fetching consensus parameters (#8032)
- Avoid compounding retry logic for fetching consensus parameters (backport #8032) (#8041)
- Avoid leaking a thread during tests (#8085)

### Sync+p2p

- Remove closer (#7805)

### Types

- Rename and extend the EventData interface (#7687)
- Make timely predicate adaptive after 10 rounds (#7739)
- Remove nested evidence field from block (#7765)
- Add string format to 64-bit integer JSON fields (#7787)
- Add default values for the synchrony parameters (#7788)
- Update synchrony params to match checked in proto (#8142)
- Minor cleanup of un or minimally used types (#8154)
- Add TimeoutParams into ConsensusParams structs (#8177)

### Types/events+evidence

- Emit events + metrics on evidence validation (#7802)

## [0.7.0-dev.6] - 2022-01-07

### ADR

- Update the proposer-based timestamp spec per discussion with @cason (#7153)

### Bug Fixes

- Change CI testnet config from ci.toml on dashcore.toml
- Update the title of pipeline task
- Ensure seed at least once connects to another seed (#200)
- Panic on precommits does not have any +2/3 votes
- Improved error handling  in DashCoreSignerClient
- Abci/example, cmd and test packages were fixed after the upstream backport
- Some fixes to be able to compile the add
- Some fixes made by PR feedback
- Use types.DefaultDashVotingPower rather than internal dashDefaultVotingPower
- Don't disconnect already disconnected validators

### Documentation

- Fix broken links and layout (#7154)
- Fix broken links and layout (#7154) (#7163)
- Set up Dependabot on new backport branches. (#7227)
- Update bounty links (#7203)
- Add description about how to keep validators public keys at full node
- Add information how to sue preset for network generation
- Change a type of code block
- Add upgrading info about node service (#7241)
- Add upgrading info about node service (#7241) (#7242)
- Clarify where doc site config settings must land (#7289)
- Add abci timing metrics to the metrics docs (#7311)
- Go tutorial fixed for 0.35.0 version (#7329) (#7330)
- Go tutorial fixed for 0.35.0 version (#7329) (#7330) (#7331)
- Update go ws code snippets (#7486)
- Update go ws code snippets (#7486) (#7487)

### Features

- Add two more CI pipeline tasks to run e2e rotate.toml
- Reset full-node pub-keys
- Manual backport the upstream commit b69ac23fd20bdc00dea00c7c8a69fa66f2e675a9
- Update CHANGELOG_PENDING.md
- Improve logging for better elasticsearch compatibility (#220)
- InitChain can set initial core lock height (#222)
- Add empty block on h-1 and h-2 apphash change (#241)
- Inter-validator set communication (#187)
- Add create_proof_block_range config option (#243)

### PBTS

- New minimal set of changes in consensus algorithm (#369)
- New system model and problem statement (#375)

### RFC006

- Semantic Versioning (#365)

### Refactor

- Minor formatting improvements
- Apply peer review feedback

### Security

- Bump prismjs from 1.23.0 to 1.25.0 in /docs (#7168)
- Bump postcss from 7.0.35 to 7.0.39 in /docs (#7167)
- Bump ws from 6.2.1 to 6.2.2 in /docs (#7165)
- Bump path-parse from 1.0.6 to 1.0.7 in /docs (#7164)
- Bump url-parse from 1.5.1 to 1.5.3 in /docs (#7166)
- Bump actions/checkout from 2.3.5 to 2.4.0 (#7199)
- Bump github.com/golangci/golangci-lint from 1.42.1 to 1.43.0 (#7219)
- Bump google.golang.org/grpc from 1.41.0 to 1.42.0 (#7200)
- Bump github.com/lib/pq from 1.10.3 to 1.10.4 (#7261)
- Bump github.com/tendermint/tm-db from 0.6.4 to 0.6.6 (#7287)
- Bump actions/cache from 2.1.6 to 2.1.7 (#7334)
- Bump watchpack from 2.2.0 to 2.3.0 in /docs (#7335)
- Bump github.com/adlio/schema from 1.1.14 to 1.1.15 (#7407)
- Bump github.com/adlio/schema from 1.2.2 to 1.2.3 (#7432)
- Bump github.com/spf13/viper from 1.9.0 to 1.10.0 (#7434)
- Bump github.com/spf13/cobra from 1.2.1 to 1.3.0 (#7456)
- Bump google.golang.org/grpc from 1.42.0 to 1.43.0 (#7455)
- Bump github.com/spf13/viper from 1.10.0 to 1.10.1 (#7470)
- Bump docker/login-action from 1.10.0 to 1.12.0 (#7494)

### Testing

- Regenerate  remote_client mock
- Get rid of workarounds for issues fixed in 0.6.1
- Clean up databases in tests (#6304)
- Improve cleanup for data and disk use (#6311)
- Close db in randConsensusNetWithPeers, just as it is in randConsensusNet
- Ensure commit stateid in wal is OK
- Add testing.T logger connector (#7447)
- Use scoped logger for all public packages (#7504)
- Pass testing.T to assert and require always, assertion cleanup (#7508)
- Remove background contexts (#7509)
- Remove panics from test fixtures (#7522)

### Abci

- Fix readme link (#7173)

### Acbi

- Fix readme link to protocol buffers (#362)

### Adr

- Lib2p implementation plan (#7282)

### Backport

- Add basic metrics to the indexer package. (#7250) (#7252)

### Buf

- Modify buf.yml, add buf generate (#5653)

### Build

- Fix proto-lint step in Makefile
- Bump github.com/rs/zerolog from 1.25.0 to 1.26.0 (#7192)
- Github workflows: fix dependabot and code coverage (#191)
- Bump github.com/adlio/schema from 1.1.13 to 1.1.14
- Bump github.com/adlio/schema from 1.1.13 to 1.1.14 (#7217)
- Bump github.com/golangci/golangci-lint (#7224)
- Bump github.com/rs/zerolog from 1.25.0 to 1.26.0 (#7222)
- Bump google.golang.org/grpc from 1.41.0 to 1.42.0 (#7218)
- Bump github.com/lib/pq from 1.10.3 to 1.10.4
- Run e2e tests in parallel
- Bump github.com/lib/pq from 1.10.3 to 1.10.4 (#7260)
- Bump technote-space/get-diff-action from 5.0.1 to 5.0.2
- Bump github.com/tendermint/tm-db from 0.6.4 to 0.6.6 (#7285)
- Update the builder image location. (#364)
- Update location of proto builder image (#7296)
- Declare packages variable in correct makefile (#7402)
- Bump github.com/adlio/schema from 1.1.14 to 1.1.15 (#7406)
- Bump github.com/adlio/schema from 1.1.14 to 1.1.15
- Bump github.com/adlio/schema from 1.1.15 to 1.2.2 (#7423)
- Bump github.com/adlio/schema from 1.1.15 to 1.2.2 (#7422)
- Bump github.com/adlio/schema from 1.1.15 to 1.2.3
- Bump github.com/spf13/viper from 1.9.0 to 1.10.0 (#7435)
- Bump github.com/adlio/schema from 1.2.2 to 1.2.3 (#7436)
- Bump watchpack from 2.3.0 to 2.3.1 in /docs (#7430)
- Bump google.golang.org/grpc from 1.42.0 to 1.43.0 (#7458)
- Bump github.com/spf13/cobra from 1.2.1 to 1.3.0 (#7457)
- Bump github.com/rs/zerolog from 1.26.0 to 1.26.1 (#7467)
- Bump github.com/rs/zerolog from 1.26.0 to 1.26.1 (#7469)
- Bump github.com/spf13/viper from 1.10.0 to 1.10.1 (#7468)
- Bump docker/login-action from 1.10.0 to 1.11.0 (#378)
- Bump github.com/rs/cors from 1.8.0 to 1.8.2
- Bump docker/login-action from 1.11.0 to 1.12.0 (#380)
- Bump github.com/rs/cors from 1.8.0 to 1.8.2 (#7484)
- Bump github.com/rs/cors from 1.8.0 to 1.8.2 (#7485)

### Ci

- Update dependabot configuration (#7204)
- Backport lint configuration changes (#7226)
- Move test execution to makefile (#7372)
- Move test execution to makefile (#7372) (#7374)
- Cleanup build/test targets (#7393)
- Fix missing dependency (#7396)
- Cleanup build/test targets (backport #7393) (#7395)
- Skip docker image builds during PRs (#7397)
- Skip docker image builds during PRs (#7397) (#7398)
- Tweak e2e configuration (#7400)

### Cmd

- Add integration test and fix bug in rollback command (#7315)
- Cosmetic changes for errors and print statements (#7377)
- Cosmetic changes for errors and print statements (#7377) (#7408)
- Add integration test for rollback functionality (backport #7315) (#7369)

### Config

- WriteConfigFile should return error (#7169)
- Expose ability to write config to arbitrary paths (#7174)
- Backport file writing changes (#7182)
- Add a Deprecation annotation to P2PConfig.Seeds. (#7496)
- Add a Deprecation annotation to P2PConfig.Seeds. (#7496) (#7497)

### Consensus

- Remove stale WAL benchmark (#7194)
- Add some more checks to vote counting (#7253)
- Add some more checks to vote counting (#7253) (#7262)
- Remove reactor options (#7526)

### Consensus+p2p

- Change how consensus reactor is constructed (#7525)

### Contexts

- Remove all TODO instances (#7466)

### E2e

- Add option to dump and analyze core dumps
- Control access to state in Info calls (#7345)
- More clear height test (#7347)
- Stabilize validator update form (#7340)
- Stabilize validator update form (#7340) (#7351)
- Clarify apphash reporting (#7348)
- Clarify apphash reporting (#7348) (#7352)
- Generate keys for more stable load (#7344)
- Generate keys for more stable load (#7344) (#7353)
- App hash test cleanup (0.35 backport) (#7350)
- Limit legacyp2p and statesyncp2p (#7361)
- Use more simple strings for generated transactions (#7513)
- Avoid global test context (#7512)
- Use more simple strings for generated transactions (#7513) (#7514)
- Constrain test parallelism and reporting (#7516)
- Constrain test parallelism and reporting (backport #7516) (#7517)
- Make tx test more stable (#7523)
- Make tx test more stable (backport #7523) (#7527)

### Errors

- Formating cleanup (#7507)

### Eventbus

- Plumb contexts (#7337)

### Evidence

- Remove source of non-determinism from test (#7266)
- Remove source of non-determinism from test (#7266) (#7268)

### Flowrate

- Cleanup unused files (#7158)

### Fuzz

- Remove fuzz cases for deleted code (#7187)

### Internal/libs/protoio

- Optimize MarshalDelimited by plain byteslice allocations+sync.Pool (#7325)
- Optimize MarshalDelimited by plain byteslice allocations+sync.Pool (#7325) (#7426)

### Internal/proxy

- Add initial set of abci metrics backport (#7342)

### Libs/os

- Remove arbitrary os.Exit (#7284)
- Remove trap signal (#7515)

### Libs/rand

- Remove custom seed function (#7473)

### Libs/service

- Pass logger explicitly (#7288)

### Light

- Remove global context from tests (#7505)

### Lint

- Cleanup branch lint errors (#7238)
- Remove lll check (#7346)
- Remove lll check (#7346) (#7357)

### Log

- Dissallow nil loggers (#7445)

### Mempool

- Port reactor tests from legacy implementation (#7162)
- Consoldate implementations (#7171)
- Avoid arbitrary background contexts (#7409)

### Node

- Cleanup construction (#7191)
- Minor package cleanups (#7444)

### Node+consensus

- Handshaker initialization (#7283)

### P2p

- Transport should be captive resposibility of router (#7160)
- Add message type into the send/recv bytes metrics (backport #7155) (#7161)
- Reduce peer score for dial failures (#7265)
- Reduce peer score for dial failures (backport #7265) (#7271)
- Remove unused trust package (#7359)
- Implement interface for p2p.Channel without channels (#7378)
- Remove unneeded close channels from p2p layer (#7392)
- Migrate to use new interface for channel errors (#7403)
- Refactor channel Send/out (#7414)
- Use recieve for channel iteration (#7425)

### P2p/upnp

- Remove unused functionality (#7379)

### Pex

- Allow disabled pex reactor (#7198)
- Allow disabled pex reactor (backport #7198) (#7201)
- Avoid starting reactor twice (#7239)
- Improve goroutine lifecycle (#7343)

### Privval

- Remove panics in privval implementation (#7475)
- Improve test hygine (#7511)

### Proto

- Update the mechanism for generating protos from spec repo (#7269)
- Abci++ changes (#348)
- Rebuild the proto files from the spec repository (#7291)

### Pubsub

- Use distinct client IDs for test subscriptions. (#7178)
- Use distinct client IDs for test subscriptions. (#7178) (#7179)
- Use a dynamic queue for buffered subscriptions (#7177)
- Remove uninformative publisher benchmarks. (#7195)
- Move indexing out of the primary subscription path (#7231)
- Report a non-nil error when shutting down. (#7310)
- Make the queue unwritable after shutdown. (#7316)

### Rfc

- Deterministic proto bytes serialization (#7427)
- Don't panic (#7472)

### Rpc

- Fix inappropriate http request log (#7244)
- Backport experimental buffer size control parameters from #7230 (tm v0.35.x) (#7276)
- Implement header and header_by_hash queries (#7270)
- Implement header and header_by_hash queries (backport #7270) (#7367)

### Service

- Remove stop method and use contexts (#7292)
- Remove quit method (#7293)
- Cleanup base implementation and some caller implementations (#7301)
- Plumb contexts to all (most) threads (#7363)
- Remove exported logger from base implemenation (#7381)
- Cleanup close channel in reactors (#7399)
- Cleanup mempool and peer update shutdown (#7401)

### State

- Pass connected context (#7410)

### Statesync

- Assert app version matches (#7463)

### Sync

- Remove special mutexes (#7438)

### Tools

- Remove tm-signer-harness (#7370)

### Tools/tm-signer-harness

- Switch to not use hardcoded bytes for configs in test (#7362)

### Types

- Fix path handling in node key tests (#7493)
- Fix path handling in node key tests (#7493) (#7502)
- Remove panic from block methods (#7501)
- Tests should not panic (#7506)

## [0.6.1-dev.1] - 2021-10-26

### Bug Fixes

- Accessing validator state safetly
- Safe state access in TestValidProposalChainLocks
- Safe state access in TestReactorInvalidBlockChainLock
- Safe state access in TestReactorInvalidBlockChainLock
- Seeds should not hang when disconnected from all nodes

### Documentation

- Add roadmap to repo (#7107)
- Add reactor sections (#6510)
- Add reactor sections (backport #6510) (#7151)

### Security

- Bump actions/checkout from 2.3.4 to 2.3.5 (#7139)

### Blocksync

- Remove v0 folder structure (#7128)

### Buf

- Modify buf.yml, add buf generate (#5653)

### Build

- Bump rtCamp/action-slack-notify from 2.1.1 to 2.2.0
- Fix proto-lint step in Makefile

### E2e

- Always enable blocksync (#7144)
- Avoid unset defaults in generated tests (#7145)
- Evidence test refactor (#7146)

### Light

- Fix panic when empty commit is received from server

### Mempool

- Remove panic when recheck-tx was not sent to ABCI application (#7134)
- Remove panic when recheck-tx was not sent to ABCI application (#7134) (#7142)

### Node,blocksync,config

- Remove support for running nodes with blocksync disabled (#7159)

### P2p

- Refactor channel description (#7130)
- Channel shim cleanup (#7129)
- Flatten channel descriptor (#7132)
- Simplify open channel interface (#7133)
- Remove final shims from p2p package (#7136)
- Use correct transport configuration (#7152)
- Add message type into the send/recv bytes metrics (#7155)

### Pex

- Remove legacy proto messages (#7147)

### Pubsub

- Simplify and improve server concurrency handling (#7070)

### State

- Add height assertion to rollback function (#7143)
- Add height assertion to rollback function (#7143) (#7148)

### Tools

- Clone proto files from spec (#6976)

## [0.6.0] - 2021-10-14

### .github

- Remove tessr and bez from codeowners (#7028)

### Bug Fixes

- Amd64 build and arm build ci
- State sync locks when trying to retrieve AppHash
- Set correct LastStateID when updating block
- StateID - update tests (WIP, some still red)
- Ractor should validate StateID correctly + other fixes
- StateID in light client implementation
- Tests sometimes fail on connection attempt
- App hash size validation + remove unused code
- Invalid generation of  tmproto.StateID request id
- State sync locks when trying to retrieve AppHash
- Correctly handle state ID of initial block
- Don't use state to verify blocks from mempool
- Incorrect state id for first block
- AppHashSize is inconsistent
- Support initial height != 1
- E2e: workaround for "chain stalled at unknown height"
- Update dashcore network config, add validator01 to validator_update.0 and add all available validators to 1010 height
- Cleanup e2e Readme.md
- Remove height 1008 from dashcore
- Race condition in p2p_switch and pex_reactor (#7015)
- Race condition in p2p_switch and pex_reactor (#7015)
- Fix MD after the lint
- To avoid potential race conditions the validator-set-update is needed to copy rather than using the pointer to the field at PersistentKVStoreApplication, 'cause it leads to a race condition
- Update a comment block

### Documentation

- Add documentation of unsafe_flush_mempool to openapi (#6947)
- Fix openapi yaml lint (#6948)
- State ID
- State-id.md typos and grammar
- Remove invalid info about initial state id
- Add some code comments
- ADR: Inter Validator Set Messaging
- Adr-d001: apllied feedback, added additional info
- Adr-d001 clarified abci protocol changes
- Adr-d001 describe 3 scenarios and minor restructure
- Adr-d001: clarify terms based on peer  review
- Create separate releases doc (#7040)
- Adr-d001 apply peer review comments
- StateID verification algorithm

### Features

- Add ProposedBlockGTimeWindow in a config
- Fix coping of PubKey pointer
- Use proto.Copy function to copy a message

### Fix

- Benchmark tests slow down light client tests

### Miscellaneous Tasks

- Bump version to 0.6.0 (#185)

### Refactor

- E2e docker: build bls in separate layer
- Golangci-lint + minor test improvements
- Minor formatting updates
- E2e docker: build bls in separate layer
- Add ErrInvalidVoteSignature
- S/GetStateID()/StateID()/
- Code style changes after peer review
- Move stateid to separate file
- Remove unused message CanonicalStateVote
- Use types instead of pb StateID in SignVote and Evidence
- Inverse behaviour of resetting fullnode pubkeys from FULLNODE_PUBKEY_RESET to FULLNODE_PUBKEY_KEEP env
- Add runner/rotate task to simplify running rotate network
- Assignment copies lock value (#7108)

### Testing

- Add StateID unit tests
- Check if wrong state ID fails VoteAdd()
- Fix: TestStateBadProposal didn't copy slices correctly
- TestHandshakePanicsIfAppReturnsWrongAppHash fixed
- Change apphash for every message
- Workaround for e2e tests starting too fast
- Consensus tests use random initial height
- Non-nil genesis apphash in genesis tests
- Add tests for initial height != 1 to consensus
- Fix: replay_test.go fails due to invalid height processing
- Add some StateID AppHash and Height assertions
- StateID verify with blocks N and N+1
- Cleanup rpc/client and node test fixtures (#7112)
- Install abci-cli when running make tests_integrations (#6834)

### Abci

- Flush socket requests and responses immediately. (#6997)
- Change client to use multi-reader mutexes (backport #6306) (#6873)

### Add

- Update e2e doc

### Blocksync

- Fix shutdown deadlock issue (#7030)

### Blocksync/v2

- Remove unsupported reactor (#7046)

### Build

- Bump codecov/codecov-action from 2.0.3 to 2.1.0 (#6938)
- Bump github.com/vektra/mockery/v2 from 2.9.0 to 2.9.3 (#6951)
- Bump github.com/vektra/mockery/v2 from 2.9.3 to 2.9.4 (#6956)
- Bump github.com/spf13/viper from 1.8.1 to 1.9.0 (#6961)
- E2e docker app can be run with dlv debugger
- Improve e2e docker container debugging
- Bump github.com/go-kit/kit from 0.11.0 to 0.12.0 (#6988)
- Bump google.golang.org/grpc from 1.40.0 to 1.41.0 (#7003)
- Update all deps to most recent version
- Bump github.com/adlio/schema from 1.1.13 to 1.1.14 (#7069)
- Replace github.com/go-kit/kit/log with github.com/go-kit/log
- Fix build-docker to include the full context. (#7114)
- Fix build-docker to include the full context. (#7114) (#7116)

### Changelog

- Add entry for interanlizations (#6989)
- Add 0.34.14 updates (#7117)

### Ci

- Disable codecov patch status check (#6930)
- Skip coverage for non-go changes (#6927)
- Skip coverage tasks for test infrastructure (#6934)
- Reduce number of groups for 0.34 e2e runs (#6968)
- Use smart merges (#6993)
- Use cheaper codecov data collection (#7009)
- Mergify support for 0.35 backports (#7050)
- 0.35.x nightly should run from master and checkout the release branch (#7067)
- Fix p2p configuration for e2e tests (#7066)
- Use run-multiple.sh for e2e pr tests (#7111)

### Cleanup

- Reduce and normalize import path aliasing. (#6975)
- Remove not needed binary test/app/grpc_client

### Cli

- Allow node operator to rollback last state (#7033)
- Allow node operator to rollback last state (backport #7033) (#7081)

### Config

- Add example on external_address (backport #6621) (#6624)

### Config/docs

- Update and deprecated (#6879)

### Consensus

- Avoid unbuffered channel in state test (#7025)
- Wait until peerUpdates channel is closed to close remaining peers (#7058)
- Wait until peerUpdates channel is closed to close remaining peers (#7058) (#7060)

### Crypto/armor

- Remove unused package (#6963)

### E2e

- Compile tests (#6926)
- Improve p2p mode selection (#6929)
- Reduce load volume (#6932)
- Slow load processes with longer evidence timeouts (#6936)
- Reduce load pressure (#6939)
- Tweak semantics of waitForHeight (#6943)
- Skip broadcastTxCommit check (#6949)
- Allow load generator to succed for short tests (#6952)
- Cleanup on all errors if preserve not specified (#6950)
- Run multiple should use preserve (#6972)
- Improve manifest sorting algorithim (#6979)
- Only check validator sets after statesync (#6980)
- Always preserve failed networks (#6981)
- Load should be proportional to network (#6983)
- Avoid non-determinism in app hash check (#6985)
- Tighten timing for load generation (#6990)
- Skip validation of status apphash (#6991)
- Do not inject evidence through light proxy (#6992)
- Add limit and sort to generator (#6998)
- Reduce number of statesyncs in test networks (#6999)
- Improve chances of statesyncing success (#7001)
- Allow running of single node using the e2e app (#6982)
- Reduce log noise (#7004)
- Avoid seed nodes when statesyncing (#7006)
- Add generator tests (#7008)
- Reduce number of stateless nodes in test networks (#7010)
- Use smaller transactions (#7016)
- Use network size in load generator (#7019)
- Generator ensure p2p modes (#7021)
- Automatically prune old app snapshots (#7034)
- Automatically prune old app snapshots (#7034) (#7063)
- Improve network connectivity (#7077)
- Abci protocol should be consistent across networks (#7078)
- Abci protocol should be consistent across networks (#7078) (#7086)
- Light nodes should use builtin abci app (#7095)
- Light nodes should use builtin abci app (#7095) (#7097)
- Disable app tests for light client (#6672)
- Avoid starting nodes from the future (#6835) (#6838)
- Cleanup node start function (#6842) (#6848)

### Inspect

- Remove duplicated construction path (#6966)

### Internal/consensus

- Update error log (#6863) (#6867)

### Internal/proxy

- Add initial set of abci metrics (#7115)

### Light

- Update initialization description (#320)
- Update links in package docs. (#7099)
- Update links in package docs. (#7099) (#7101)
- Fix early erroring (#6905)

### Lint

- Fix collection of stale errors (#7090)

### Mempool,rpc

- Add removetx rpc method (#7047)
- Add removetx rpc method (#7047) (#7065)

### Node

- Always close database engine (#7113)

### P2p

- Delete legacy stack initial pass (#7035)
- Remove wdrr queue (#7064)
- Cleanup transport interface (#7071)
- Cleanup unused arguments (#7079)
- Rename pexV2 to pex (#7088)
- Fix priority queue bytes pending calculation (#7120)

### Pex

- Update pex messages (#352)

### Proto

- Add tendermint go changes (#349)
- Regenerate code (#6977)

### Proxy

- Move proxy package to internal (#6953)

### Readme

- Update discord links (#6965)

### Rfc

- E2e improvements (#6941)
- Add performance taxonomy rfc (#6921)
- Fix a few typos and formatting glitches p2p roadmap (#6960)
- Event system (#6957)

### Rpc

- Strip down the base RPC client interface. (#6971)
- Implement BroadcastTxCommit without event subscriptions (#6984)
- Add chunked rpc interface (backport #6445) (#6717)
- Move evidence tests to shared fixtures (#7119)
- Remove the deprecated gRPC interface to the RPC service (#7121)
- Fix typo in broadcast commit (#7124)

### Scripts

- Fix authors script to take a ref (#7051)

### State

- Move package to internal (#6964)

### Statesync

- Shut down node when statesync fails (#6944)
- Clean up reactor/syncer lifecylce (#6995)
- Add logging while waiting for peers (#7007)
- Ensure test network properly configured (#7026)
- Remove deadlock on init fail (#7029)
- Improve rare p2p race condition (#7042)
- Improve stateprovider handling in the syncer (backport) (#6881)

### Statesync/rpc

- Metrics for the statesync and the rpc SyncInfo (#6795)

### Store

- Move pacakge to internal (#6978)

## [0.6.0-dev.2] - 2021-09-10

### Documentation

- Add package godoc for indexer (#6839)
- Remove return code in normal case from go built-in example (#6841)
- Fix a typo in the indexing section (#6909)

### Features

- Info field with arbitrary data to ResultBroadcastTx

### Security

- Bump github.com/rs/zerolog from 1.24.0 to 1.25.0 (#6923)

### Abci

- Clarify what abci stands for (#336)
- Clarify connection use in-process (#337)
- Change client to use multi-reader mutexes (backport #6306) (#6873)

### Blocksync

- Complete transition from Blockchain to BlockSync (#6847)

### Build

- Bump docker/build-push-action from 2.6.1 to 2.7.0 (#6845)
- Bump codecov/codecov-action from 2.0.2 to 2.0.3 (#6860)
- Bump github.com/rs/zerolog from 1.23.0 to 1.24.0 (#6874)
- Bump github.com/lib/pq from 1.10.2 to 1.10.3 (#6890)
- Bump docker/setup-buildx-action from 1.5.0 to 1.6.0 (#6903)
- Bump github.com/golangci/golangci-lint (#6907)

### Ci

- Drop codecov bot (#6917)
- Tweak code coverage settings (#6920)

### Cleanup

- Fix order of linters in the golangci-lint config (#6910)

### Cmd

- Remove deprecated snakes (#6854)

### Contributing

- Remove release_notes.md reference (#6846)

### E2e

- Cleanup node start function (#6842)
- Cleanup node start function (#6842) (#6848)
- More consistent node selection during tests (#6857)
- Add weighted random configuration selector (#6869)
- More reliable method for selecting node to inject evidence (#6880)
- Change restart mechanism (#6883)
- Weight protocol dimensions (#6884)
- Skip light clients when waiting for height (#6891)
- Wait for all nodes rather than just one (#6892)
- Skip assertions for stateless nodes (#6894)
- Clean up generation of evidence (#6904)
- Introduce canonical ordering of manifests (#6918)
- Load generation and logging changes (#6912)
- Increase retain height to at least twice evidence age (#6924)
- Test multiple broadcast tx methods (#6925)

### Inspect

- Add inspect mode for debugging crashed tendermint node (#6785)

### Internal/consensus

- Update error log (#6863)
- Update error log (#6863) (#6867)

### Light

- Fix early erroring (#6905)

### Lint

- Change deprecated linter (#6861)

### Network

- Update terraform config (#6901)

### Networks

- Update to latest DigitalOcean modules (#6902)

### P2p

- Change default to use new stack (#6862)

### Proto

- Move proto files under the correct directory related to their package name (#344)

### Psql

- Add documentation and simplify constructor API (#6856)

### Pubsub

- Improve handling of closed blocking subsciptions. (#6852)

### Rfc

- P2p next steps (#6866)
- Fix link style (#6870)
- Database storage engine (#6897)

### Rpc

- Fix hash encoding in JSON parameters (#6813)

### Statesync

- Improve stateprovider handling in the syncer (backport) (#6881)
- Implement p2p state provider (#6807)

### Time

- Make median time library type private (#6853)

### Types

- Move mempool error for consistency (#6875)

### Upgrading

- Add information into the UPGRADING.md for users of the codebase wishing to upgrade (#6898)

## [0.6.0-dev.1] - 2021-08-19

### Documentation

- Upgrade documentation for custom mempools (#6794)
- Fix typos in /tx_search and /tx. (#6823)

### Features

- [**breaking**] Proposed app version (#148)

### Miscellaneous Tasks

- Bump tenderdash version to 0.6.0-dev.1

### Security

- Bump google.golang.org/grpc from 1.39.0 to 1.39.1 (#6801)
- Bump google.golang.org/grpc from 1.39.1 to 1.40.0 (#6819)

### Testing

- Install abci-cli when running make tests_integrations (#6834)

### Adr

- Node initialization (#6562)

### Build

- Bump github.com/golangci/golangci-lint (#6837)

### Bytes

- Clean up and simplify encoding of HexBytes (#6810)

### Changelog

- Prepare for v0.34.12 (#6831)
- Update to reflect 0.34.12 release (#6833)
- Linkify the 0.34.11 release notes (#6836)

### Changelog_pending

- Add missing item (#6829)
- Add missing entry (#6830)

### Commands

- Add key migration cli (#6790)

### Contributing

- Update release instructions to use backport branches (#6827)

### Core

- Text cleanup (#332)

### E2e

- Avoid starting nodes from the future (#6835)
- Avoid starting nodes from the future (#6835) (#6838)

### Node

- Minimize hardcoded service initialization (#6798)

### Pubsub

- Unsubscribe locking handling (#6816)

### Rpc

- Avoid panics in unsafe rpc calls with new p2p stack (#6817)
- Support new p2p infrastructure (#6820)
- Log update (#6825)
- Log update (backport #6825) (#6826)
- Update peer format in specification in NetInfo operation (#331)

### Statesync

- New messages for gossiping consensus params (#328)

### Version

- Bump for 0.34.12 (#6832)

## [0.5.12-dev.1] - 2021-08-06

### Documentation

- Fix typo (#6789)
- Fix a typo in the genesis_chunked description (#6792)

### Build

- Bump technote-space/get-diff-action from 4 to 5 (#6788)
- Bump github.com/BurntSushi/toml from 0.3.1 to 0.4.1 (#6796)

### Clist

- Add simple property tests (#6791)

### Evidence

- Add section explaining evidence (#324)

### Mempool/v1

- Test reactor does not panic on broadcast (#6772)

## [0.5.11-dev.4] - 2021-07-31

### Blockstore

- Fix problem with seen commit (#6782)

### Build

- Bump styfle/cancel-workflow-action from 0.9.0 to 0.9.1 (#6786)

### State/privval

- Vote timestamp fix (backport #6748) (#6783)

### Tools

- Add mockery to tools.go and remove mockery version strings (#6787)

## [0.5.11-dev.3] - 2021-07-30

### Blockchain

- Rename to blocksync service (#6755)

### Cleanup

- Remove redundant error plumbing (#6778)

### Light

- Replace homegrown mock with mockery (#6735)

### Rpc

- Add documentation for genesis chunked api (#6776)

### State/privval

- Vote timestamp fix (#6748)

## [0.5.11-dev.2] - 2021-07-28

### Abci

- Add changelog entry for mempool_error field (#6770)

### Cli/indexer

- Reindex events (#6676)

### Light

- Wait for tendermint node to start before running example test (#6744)

## [0.5.10-dev.3] - 2021-07-26

### Testing

- Add mechanism to reproduce found fuzz errors (#6768)

## [0.5.10-dev.1] - 2021-07-26

### P2p

- Add test for pqueue dequeue full error (#6760)

## [0.5.10] - 2021-07-26

### Testing

- Add test to reproduce found fuzz errors (#6757)

### Build

- Bump golangci/golangci-lint-action from 2.3.0 to 2.5.2
- Bump codecov/codecov-action from 1.5.2 to 2.0.1 (#6739)
- Bump codecov/codecov-action from 2.0.1 to 2.0.2 (#6764)

### E2e

- Avoid systematic key-type variation (#6736)
- Drop single node hybrid configurations (#6737)
- Remove cartesian testing of ipv6 (#6734)
- Run tests in fewer groups (#6742)
- Prevent adding light clients as persistent peers (#6743)
- Longer test harness timeouts (#6728)
- Allow for both v0 and v1 mempool implementations (#6752)

### Fastsync/event

- Emit fastsync status event when switching consensus/fastsync (#6619)

### Internal

- Update blockchain reactor godoc (#6749)

### Light

- Run examples as integration tests (#6745)
- Improve error handling and allow providers to be added (#6733)

### Mempool

- Return mempool errors to the abci client (#6740)

### P2p

- Add coverage for mConnConnection.TrySendMessage (#6754)
- Avoid blocking on the dequeCh (#6765)

### Statesync/event

- Emit statesync start/end event  (#6700)

## [0.5.8] - 2021-07-20

### Blockchain

- Error on v2 selection (#6730)

### Clist

- Add a few basic clist tests (#6727)

### Libs/clist

- Revert clear and detach changes while debugging (#6731)

### Mempool

- Add TTL configuration to mempool (#6715)

## [0.5.7] - 2021-07-16

### Bug Fixes

- Maverick compile issues (#104)
- Private validator key still automatically creating (#120)
- Getting pro tx hash from full node
- Incorrectly assume amd64 arch during docker build
- Image isn't pushed after build

### Documentation

- Rename tenderdash and update target repo
- Update events (#6658)
- Add sentence about windows support (#6655)
- Add docs file for the peer exchange (#6665)
- Update github issue and pr templates (#131)
- Fix broken links (#6719)

### Features

- Improve initialisation (#117)
- Add arm64 arch for builds

### Miscellaneous Tasks

- Target production dockerhub org
- Use official docker action

### RPC

- Mark grpc as deprecated (#6725)

### Security

- Bump github.com/spf13/viper from 1.8.0 to 1.8.1 (#6622)
- Bump github.com/rs/cors from 1.7.0 to 1.8.0 (#6635)
- Bump github.com/go-kit/kit from 0.10.0 to 0.11.0 (#6651)
- Bump github.com/spf13/cobra from 1.2.0 to 1.2.1 (#6650)

### Testing

- Add current fuzzing to oss-fuzz-build script (#6576)
- Fix wrong compile fuzzer command (#6579)
- Fix wrong path for some p2p fuzzing packages (#6580)
- Fix non-deterministic backfill test (#6648)

### Abci

- Fix gitignore abci-cli (#6668)
- Remove counter app (#6684)

### Build

- Bump github.com/rs/zerolog from 1.22.0 to 1.23.0 (#6575)
- Bump github.com/spf13/viper from 1.7.1 to 1.8.0 (#6586)
- Bump docker/login-action from 1.9.0 to 1.10.0 (#6614)
- Bump docker/setup-buildx-action from 1.3.0 to 1.4.0 (#6629)
- Bump docker/setup-buildx-action from 1.4.0 to 1.4.1 (#6632)
- Bump google.golang.org/grpc from 1.38.0 to 1.39.0 (#6633)
- Bump github.com/spf13/cobra from 1.1.3 to 1.2.0 (#6640)
- Bump docker/build-push-action from 2.5.0 to 2.6.1 (#6639)
- Bump docker/setup-buildx-action from 1.4.1 to 1.5.0 (#6649)
- Bump gaurav-nelson/github-action-markdown-link-check (#6679)
- Bump github.com/golangci/golangci-lint (#6686)
- Bump gaurav-nelson/github-action-markdown-link-check (#313)
- Bump github.com/google/uuid from 1.2.0 to 1.3.0 (#6708)
- Bump actions/stale from 3.0.19 to 4 (#319)
- Bump actions/stale from 3.0.19 to 4 (#6726)

### Changelog

- Have a single friendly bug bounty reminder (#6600)
- Update and regularize changelog entries (#6594)

### Ci

- Make ci consistent and push to docker hub
- Trigger docker build on release
- Disable arm build
- Always push to dockerhub
- Test enabling cache
- Test arm build
- Manually trigger build
- Disable arm64 builds
- Set release to workflow dispatch (manual) trigger
- Enable arm64 in CI

### Cmd/tendermint/commands

- Replace $HOME/.some/test/dir with t.TempDir (#6623)

### Config

- Add root dir to priv validator (#6585)
- Add example on external_address (#6621)
- Add example on external_address (backport #6621) (#6624)

### Consensus

- Skip all messages during sync (#6577)

### Crypto

- Use a different library for ed25519/sr25519 (#6526)

### Deps

- Remove pkg errors (#6666)
- Run go mod tidy (#6677)

### E2e

- Fix looping problem while waiting (#6568)
- Allow variable tx size  (#6659)
- Disable app tests for light client (#6672)
- Remove colorized output from docker-compose (#6670)
- Extend timeouts in test harness (#6694)
- Ensure evidence validator set matches nodes validator set (#6712)
- Tweak sleep for pertubations (#6723)

### Evidence

- Update ADR 59 and add comments to the use of common height (#6628)

### Fastsync

- Update the metrics during fast-sync (#6590)

### Fastsync/rpc

- Add TotalSyncedTime & RemainingTime to SyncInfo in /status RPC (#6620)

### Fuzz

- Initial support for fuzzing (#6558)

### Internal/blockchain/v0

- Prevent all possible race for blockchainCh.Out (#6637)

### Libs/CList

- Automatically detach the prev/next elements in Remove function (#6626)

### Libs/log

- Text logging format changes (#6589)

### Libs/time

- Move types/time into libs (#6595)

### Light

- Correctly handle contexts (backport -> v0.34.x) (#6685)
- Correctly handle contexts (#6687)
- Add case to catch cancelled contexts within the detector (backport #6701) (#6720)

### Linter

- Linter checks non-ASCII identifiers (#6574)

### Mempool

- Move errors to be public (#6613)

### P2p

- Increase queue size to 16MB (#6588)
- Avoid retry delay in error case (#6591)
- Address audit issues with the peer manager (#6603)
- Make NodeID and NetAddress public (#6583)
- Reduce buffering on channels (#6609)
- Do not redial peers with different chain id (#6630)
- Track peer channels to avoid sending across a channel a peer doesn't have (#6601)
- Remove annoying error log (#6688)

### Pkg

- Expose p2p functions (#6627)

### Privval

- Missing privval type check in SetPrivValidator (#6645)

### Psql

- Close opened rows in tests (#6669)

### Pubsub

- Refactor Event Subscription (#6634)

### Release

- Prepare changelog for v0.34.11 (#6597)
- Update changelog and version (#6599)

### Router/statesync

- Add helpful log messages (#6724)

### Rpc

- Fix RPC client doesn't handle url's without ports (#6507)
- Add subscription id to events (#6386)
- Use shorter path names for tests (#6602)
- Add totalGasUSed to block_results response (#308)
- Add max peer block height into /status rpc call (#6610)
- Add `TotalGasUsed` to `block_results` response (#6615)
- Re-index missing events (#6535)
- Add chunked rpc interface (backport #6445) (#6717)

### State

- Move pruneBlocks from consensus/state to state/execution (#6541)

### State/indexer

- Close row after query (#6664)

### State/privval

- No GetPubKey retry beyond the proposal/voting window (#6578)

### State/types

- Refactor makeBlock, makeBlocks and makeTxs (#6567)

### Statesync

- Tune backfill process (#6565)
- Increase chunk priority and robustness (#6582)
- Make fetching chunks more robust (#6587)
- Keep peer despite lightblock query fail (#6692)
- Remove outgoingCalls race condition in dispatcher (#6699)
- Use initial height as a floor to backfilling (#6709)
- Increase dispatcher timeout (#6714)
- Dispatcher test uses internal channel for timing (#6713)

### Tooling

- Use go version 1.16 as minimum version (#6642)

### Tools

- Remove k8s (#6625)
- Move tools.go to subdir (#6689)

### Types

- Move NodeInfo from p2p (#6618)

## [0.4.2] - 2021-06-10

### Blockchain/v0

- Fix data race in blockchain channel (#6518)

### Build

- Bump github.com/btcsuite/btcd (#6560)
- Bump codecov/codecov-action from 1.5.0 to 1.5.2 (#6559)

### Indexer

- Use INSERT ... ON CONFLICT in the psql eventsink insert functions (#6556)

### Node

- Fix genesis on start up (#6563)

## [0.4.1] - 2021-06-09

### .github

- Add markdown link checker (#4513)
- Move checklist from PR description into an auto-comment (#4745)
- Fix whitespace for autocomment (#4747)
- Fix whitespace for auto-comment (#4750)
- Move mergify config
- Move codecov.yml into .github
- Move codecov config into .github
- Fix fuzz-nightly job (#5965)
- Archive crashers and fix set-crashers-count step (#5992)
- Rename crashers output (fuzz-nightly-test) (#5993)
- Clean up PR template (#6050)
- Use job ID (not step ID) inside if condition (#6060)
- Remove erik as reviewer from dependapot (#6076)
- Rename crashers output (fuzz-nightly-test) (#5993)
- Archive crashers and fix set-crashers-count step (#5992)
- Fix fuzz-nightly job (#5965)
- Use job ID (not step ID) inside if condition (#6060)
- Remove erik as reviewer from dependapot (#6076)
- Jepsen workflow - initial version (#6123)
- [jepsen] fix inputs and remove TTY from docker (#6134)
- [jepsen] use working-directory instead of 'cd' (#6135)
- [jepsen] use "bash -c" to execute lein run cmd (#6136)
- [jepsen] cd inside the container, not outside (#6137)
- [jepsen] fix directory name (#6138)
- [jepsen] source .bashrc (#6139)
- [jepsen] add more docs (#6141)
- [jepsen] archive results (#6164)
- Remove myself from CODEOWNERS (#6248)
- Make core team codeowners (#6384)
- Make core team codeowners (#6383)

### .github/codeowners

- Add alexanderbez (#5913)
- Add alexanderbez (#5913)

### .github/issue_template

- Update `/dump_consensus_state` request. (#5060)

### .github/workflows

- Enable manual dispatch for some workflows (#5929)
- Try different e2e nightly test set (#6036)
- Separate e2e workflows for 0.34.x and master (#6041)
- Fix whitespace in e2e config file (#6043)
- Cleanup yaml for e2e nightlies (#6049)
- Try different e2e nightly test set (#6036)
- Separate e2e workflows for 0.34.x and master (#6041)
- Fix whitespace in e2e config file (#6043)
- Cleanup yaml for e2e nightlies (#6049)

### .gitignore

- Sort (#5690)

### .golangci

- Disable new linters (#4024)
- Set locale to US for misspell linter (#6038)

### .goreleaser

- Don't build linux/arm
- Build for windows
- Add windows, remove arm (32 bit) (#5692)
- Remove arm64 build instructions and bump changelog again (#6131)

### .vscode

- Remove directory (#5626)

### ABCI

- Update readme to fix broken link to proto (#5847)
- Fix ReCheckTx for Socket Client (#6124)

### ADR

- Add missing numbers as blank templates (#5154)

### ADR-037

- DeliverBlock (#3420)

### ADR-053

- Update with implementation plan after prototype (#4427)
- Strengthen and simplify the state sync ABCI interface (#4610)

### ADR-057

- RPC (#4857)

### ADR-062

- Update with new P2P core implementation (#6051)

### Backport

- #6494 (#6506)

### Bug Fixes

- Fix spelling of comment (#4566)
- Make p2p evidence_pending test not timing dependent (#6252)
- Avoid race with a deeper copy (#6285)
- Jsonrpc url parsing and dial function (#6264)
- Jsonrpc url parsing and dial function (#6264) (#6288)
- Theoretical leak in clisit.Init (#6302)
- Test fixture peer manager in mempool reactor tests (#6308)
- Benchmark single operation in parallel benchmark not b.N (#6422)

### CHANGELOG

- Update release/v0.32.8 details (#4162)
- Update to reflect 0.33.5 (#4915)
- Add 0.32.12 changelog entry (#4918)
- Update for 0.34.0-rc4 (#5400)
- Update to reflect v0.34.0-rc6 (#5622)
- Add breaking Version name change (#5628)
- Prepare 0.34.1-rc1 (#5832)

### CHANGELOG_PENDING

- Fix the upcoming release number (#5103)
- Update changelog for changes to American spelling (#6100)

### CODEOWNERS

- Specify more precise codeowners (#5333)
- Remove erikgrinaker (#6057)
- Remove erikgrinaker (#6057)

### CONTRIBUTING

- Include instructions for installing protobuf
- Update minor release process (#4909)
- Update to match the release flow used for 0.34.0 (#5697)

### CONTRIBUTING.md

- Update testing section (#5979)

### Core

- Move validation & data structures together (#176)

### Documentation

- Fix broken links (#3482) (#3488)
- Fix broken links (#3482) (#3488)
- Fix block.Header.Time description (#3529)
- Abci#Commit: better explain the possible deadlock (#3536)
- Fix typo in clist readme (#3574)
- Update contributing.md (#3503)
- Fix minor typo (#3681)
- Update RPC docs for /subscribe & /unsubscribe (#3705)
- Update /block_results RPC docs (#3708)
- Missing 'b' in python command (#3728)
- Fix some language issues and deprecated link (#3733)
- (rpc/broadcast_tx_*) write expectations for a client (#3749)
- Update JS section of abci-cli.md (#3747)
- Update to contributing.md (#3760)
- Add readme image (#3763)
- Remove confusing statement from contributing.md (#3764)
- Quick link fixes throughout docs and repo (#3776)
- Replace priv_validator.json with priv_validator_key.json (#3786)
- Fix consensus spec formatting (#3804)
- "Writing a built-in Tendermint Core application in Go" guide (#3608)
- Add guides to docs (#3830)
- Add a footer to guides (#3835)
- "Writing a Tendermint Core application in Kotlin (gRPC)" guide (#3838)
- "Writing a Tendermint Core application in Java (gRPC)" guide (#3887)
- Fix some typos and changelog entries (#3915)
- Switch the data in `/unconfirmed_txs` and `num_unconfirmed_txs` (#3933)
- Add dev sessions from YouTube (#3929)
- Move dev sessions into docs (#3934)
- Specify a fix for badger err on Windows (#3974)
- Remove traces of develop branch (#4022)
- Any path can be absolute or relative (#4035)
- Add previous dev sessions (#4040)
- Add ABCI Overview (2/2) dev session (#4044)
- Update fork-accountability.md (#4068)
- Add assumption to getting started with abci-cli (#4098)
- Fix build instructions (#4123)
- Add GA for docs.tendermint.com (#4149)
- Replace dead original whitepaper link (#4155)
- Update wording (#4174)
- Mention that Evidence votes are now sorted
- Fix broken links (#4186)
- Fix broken links in consensus/readme.md (#4200)
- Update ADR 43 with links to PRs (#4207)
- Add flag documentation (#4219)
- Fix broken rpc link (#4221)
- Fix broken ecosystem link (#4222)
- Add notes on architecture intro (#4175)
- Remove "0 means latest" from swagger docs (#4236)
- Update app-architecture.md (#4259)
- Link fixes in readme (#4268)
- Add link for installing Tendermint (#4307)
- Update theme version (#4315)
- Minor doc fixes (#4335)
- Update links to rpc (#4348)
- Update npm dependencies (#4364)
- Update guides proto paths (#4365)
- Update specs to remove cmn (#77)
- Fix incorrect link (#4377)
- Fix spec links (#4384)
- Update Light Client Protocol page (#4405)
- Adr-046 add bisection algorithm details (#4496)
- `tendermint node --help` dumps all supported flags (#4511)
- Write about debug kill and dump (#4516)
- Fix links (#4531)
- Validator setup & Key info (#4604)
- Add adr-55 for proto repo design (#4623)
- Amend adr-54 with changes in the sdk (#4684)
- Create adr 56: prove amnesia attack
- Mention unbonding period in MaxAgeNumBlocks/MaxAgeDuration
- State we don't support non constant time crypto
- Move tcp-window.png to imgs/
- Document open file limit in production guide (#4945)
- Update amnesia adr (#4994)
- Update .vuepress/config.js (#5043)
- Add warning for chainid (#5072)
- Added further documentation to the subscribing to events page (#5110)
- Tweak light client documentation (#5121)
- Modify needed proto files for guides (#5123)
- EventAttribute#Index is not deterministic (#5132)
- Event hashing ADR 058 (#5134)
- Simplify choosing an ADR number (#5156)
- Add more details on Vote struct from /consensus_state (#5164)
- Document ConsensusParams (#5165)
- Document canonical field (#5166)
- Cleanup (#5252)
- Dont display duplicate  (#5271)
- Rename swagger to openapi (#5263)
- Fix go tutorials (#5267)
- Versioned (#5241)
- Remove duplicate secure p2p (#5279)
- Remove interview transcript (#5282)
- Add block retention to upgrading.md (#5284)
- Updates to various sections (#5285)
- Add algolia docsearch configs (#5309)
- Add sections to abci (#150)
- Add doc on state sync configuration (#5304)
- Move subscription to tendermint-core (#5323)
- Add missing metrics (#5325)
- Add more description to initial_height (#5350)
- Make rfc section disppear (#5353)
- Document max entries for `/blockchain` RPC (#5356)
- Fix incorrect time_iota_ms configuration (#5385)
- Minor tweaks (#5404)
- Update state sync config with discovery_time (#5405)
- Add explanation of p2p configuration options (#5397)
- Specify TM version in go tutorials (#5427)
- Revise ADR 56, documenting short term decision around amnesia evidence  (#5440)
- Fix links to adr 56 (#5464)
- Docs-staging → master (#5468)
- Make /master the default (#5474)
- Update url for kms repo (#5510)
- Footer cleanup (#5457)
- Remove DEV_SESSIONS list (#5579)
- Add ADR on P2P refactor scope (#5592)
- Bump vuepress-theme-cosmos (#5614)
- Make blockchain not viewable (#211)
- Add missing ADRs to README, update status of ADR 034 (#5663)
- Add P2P architecture ADR (#5637)
- Warn developers about calling blocking funcs in Receive (#5679)
- Add nodes section  (#5604)
- Add version dropdown and v0.34 docs(#5762)
- Fix link (#5763)
- Use hyphens instead of snake case (#5802)
- Specify master for tutorials (#5822)
- Specify 0.34 (#5823)
- Fix broken redirect links (#5881)
- Update package-lock.json (#5928)
- Package-lock.json fix (#5948)
- Change v0.33 version (#5950)
- Bump package-lock.json of v0.34.x (#5952)
- Dont login when in PR (#5961)
- Release Linux/ARM64 image (#5925)
- Log level docs (#5945)
- Fix typo in state sync example (#5989)
- External address (#6035)
- Reword configuration (#6039)
- Change v0.33 version (#5950)
- Release Linux/ARM64 image (#5925)
- Dont login when in PR (#5961)
- Fix typo in state sync example (#5989)
- Fix proto file names (#6112)
- How to add tm version to RPC (#6151)
- Add preallocated list of security vulnerability names (#6167)
- Fix sample code (#6186)
- Fix sample code #6186
- Bump vuepress-theme-cosmos (#6344)
- Remove RFC section and s/RFC001/ADR066 (#6345)
- Adr-65 adjustments (#6401)
- Adr cleanup (#6489)
- Hide security page (second attempt) (#6511)
- Rename tendermint-core to system (#6515)
- Logger updates (#6545)

### Makefile

- Parse TENDERMINT_BUILD_OPTIONS (#4738)
- Use git 2.20-compatible branch detection (#5778)
- Always pull image in proto-gen-docker. (#5953)
- Always pull image in proto-gen-docker. (#5953)

### P2P

- Evidence Reactor Test Refactor (#6238)

### README

- Specify supported versions (#4660)
- Update chat link with Discord instead of Riot (#5071)
- Clean up README (#5391)
- Update link to Tendermint blog (#5713)

### RFC

- Adopt zip 215 (#144)
- ReverseSync - fetching historical data (#224)

### RFC-001

- Configurable block retention (#84)

### RFC-002

- Non-zero genesis (#119)

### RPC

- Don't cap page size in unsafe mode (#6329)

### Security

- Refactor Remote signers (#3370)
- Cross-check new header with all witnesses (#4373)
- [Security] Bump websocket-extensions from 0.1.3 to 0.1.4 in /docs (#4976)
- [Security] Bump lodash from 4.17.15 to 4.17.19 in /docs
- [Security] Bump prismjs from 1.20.0 to 1.21.0 in /docs
- Bump vuepress-theme-cosmos from 1.0.169 to 1.0.172 in /docs (#5286)
- Bump google.golang.org/grpc from 1.31.0 to 1.31.1 (#5290)
- Bump github.com/golang/protobuf from 1.4.2 to 1.4.3 (#5506)
- Bump github.com/spf13/cobra from 1.0.0 to 1.1.0 (#5505)
- Bump github.com/prometheus/client_golang from 1.7.1 to 1.8.0 (#5515)
- Bump github.com/spf13/cobra from 1.1.0 to 1.1.1 (#5526)
- Bump google.golang.org/grpc from 1.33.1 to 1.33.2 (#5635)
- Bump github.com/golang/protobuf from 1.4.2 to 1.4.3 (#5506)
- Bump github.com/spf13/cobra from 1.0.0 to 1.1.0 (#5505)
- Bump github.com/prometheus/client_golang from 1.7.1 to 1.8.0 (#5515)
- Bump github.com/spf13/cobra from 1.1.0 to 1.1.1 (#5526)
- Bump google.golang.org/grpc from 1.33.1 to 1.33.2 (#5635)
- Bump vuepress-theme-cosmos from 1.0.176 to 1.0.177 in /docs (#5746)
- Bump vuepress-theme-cosmos from 1.0.177 to 1.0.178 in /docs (#5754)
- Bump github.com/prometheus/client_golang from 1.8.0 to 1.9.0 (#5807)
- Bump github.com/cosmos/iavl from 0.15.2 to 0.15.3 (#5814)
- Bump github.com/stretchr/testify from 1.6.1 to 1.7.0 (#5897)
- Bump google.golang.org/grpc from 1.34.0 to 1.35.0 (#5902)
- Bump vuepress-theme-cosmos from 1.0.179 to 1.0.180 in /docs (#5915)
- Bump github.com/stretchr/testify from 1.6.1 to 1.7.0 (#5897)
- Bump google.golang.org/grpc from 1.34.0 to 1.35.0 (#5902)
- Bump vuepress-theme-cosmos from 1.0.179 to 1.0.180 in /docs (#5915)
- Bump watchpack from 2.1.0 to 2.1.1 in /docs (#6063)
- Bump github.com/tendermint/tm-db from 0.6.3 to 0.6.4 (#6073)
- Bump github.com/tendermint/tm-db from 0.6.3 to 0.6.4 (#6073)
- Bump watchpack from 2.1.0 to 2.1.1 in /docs (#6063)
- Update 0.34.3 changelog with details on security vuln (bp #6108) (#6110)
- Bump vuepress-theme-cosmos from 1.0.180 to 1.0.181 in /docs (#6266)
- Bump github.com/minio/highwayhash from 1.0.1 to 1.0.2 (#6280)
- Bump google.golang.org/grpc from 1.36.1 to 1.37.0 (#6330)
- Bump github.com/confio/ics23/go from 0.6.3 to 0.6.6 (#6374)
- Bump github.com/grpc-ecosystem/go-grpc-middleware from 1.2.2 to 1.3.0 (#6387)
- Bump google.golang.org/grpc from 1.37.0 to 1.37.1 (#6461)
- Bump google.golang.org/grpc from 1.37.1 to 1.38.0 (#6483)
- Bump github.com/lib/pq from 1.10.1 to 1.10.2 (#6505)

### Testing

- Add consensus_params to testnet config generation (#3781)
- Branch for fix of ci (#4266)
- Bind test servers to 127.0.0.1 (#4322)
- Simplified txsearch cancellation test (#4500)
- Fix p2p test build breakage caused by Debian testing
- Revert Go 1.13→1.14 bump
- Use random socket names to avoid collisions (#4885)
- Mitigate test data race (#4886)
- Use github.sha in binary cache key (#5062)
- Deflake TestAddAndRemoveListenerConcurrency and TestSyncer_SyncAny (#5101)
- Protobuf vectors for reactors (#5221)
- Add end-to-end testing framework (#5435)
- Add basic end-to-end test cases (#5450)
- Add GitHub action for end-to-end tests (#5452)
- Remove P2P tests (#5453)
- Add E2E test for node peering (#5465)
- Add random testnet generator (#5479)
- Clean up E2E test volumes using a container (#5509)
- Tweak E2E tests for nightly runs (#5512)
- Enable ABCI gRPC client in E2E testnets (#5521)
- Enable blockchain v2 in E2E testnet generator (#5533)
- Enable restart/kill perturbations in E2E tests (#5537)
- Add end-to-end testing framework (#5435)
- Add basic end-to-end test cases (#5450)
- Add GitHub action for end-to-end tests (#5452)
- Remove P2P tests (#5453)
- Add E2E test for node peering (#5465)
- Add random testnet generator (#5479)
- Clean up E2E test volumes using a container (#5509)
- Tweak E2E tests for nightly runs (#5512)
- Enable ABCI gRPC client in E2E testnets (#5521)
- Enable blockchain v2 in E2E testnet generator (#5533)
- Enable restart/kill perturbations in E2E tests (#5537)
- Run remaining E2E testnets on run-multiple.sh failure (#5557)
- Tag E2E Docker resources and autoremove them (#5558)
- Add evidence e2e tests (#5488)
- Run remaining E2E testnets on run-multiple.sh failure (#5557)
- Tag E2E Docker resources and autoremove them (#5558)
- Add evidence e2e tests (#5488)
- Fix handling of start height in generated E2E testnets (#5563)
- Disable E2E misbehaviors due to bugs (#5569)
- Fix handling of start height in generated E2E testnets (#5563)
- Disable E2E misbehaviors due to bugs (#5569)
- Fix various E2E test issues (#5576)
- Fix various E2E test issues (#5576)
- Fix secp failures (#5649)
- Fix secp failures (#5649)
- Switched node keys back to edwards (#4)
- Enable v1 and v2 blockchains (#5702)
- Fix TestByzantinePrevoteEquivocation flake (#5710)
- Fix TestByzantinePrevoteEquivocation flake (#5710)
- Fix integration tests and rename binary
- Disable abci/grpc and blockchain/v2 due to flake (#5854)
- Add conceptual overview (#5857)
- Improve WaitGroup handling in Byzantine tests (#5861)
- Improve WaitGroup handling in Byzantine tests (#5861)
- Tolerate up to 2/3 missed signatures for a validator (#5878)
- Disable abci/grpc and blockchain/v2 due to flake (#5854)
- Fix TestPEXReactorRunning data race (#5955)
- Move fuzz tests into this repo (#5918)
- Fix `make test` (#5966)
- Close transports to avoid goroutine leak failures (#5982)
- Don't use foo-bar.net in TestHTTPClientMakeHTTPDialer (#5997)
- Fix TestSwitchAcceptRoutine flake by ignoring error type (#6000)
- Disable TestPEXReactorSeedModeFlushStop due to flake (#5996)
- Fix test data race in p2p.MemoryTransport with logger (#5995)
- Fix TestSwitchAcceptRoutine by ignoring spurious error (#6001)
- Fix TestRouter to take into account PeerManager reconnects (#6002)
- Fix flaky router broadcast test (#6006)
- Enable pprof server to help debugging failures (#6003)
- Increase sign/propose tolerances (#6033)
- Increase validator tolerances (#6037)
- Don't use foo-bar.net in TestHTTPClientMakeHTTPDialer (#5997) (#6047)
- Enable pprof server to help debugging failures (#6003)
- Increase sign/propose tolerances (#6033)
- Increase validator tolerances (#6037)
- Move fuzz tests into this repo (#5918)
- Fix `make test` (#5966)
- Fix TestByzantinePrevoteEquivocation (#6132)
- Fix PEX reactor test (#6188)
- Fix rpc, secret_connection and pex tests (#6190)
- Refactor mempool reactor to use new p2ptest infrastructure (#6250)
- Clean up databases in tests (#6304)
- Improve cleanup for data and disk use (#6311)
- Produce structured reporting from benchmarks (#6343)
- Create common functions for easily producing tm data structures (#6435)
- HeaderHash test vector (#6531)
- Add evidence hash testvectors (#6536)

### UPGRADING

- Polish upgrading instructions for 0.34 (#5398)

### UPGRADING.md

- Write about the LastResultsHash change (#5000)

### UX

- Version configuration (#5740)

### Vagrantfile

- Update Go version

### WIP

- Add implementation of mock/fake http-server
- Rename package name from fakeserver to mockcoreserver
- Change the method names of call structure, Fix adding headers
- Add mock of JRPCServer implementation on top of HTTServer mock

### [Docs]

- Minor doc touchups (#4171)

### Abci

- Refactor tagging events using list of lists (#3643)
- Refactor ABCI CheckTx and DeliverTx signatures (#3735)
- Refactor CheckTx to notify of recheck (#3744)
- Minor cleanups in the socket client (#3758)
- Fix documentation regarding CheckTx type update (#3789)
- Remove TotalTxs and NumTxs from Header (#3783)
- Fix broken spec link (#4366)
- Add basic description of ABCI Commit.ResponseHeight (#85)
- Add MaxAgeNumBlocks/MaxAgeDuration to EvidenceParams (#87)
- Update MaxAgeNumBlocks & MaxAgeDuration docs (#88)
- Fix protobuf lint issues
- Regenerate proto files
- Remove protoreplace script
- Remove python examples
- Proto files follow same path  (#5039)
- Add AppVersion to ConsensusParams (#106)
- Tweak node sync estimate (#115)
- Fix abci evidence types (#5174)
- Add ResponseInitChain.app_hash, check and record it (#5227)
- Add ResponseInitChain.app_hash (#140)
- Update evidence (#5324)
- Fix socket client error for state sync responses (#5395)
- Remove setOption (#5447)
- Lastcommitinfo.round extra sentence (#221)
- Add abci_version to requestInfo (#223)
- Modify Client interface and socket client (#5673)
- Use protoio for length delimitation (#5818)
- Rewrite to proto interface (#237)
- Fix ReCheckTx for Socket Client (bp #6124) (#6125)
- Note on concurrency (#258)
- Change client to use multi-reader mutexes (#6306)
- Reorder sidebar (#282)

### Abci/client

- Fix DATA RACE in gRPC client (#3798)

### Abci/example/kvstore

- Decrease val power by 1 upon equivocation (#5056)

### Abci/examples

- Switch from hex to base64 pubkey in kvstore (#3641)

### Abci/grpc

- Return async responses in order (#5520)
- Return async responses in order (#5520) (#5531)
- Fix ordering of sync/async callback combinations (#5556)
- Fix ordering of sync/async callback combinations (#5556)
- Fix invalid mutex handling in StopForError() (#5849)
- Fix invalid mutex handling in StopForError() (#5849)

### Abci/kvstore

- Return `LastBlockHeight` and `LastBlockAppHash` in `Info` (#4233)

### Abci/server

- Recover from app panics in socket server (#3809)
- Print panic & stack trace to STDERR if logger is not set

### Abci/types

- Update comment (#3612)
- Add comment for TotalVotingPower (#5081)

### Adr

- Peer Behaviour (#3539)
- PeerBehaviour updates (#3558)
- [43] blockchain riri-org (#3753)
- ADR-052: Tendermint Mode (#4302)
- ADR-051: Double Signing Risk Reduction (#4262)
- Light client implementation (#4397)
- Crypto encoding for proto (#4481)
- Add API stability ADR (#5341)
- Privval gRPC (#5712)
- Batch verification (#6008)
- ADR 065: Custom Event Indexing (#6307)

### Adr#50

- Improve trusted peering (#4072)

### Adr-047

- Evidence handling (#4429)

### Adr-053

- Update after state sync merge (#4768)

### All

- Name reactors when they are initialized (#4608)

### Autofile

- Resolve relative paths (#4390)

### Backports

- Mergify (#6107)

### Behaviour

- Return correct reason in MessageOutOfOrder (#3772)
- Add simple doc.go (#5055)

### Block

- Use commit sig size instead of vote size (#5490)
- Fix max commit sig size (#5567)
- Fix max commit sig size (#5567)

### Blockchain

- Update the maxHeight when a peer is removed (#3350)
- Comment out logger in test code that causes a race condition (#3500)
- Dismiss request channel delay (#3459)
- Reorg reactor (#3561)
- Add v2 reactor (#4361)
- Enable v2 to be set (#4597)
- Change validator set sorting method (#91)
- Proto migration  (#4969)
- Test vectors for proto encoding (#5073)
- Rename to core (#123)
- Remove duplicate evidence sections (#124)
- Fix fast sync halt with initial height > 1 (#5249)
- Verify +2/3 (#5278)
- Remove duplication of validate basic (#5418)

### Blockchain/v0

- Relax termination conditions and increase sync timeout (#5741)
- Stop tickers on poolRoutine exit (#5860)
- Stop tickers on poolRoutine exit (#5860)

### Blockchain/v1

- Add noBlockResponse handling  (#5401)
- Add noBlockResponse handling  (#5401)
- Handle peers without blocks (#5701)
- Fix deadlock (#5711)
- Remove in favor of v2 (#5728)
- Omit incoming message bytes from log

### Blockchain/v2

- Allow setting nil switch, for CustomReactors()
- Don't broadcast base if height is 0
- Fix excessive CPU usage due to spinning on closed channels (#4761)
- Respect fast_sync option (#4772)
- Integrate with state sync
- Correctly set block store base in status responses (#4971)
- Fix "panic: duplicate block enqueued by processor" (#5499)
- Fix panic: processed height X+1 but expected height X (#5530)
- Fix "panic: duplicate block enqueued by processor" (#5499)
- Fix panic: processed height X+1 but expected height X (#5530)
- Make the removal of an already removed peer a noop (#5553)
- Make the removal of an already removed peer a noop (#5553)
- Remove peers from the processor  (#5607)
- Remove peers from the processor  (#5607)
- Send status request when new peer joins (#5774)
- Fix missing mutex unlock (#5862)
- Fix missing mutex unlock (#5862)
- Internalize behavior package (#6094)

### Blockchain[v1]

- Increased timeout times for peer tests (#4871)

### Blockstore

- Allow initial SaveBlock() at any height
- Fix race conditions when loading data (#5382)
- Save only the last seen commit (#6212)

### Buf

- Modify buf.yml, add buf generate (#5653)

### Build

- Bump github.com/tendermint/tm-db from 0.1.1 to 0.2.0 (#4001)
- Bump github.com/gogo/protobuf from 1.3.0 to 1.3.1 (#4055)
- Bump github.com/spf13/viper from 1.4.0 to 1.5.0 (#4102)
- Bump github.com/spf13/viper from 1.5.0 to 1.6.1 (#4224)
- Bump github.com/pkg/errors from 0.9.0 to 0.9.1 (#4310)
- Bump google.golang.org/grpc from 1.26.0 to 1.27.0 (#4355)
- Bump github.com/stretchr/testify from 1.5.0 to 1.5.1 (#4441)
- Bump github.com/spf13/cobra from 0.0.3 to 0.0.6 (#4440)
- Bump github.com/golang/protobuf from 1.3.3 to 1.3.4 (#4485)
- Bump github.com/prometheus/client_golang (#4525)
- Bump github.com/Workiva/go-datastructures (#4545)
- Bump google.golang.org/grpc from 1.27.1 to 1.28.0 (#4551)
- Bump github.com/tendermint/tm-db from 0.4.1 to 0.5.0 (#4554)
- Bump github.com/golang/protobuf from 1.3.4 to 1.3.5 (#4563)
- Bump github.com/prometheus/client_golang (#4574)
- Bump github.com/gorilla/websocket from 1.4.1 to 1.4.2 (#4584)
- Bump github.com/spf13/cobra from 0.0.6 to 0.0.7 (#4612)
- Bump github.com/tendermint/tm-db from 0.5.0 to 0.5.1 (#4613)
- Bump google.golang.org/grpc from 1.28.0 to 1.28.1 (#4653)
- Bump github.com/spf13/viper from 1.6.2 to 1.6.3 (#4664)
- Bump @vuepress/plugin-google-analytics in /docs (#4692)
- Bump google.golang.org/grpc from 1.28.1 to 1.29.0
- Bump google.golang.org/grpc from 1.29.0 to 1.29.1 (#4735)
- Manually bump github.com/prometheus/client_golang from 1.5.1 to 1.6.0 (#4758)
- Bump github.com/golang/protobuf from 1.4.0 to 1.4.1 (#4794)
- Bump vuepress-theme-cosmos from 1.0.163 to 1.0.164 in /docs (#4815)
- Bump github.com/spf13/viper from 1.6.3 to 1.7.0 (#4814)
- Bump github.com/golang/protobuf from 1.4.1 to 1.4.2 (#4849)
- Bump vuepress-theme-cosmos from 1.0.164 to 1.0.165 in /docs
- Bump github.com/stretchr/testify from 1.5.1 to 1.6.0
- Bump vuepress-theme-cosmos from 1.0.165 to 1.0.166 in /docs (#4920)
- Bump github.com/stretchr/testify from 1.6.0 to 1.6.1
- Bump github.com/prometheus/client_golang from 1.6.0 to 1.7.0 (#5027)
- Bump google.golang.org/grpc from 1.29.1 to 1.30.0
- Bump github.com/prometheus/client_golang
- Bump vuepress-theme-cosmos from 1.0.168 to 1.0.169 in /docs
- Bump google.golang.org/grpc from 1.30.0 to 1.31.0
- Bump github.com/spf13/viper from 1.7.0 to 1.7.1
- Bump golangci/golangci-lint-action from v2.1.0 to v2.2.0 (#5245)
- Bump actions/cache from v1 to v2.1.0 (#5244)
- Bump codecov/codecov-action from v1.0.7 to v1.0.12 (#5247)
- Bump technote-space/get-diff-action from v1 to v3 (#5246)
- Bump gaurav-nelson/github-action-markdown-link-check from 0.6.0 to 1.0.5 (#5248)
- Bump codecov/codecov-action from v1.0.12 to v1.0.13 (#5258)
- Bump gaurav-nelson/github-action-markdown-link-check from 1.0.5 to 1.0.6 (#5265)
- Bump gaurav-nelson/github-action-markdown-link-check from 1.0.6 to 1.0.7 (#5269)
- Bump actions/cache from v2.1.0 to v2.1.1 (#5268)
- Bump gaurav-nelson/github-action-markdown-link-check from 0.6.0 to 1.0.7 (#149)
- Bump github.com/tendermint/tm-db from 0.6.1 to 0.6.2 (#5296)
- Bump google.golang.org/grpc from 1.31.1 to 1.32.0 (#5346)
- Bump github.com/minio/highwayhash from 1.0.0 to 1.0.1 (#5370)
- Bump vuepress-theme-cosmos from 1.0.172 to 1.0.173 in /docs (#5390)
- Bump watchpack from 1.7.4 to 2.0.0 in /docs (#5470)
- Bump actions/cache from v2.1.1 to v2.1.2 (#5487)
- Bump golangci/golangci-lint-action from v2.2.0 to v2.2.1 (#5486)
- Bump technote-space/get-diff-action from v3 to v4 (#5485)
- Bump codecov/codecov-action from v1.0.13 to v1.0.14 (#5525)
- Bump gaurav-nelson/github-action-markdown-link-check from 1.0.7 to 1.0.8 (#5543)
- Bump gaurav-nelson/github-action-markdown-link-check from 1.0.7 to 1.0.8 (#188)
- Bump google.golang.org/grpc from 1.32.0 to 1.33.1 (#5544)
- Bump actions/cache from v2.1.1 to v2.1.2 (#5487)
- Bump golangci/golangci-lint-action from v2.2.0 to v2.2.1 (#5486)
- Bump technote-space/get-diff-action from v3 to v4 (#5485)
- Bump golangci/golangci-lint-action from v2.2.1 to v2.3.0 (#5571)
- Bump codecov/codecov-action from v1.0.13 to v1.0.14 (#5582)
- Bump watchpack from 2.0.0 to 2.0.1 in /docs (#5605)
- Bump actions/cache from v2.1.2 to v2.1.3 (#5633)
- Bump github.com/tendermint/tm-db from 0.6.2 to 0.6.3
- Bump rtCamp/action-slack-notify from e9db0ef to 2.1.1
- Bump google.golang.org/grpc from 1.32.0 to 1.33.1 (#5544)
- Bump github.com/tendermint/tm-db from 0.6.2 to 0.6.3
- Bump codecov/codecov-action from v1.0.14 to v1.0.15 (#5676)
- Bump github.com/cosmos/iavl from 0.15.0-rc5 to 0.15.0 (#5708)
- Bump vuepress-theme-cosmos from 1.0.175 to 1.0.176 in /docs (#5727)
- Refactor BLS library/bindings integration  (#9)
- BLS scripts - Improve build.sh, Fix install.sh (#10)
- Dashify some files (#11)
- Fix docker image and docker.yml workflow (#12)
- Fix coverage.yml, bump go version, install BLS, drop an invalid character (#19)
- Fix test.yml, bump go version, install BLS, fix job names (#18)
- Bump google.golang.org/grpc from 1.33.2 to 1.34.0 (#5737)
- Bump gaurav-nelson/github-action-markdown-link-check (#22)
- Bump codecov/codecov-action from v1.0.13 to v1.0.15 (#23)
- Bump golangci/golangci-lint-action from v2.2.1 to v2.3.0 (#24)
- Bump rtCamp/action-slack-notify from e9db0ef to 2.1.1 (#25)
- Bump google.golang.org/grpc from 1.33.2 to 1.34.0 (#26)
- Bump vuepress-theme-cosmos from 1.0.173 to 1.0.177 in /docs (#27)
- Bump watchpack from 2.0.1 to 2.1.0 in /docs (#5768)
- Bump rtCamp/action-slack-notify from ecc1353ce30ef086ce3fc3d1ea9ac2e32e150402 to 2.1.2 (#5767)
- Bump vuepress-theme-cosmos from 1.0.178 to 1.0.179 in /docs (#5780)
- Bump gaurav-nelson/github-action-markdown-link-check from 1.0.8 to 1.0.9 (#5779)
- Bump gaurav-nelson/github-action-markdown-link-check (#5787)
- Bump gaurav-nelson/github-action-markdown-link-check (#5793)
- Bump gaurav-nelson/github-action-markdown-link-check (#233)
- Bump github.com/cosmos/iavl from 0.15.0 to 0.15.2
- Bump codecov/codecov-action from v1.0.15 to v1.1.1 (#5825)
- Bump codecov/codecov-action from v1.1.1 to v1.2.0 (#5863)
- Bump codecov/codecov-action from v1.2.0 to v1.2.1
- Bump gaurav-nelson/github-action-markdown-link-check (#239)
- Bump gaurav-nelson/github-action-markdown-link-check (#5884)
- Bump actions/cache from v2.1.3 to v2.1.4 (#6055)
- Bump JamesIves/github-pages-deploy-action (#6062)
- Bump actions/cache from v2.1.3 to v2.1.4 (#6055)
- Bump github.com/spf13/cobra from 1.1.1 to 1.1.2 (#6075)
- Bump github.com/spf13/cobra from 1.1.2 to 1.1.3 (#6098)
- Bump golangci/golangci-lint-action from v2.3.0 to v2.4.0 (#6111)
- Bump golangci/golangci-lint-action from v2.4.0 to v2.5.1 (#6175)
- Bump google.golang.org/grpc from 1.35.0 to 1.36.0 (#6180)
- Bump JamesIves/github-pages-deploy-action from 4.0.0 to 4.1.0 (#6215)
- Bump rtCamp/action-slack-notify from ae4223259071871559b6e9d08b24a63d71b3f0c0 to 2.1.3 (#6234)
- Bump codecov/codecov-action from v1.2.1 to v1.2.2 (#6231)
- Bump codecov/codecov-action from v1.2.2 to v1.3.1 (#6247)
- Bump github.com/golang/protobuf from 1.4.3 to 1.5.1 (#6254)
- Bump github.com/prometheus/client_golang (#6258)
- Bump google.golang.org/grpc from 1.36.0 to 1.36.1 (#6281)
- Bump github.com/golang/protobuf from 1.5.1 to 1.5.2 (#6299)
- Bump github.com/Workiva/go-datastructures (#6298)
- Bump golangci/golangci-lint-action from v2.5.1 to v2.5.2 (#6317)
- Bump codecov/codecov-action from v1.3.1 to v1.3.2 (#6319)
- Bump JamesIves/github-pages-deploy-action (#6316)
- Bump docker/setup-buildx-action from v1 to v1.1.2 (#6324)
- Bump google.golang.org/grpc from 1.36.1 to 1.37.0 (bp #6330) (#6335)
- Bump styfle/cancel-workflow-action from 0.8.0 to 0.9.0 (#6341)
- Bump actions/cache from v2.1.4 to v2.1.5 (#6350)
- Bump codecov/codecov-action from v1.3.2 to v1.4.0 (#6365)
- Bump codecov/codecov-action from v1.4.0 to v1.4.1 (#6379)
- Bump docker/setup-buildx-action from v1.1.2 to v1.2.0 (#6391)
- Bump docker/setup-buildx-action from v1.2.0 to v1.3.0 (#6413)
- Bump codecov/codecov-action from v1.4.1 to v1.5.0 (#6417)
- Bump github.com/cosmos/iavl from 0.15.3 to 0.16.0 (#6421)
- Bump JamesIves/github-pages-deploy-action (#6448)
- Bump docker/build-push-action from 2 to 2.4.0 (#6454)
- Bump actions/stale from 3 to 3.0.18 (#6455)
- Bump actions/checkout from 2 to 2.3.4 (#6456)
- Bump docker/login-action from 1 to 1.9.0 (#6460)
- Bump actions/stale from 3.0.18 to 3.0.19 (#6477)
- Bump actions/stale from 3 to 3.0.18 (#300)
- Bump watchpack from 2.1.1 to 2.2.0 in /docs (#6482)
- Bump actions/stale from 3.0.18 to 3.0.19 (#302)
- Bump browserslist from 4.16.4 to 4.16.6 in /docs (#6487)
- Bump docker/build-push-action from 2.4.0 to 2.5.0 (#6496)
- Bump dns-packet from 1.3.1 to 1.3.4 in /docs (#6500)
- Bump actions/cache from 2.1.5 to 2.1.6 (#6504)
- Bump rtCamp/action-slack-notify from 2.1.3 to 2.2.0 (#6543)
- Bump github.com/prometheus/client_golang (#6552)

### Changelog

- Add summary & fix link & add external contributor (#3490)
- Add v0.31.9 and v0.31.8 updates (#4034)
- Fix typo (#4106)
- Explain breaking changes better
- GotVoteFromUnwantedRoundError -> ErrGotVoteFromUnwantedRound
- Add 0.32.9 changelog to master (#4305)
- Add entries from secruity releases
- Update 0.33.6 (#5075)
- Note breaking change in the 0.33.6 release (#5077)
- Reorgranize (#5065)
- Move entries from pending  (#5172)
- Bump to 0.34.0-rc2 (#5176)
- Add v0.33.7 release (#5203)
- Add v0.32.13 release (#5204)
- Update for 0.34.0-rc3 (#5240)
- Add v0.33.8 from release (#5242)
- Minor tweaks (#5389)
- Add missing date to v0.33.5 release, fix indentation (#5454)
- Add missing date to v0.33.5 release, fix indentation (#5454) (#5455)
- Prepare changelog for RC5 (#5494)
- Squash changelog from 0.34 RCs into one (#5691)
- Squash changelog from 0.34 RCs into one (#5687)
- Add entry back (#5738)
- Update changelog for v0.34.1 (#5872)
- Update with changes released in 0.34.1 (#5875)
- Prepare 0.34.2 release (#5894)
- Update changelogs to reflect changes released in 0.34.2
- Update for 0.34.3 (#5926)
- Update changelog for v0.34.3 (#5927)
- Update for v0.34.4 (#6096)
- Improve with suggestions from @melekes (#6097)
- Update to reflect v0.34.4 release (#6105)
- Update 0.34.3 changelog with details on security vuln (#6108)
- Update for 0.34.5 (#6129)
- Bump to v0.34.6
- Fix changelog pending version numbering (#6149)
- Update with changes from 0.34.7 (and failed 0.34.5, 0.34.6) (#6150)
- Update for 0.34.8 (#6181)
- Update for 0.34.8 (#6183)
- Prepare changelog for 0.34.9 release (#6333)
- Update to reflect 0.34.9 (#6334)
- Update for 0.34.10 (#6357)
- Update for 0.34.10 (#6358)

### Ci

- Transition some ci to github actions
- Only run when applicable (#4752)
- Check git diff on each job (#4770)
- Checkout code before git diff check (#4779)
- Add paths 
- Bump the timeout for test_coverage (#4864)
- Migrate localnet to github actions (#4878)
- Add timeouts (#4912)
- Migrate test_cover (#4869)
- Fix spacing of if statement (#5015)
- Try to fix codecov (#5095)
- Only run tests when go files are touched (#5097)
- Version linter fix  (#5128)
- Freeze golangci action version (#5196)
- Fix net pipeline (#5272)
- Delay codecov notification  (#5275)
- Add markdown linter (#146)
- Add dependabot config (#148)
- Fix net run (#5343)
- Docker remvoe circleci and add github action (#5420)
- Docker remove circleci and add github action (#5551)
- Add goreleaser (#5527)
- Tests (#5577)
- Add goreleaser (#5527)
- Tests (#5577)
- Use gh pages (#5609)
- Remove `add-path` (#5674)
- Remove `add-path` (#5674)
- Remove circle (#5714)
- Build for 32 bit, libs: fix overflow (#5700)
- Build for 32 bit, libs: fix overflow (#5700)
- Install BLS library in lint.yml and bump its go version (#15)
- E2e fixes - docker image, e2e.yml BLS library, default KeyType (#21)
- Make timeout-minutes 8 for golangci (#5821)
- Run `goreleaser build` (#5824)
- Add janitor (#6292)

### Ci/e2e

- Avoid running job when no go files are touched (#5471)
- Avoid running job when no go files are touched (#5471)

### Circleci

- Run P2P IPv4 and IPv6 tests in parallel (#4459)
- Fix reproducible builds test (#4497)
- Remove Gitian reproducible_builds job (#5462)
- Remove Gitian reproducible_builds job (#5462)

### Cli

- Add option to not clear address book with unsafe reset (#3606)
- Add `--cs.create_empty_blocks_interval` flag (#4205)
- Add `--db_backend` and `--db_dir` flags to tendermint node cmd (#4235)
- Add optional `--genesis_hash` flag to check genesis hash upon startup (#4238)
- Debug sub-command (#4227)
- Add command to generate shell completion scripts (#4665)
- Light home dir should default to where the full node default is (#5392)
- Light home dir should default to where the full node default is (#5392)

### Cmd

- Show useful error when tm not initialised (#4512)
- Fix debug kill and change debug dump archive filename format (#4517)
- Add support for --key (#5612)
- Modify `gen_node_key` to print key to STDOUT (#5772)
- Hyphen case cli and config (#5777)
- Hyphen-case cli  v0.34.1 (#5786)
- Ignore missing wal in debug kill command (#6160)

### Cmd/debug

- Execute p.Signal only when p is not nil (#4271)

### Cmd/lite

- Switch to new lite2 package (#4300)

### Codecov

- Disable annotations (#5413)
- Validate codecov.yml (#5699)

### Codeowners

- Add code owners (#82)

### Common

- CMap: slight optimization in Keys() and Values(). (#3567)

### Config

- Make possible to set absolute paths for TLS cert and key (#3765)
- Move max_msg_bytes into mempool section (#3869)
- Add rocksdb as a db backend option (#4239)
- Allow fastsync.version = v2 (#4639)
- Trust period consistency (#5297)
- Rename prof_laddr to pprof_laddr and move it to rpc (#5315)
- Set time_iota_ms to timeout_commit in test genesis (#5386)
- Add state sync discovery_time setting (#5399)
- Set statesync.rpc_servers when generating config file (#5433)
- Set statesync.rpc_servers when generating config file (#5433) (#5438)
- Increase MaxPacketMsgPayloadSize to 1400
- Fix mispellings (#5914)
- Fix mispellings (#5914)
- Create `BootstrapPeers` p2p config parameter (#6372)
- Add private peer id /net_info expose information in default config (#6490)
- Seperate priv validator config into seperate section (#6462)

### Config/indexer

- Custom event indexing (#6411)

### Consensus

- Reduce "Error attempting to add vote" message severity (Er… (#3871)
- Reduce log severity for ErrVoteNonDeterministicSignature (#4431)
- Add comment as to why use mocks during replay (#4785)
- Fix TestSimulateValidatorsChange
- Fix and rename TestStateLockPOLRelock (#4796)
- Bring back log.Error statement (#4899)
- Increase ensureTimeout (#4891)
- Fix startnextheightcorrectly test (#4938)
- Attempt to repair the WAL file on data corruption (#4682)
- Change logging and handling of height mismatch (#4954)
- Stricter on LastCommitRound check (#4970)
- Proto migration (#4984)
- Do not allow signatures for a wrong block in commits
- Msg testvectors (#5076)
- Added byzantine test, modified previous test (#5150)
- Only call privValidator.GetPubKey once per block (#5143)
- Don't check InitChain app hash vs genesis app hash, replace it (#5237)
- Double-sign risk reduction (ADR-51) (#5147)
- Fix wrong proposer schedule for `InitChain` validators (#5329)
- Check block parts don't exceed maximum block bytes (#5431)
- Check block parts don't exceed maximum block bytes (#5436)
- Open target WAL as read/write during autorepair (#5536)
- Open target WAL as read/write during autorepair (#5536) (#5547)
- Fix flaky tests (#5734)
- Change log level to error when adding vote
- Deprecate time iota ms (#5792)
- Groom Logs (#5917)
- P2p refactor (#5969)
- Groom Logs (#5917)
- Remove privValidator from log call (#6128)
- More log grooming (#6140)
- Log private validator address and not struct (#6144)
- More log grooming (bp #6140) (#6143)
- Reduce shared state in tests (#6313)
- Add test vector for hasvote (#6469)

### Consensus/types

- Fix BenchmarkRoundStateDeepCopy panics (#4244)

### Contributing

- Add steps for adding and removing rc branches (#5223)
- Include instructions for a release candidate (#5498)
- Simplify our minor release process (#5749)

### Core

- Update a few sections  (#284)

### Cov

- Ignore autogen file (#5033)

### Crypto

- Delete unused code (#3426)
- Proof of Concept for iterative version of SimpleHashFromByteSlices (#2611) (#3530)
- Add sr25519 signature scheme (#4190)
- Fix sr25519 from raw import (#4272)
- Remove SimpleHashFromMap() and SimpleProofsFromMap()
- Remove key suffixes (#4941)
- Removal of multisig (#4988)
- Consistent api across keys (#5214)
- API modifications (#5236)
- Remove secp256k1 (#5280)
- Remove proto privatekey (#5301)
- Reword readme (#5349)
- Add in secp256k1 support (#5500)
- Add in secp256k1 support (#5500)
- Adopt zip215 ed25519 verification (#5632)
- Fix infinite recursion in Secp256k1 string formatting (#5707)
- Fix infinite recursion in Secp256k1 string formatting (#5707) (#5709)
- Ed25519 & sr25519 batch verification (#6120)
- Add sr25519 as a validator key (#6376)

### Crypto/amino

- Add function to modify key codec (#4112)

### Crypto/merkle

- Remove simple prefix (#4989)
- Pre-allocate data slice in innherHash (#6443)
- Optimize merkle tree hashing (#6513)

### Crypto|p2p|state|types

- Rename Pub/PrivKey's TypeIdentifier() and Type()

### Cs

- Fix nondeterministic tests (#3582)
- Exit if SwitchToConsensus fails (#3706)
- Check for SkipTimeoutCommit or wait timeout in handleTxsAvailable (#3928)
- Don't panic when block is not found in store (#4163)
- Clarify where 24 comes from in maxMsgSizeBytes (wal.go)
- Set missing_validators(_power) metrics to 0 for 1st block (#4194)
- Check if cs.privValidator is nil (#4295)

### Cs/replay

- Check appHash for each block (#3579)
- ExecCommitBlock should not read from state.lastValidators (#3067)

### Db

- Add support for badgerdb (#5233)
- Migration script for key format change (#6355)

### Dep

- Update tm-db to 0.4.0 (#4289)
- Bump gokit dep (#4424)
- Maunally bump dep (#4436)
- Bump protobuf, cobra, btcutil & std lib deps (#4676)
- Bump ed25519consensus version (#5760)
- Remove IAVL dependency (#6550)

### Deps

- Update gogo/protobuf version from v1.2.1 to v1.3.0 (#3947)
- Bump github.com/magiconair/properties from 1.8.0 to 1.8.1 (#3937)
- Bump github.com/rs/cors from 1.6.0 to 1.7.0 (#3939)
- Bump github.com/fortytw2/leaktest from 1.2.0 to 1.3.0 (#3943)
- Bump github.com/libp2p/go-buffer-pool from 0.0.1 to 0.0.2 (#3948)
- Bump google.golang.org/grpc from 1.22.0 to 1.23.0 (#3942)
- Bump github.com/gorilla/websocket from 1.2.0 to 1.4.1 (#3945)
- Bump viper to 1.4.0 and logfmt to 0.4.0 (#3950)
- Bump github.com/stretchr/testify from 1.3.0 to 1.4.0 (#3951)
- Bump github.com/go-kit/kit from 0.6.0 to 0.9.0 (#3952)
- Bump google.golang.org/grpc from 1.23.0 to 1.23.1 (#3982)
- Bump google.golang.org/grpc from 1.23.1 to 1.24.0 (#4021)
- Bump google.golang.org/grpc from 1.25.0 to 1.25.1 (#4127)
- Bump google.golang.org/grpc from 1.25.1 to 1.26.0 (#4264)
- Bump github.com/go-logfmt/logfmt from 0.4.0 to 0.5.0 (#4282)
- Bump github.com/pkg/errors from 0.8.1 to 0.9.0 (#4301)
- Bump github.com/spf13/viper from 1.6.1 to 1.6.2 (#4318)
- Bump github.com/golang/protobuf from 1.3.2 to 1.3.3 (#4359)
- Bump google.golang.org/grpc from 1.27.0 to 1.27.1 (#4372)
- Bump github.com/stretchr/testify from 1.4.0 to 1.5.0 (#4435)
- Bump github.com/tendermint/tm-db from 0.4.0 to 0.4.1 (#4476)
- Bump github.com/Workiva/go-datastructures (#4519)
- Bump deps that bot cant (#4555)
- Run go mod tidy (#4587)
- Bump tm-db to 0.6.0 (#5058)

### E2e

- Use ed25519 for secretConn (remote signer) (#5678)
- Use ed25519 for secretConn (remote signer) (#5678)
- Releases nightly (#5906)
- Add control over the log level of nodes (#5958)
- Releases nightly (#5906)
- Disconnect maverick (#6099)
- Adjust timeouts to be dynamic to size of network (#6202)
- Add benchmarking functionality (#6210)
- Integrate light clients (#6196)
- Add benchmarking functionality (bp #6210) (#6216)
- Fix light client generator (#6236)
- Integrate light clients (bp #6196)
- Fix perturbation of seed nodes (#6272)
- Add evidence generation and testing (#6276)
- Tx load to use broadcast sync instead of commit (#6347)
- Tx load to use broadcast sync instead of commit (backport #6347) (#6352)
- Relax timeouts (#6356)
- Split out nightly tests (#6395)
- Prevent non-viable testnets (#6486)

### Encoding

- Remove codecs (#4996)
- Add secp, ref zip215, tables (#212)

### Events

- Add block_id to NewBlockEvent (#6478)

### Evidence

- Enforce ordering in DuplicateVoteEvidence (#4151)
- Introduce time.Duration to evidence params (#4254)
- Add time to evidence params (#69)
- Both MaxAgeDuration and MaxAgeNumBlocks need to be surpassed (#4667)
- Handling evidence from light client(s) (#4532)
- Remove unused param (#4726)
- Remove pubkey from duplicate vote evidence
- Add doc.go
- Protect valToLastHeight w/ mtx
- Check evidence is pending before validating evidence
- Refactor evidence mocks throughout packages (#4787)
- Cap evidence to an absolute number (#4780)
- Create proof of lock change and implement it in evidence store (#4746)
- Prevent proposer from proposing duplicate pieces of evidence (#4839)
- Remove header from phantom evidence (#4892)
- Retrieve header at height of evidence for validation (#4870)
- Json tags for DuplicateVoteEvidence (#4959)
- Migrate reactor to proto (#4949)
- Adr56 form amnesia evidence (#4821)
- Improve amnesia evidence handling (#5003)
- Replace mock evidence with mocked duplicate vote evidence (#5036)
- Fix data race in Pool.updateValToLastHeight() (#5100)
- Check lunatic vote matches header (#5093)
- New evidence event subscription (#5108)
- Minor correction to potential amnesia ev validate basic (#5151)
- Remove phantom validator evidence (#5181)
- Don't stop evidence verification if an evidence fails (#5189)
- Fix usage of time field in abci evidence (#5201)
- Change evidence time to block time (#5219)
- Remove validator index verification (#5225)
- Modularise evidence by moving verification function into evidence package (#5234)
- Remove ConflictingHeaders type (#5317)
- Remove lunatic  (#5318)
- Remove amnesia & POLC (#5319)
- Introduction of LightClientAttackEvidence and refactor of evidence lifecycle (#5361)
- Update data structures (#165)
- Use bytes instead of quantity to limit size (#5449)
- Use bytes instead of quantity to limit size (#5449)(#5476)
- Don't gossip consensus evidence too soon (#5528)
- Don't send committed evidence and ignore inbound evidence that is already committed (#5574)
- Don't gossip consensus evidence too soon (#5528)
- Don't send committed evidence and ignore inbound evidence that is already committed (#5574)
- Structs can independently form abci evidence (#5610)
- Structs can independently form abci evidence (#5610)
- Update data structures to reflect added support of abci evidence (#213)
- Omit bytes field (#5745)
- Omit bytes field (#5745)
- P2p refactor (#5747)
- Buffer evidence from consensus (#5890)
- Buffer evidence from consensus (#5890)
- Terminate broadcastEvidenceRoutine when peer is stopped (#6068)
- Fix bug with hashes (#6375)
- Fix bug with hashes (backport #6375) (#6381)
- Separate abci specific validation (#6473)

### Example/kvstore

- Return ABCI query height (#4509)

### Format

- Add format cmd & goimport repo (#4586)

### Genesis

- Add support for arbitrary initial height (#5191)
- Explain fields in genesis file (#270)

### Github

- Edit templates for use in issues and pull requests (#4483)
- Rename e2e jobs (#5502)
- Add nightly E2E testnet action (#5480)
- Add nightly E2E testnet action (#5480)
- Rename e2e jobs (#5502)
- Only notify nightly E2E failures once (#5559)
- Only notify nightly E2E failures once (#5559)
- Issue template for proposals (#190)
- Add @tychoish to code owners (#6273)
- Fix linter configuration errors and occluded errors (#6400)

### Gitian

- Update reproducible builds to build with Go 1.12.8 (#3902)

### Gitignore

- Add .vendor-new (#3566)

### Go.mod

- Upgrade iavl and deps (#5657)
- Upgrade iavl and deps (#5657)

### Goreleaser

- Lowercase binary name (#5765)
- Downcase archive and binary names (#6029)
- Downcase archive and binary names (#6029)
- Reintroduce arm64 build instructions

### Header

- Check block protocol (#5340)

### Improvement

- Update TxInfo (#6529)

### Indexer

- Allow indexing an event at runtime (#4466)
- Remove index filtering (#5006)
- Remove info log (#6194)
- Remove info log (#6194)

### Ints

- Stricter numbers (#4939)

### Json

- Add Amino-compatible encoder/decoder (#4955)

### Jsonrpc

- Change log to debug (#5131)

### Keys

- Change to []bytes  (#4950)

### Layout

- Add section titles (#240)

### Libs

- Remove useless code in group (#3504)
- Remove commented and unneeded code (#3757)
- Minor cleanup (#3794)
- Remove db from tendermint in favor of tendermint/tm-cmn (#3811)
- Remove bech32
- Remove kv (#4874)
- Wrap mutexes for build flag with godeadlock (#5126)
- Remove most of libs/rand (#6364)
- Internalize some packages (#6366)

### Libs/bits

- Inline defer and change order of mutexes (#5187)
- Validate BitArray in FromProto (#5720)

### Libs/clist

- Fix flaky tests (#6453)

### Libs/common

- Remove deprecated PanicXXX functions (#3595)
- Remove heap.go (#3780)
- Remove unused functions (#3784)
- Refactor libs/common 01 (#4230)
- Refactor libs/common 2 (#4231)
- Refactor libs common 3 (#4232)
- Refactor libs/common 4 (#4237)
- Refactor libs/common 5 (#4240)

### Libs/db

- Bbolt (etcd's fork of bolt) (#3610)
- Close boltDBIterator (#3627)
- Fix boltdb batching
- Conditional compilation (#3628)
- Boltdb: use slice instead of sync.Map (#3633)
- Remove deprecated `LevelDBBackend` const (#3632)
- Fix the BoltDB Batch.Delete
- Fix the BoltDB Get and Iterator

### Libs/fail

- Clean up `fail.go` (#3785)

### Libs/kv

- Remove unused type KI64Pair (#4542)

### Libs/log

- Format []byte as hexidecimal string (uppercased) (#5960)
- Format []byte as hexidecimal string (uppercased) (#5960)
- [JSON format] include timestamp (#6174)
- [JSON format] include timestamp (bp #6174) (#6179)
- Use fmt.Fprintf directly with *bytes.Buffer to avoid unnecessary allocations (#6503)

### Libs/os

- Add test case for TrapSignal (#5646)
- Remove unused aliases, add test cases (#5654)
- EnsureDir now returns IO errors and checks file type (#5852)
- EnsureDir now returns IO errors and checks file type (#5852)
- Avoid CopyFile truncating destination before checking if regular file (#6428)
- Avoid CopyFile truncating destination before checking if regular file (backport: #6428) (#6436)

### Libs/pubsub

- Relax tx querying (#4070)

### Libs/pubsub/query

- Add EXISTS operator (#4077)

### Libs/rand

- Fix "out-of-memory" error on unexpected argument (#5215)

### Light

- Rename lite2 to light & remove lite (#4946)
- Implement validate basic (#4916)
- Migrate to proto (#4964)
- Added more tests for pruning, initialization and bisection (#4978)
- Fix rpc calls: /block_results & /validators (#5104)
- Use bisection (not VerifyCommitTrusting) when verifying a head… (#5119)
- Return if target header is invalid (#5124)
- Update ADR 47 with light traces (#5250)
- Implement light block (#5298)
- Move dropout handling and invalid data to the provider (#5308)
- Expand on errors and docs (#5443)
- Cross-check the very first header (#5429)
- Cross-check the very first header (#5429)
- Model-based tests (#5461)
- Run detector for sequentially validating light client (#5538)
- Run detector for sequentially validating light client (#5538) (#5601)
- Make fraction parts uint64, ensuring that it is always positive (#5655)
- Make fraction parts uint64, ensuring that it is always positive (#5655)
- Ensure required header fields are present for verification (#5677)
- Minor fixes / standardising errors (#5716)
- Fix light store deadlock (#5901)
- Fix panic with RPC calls to commit and validator when height is nil (#6026)
- Fix panic with RPC calls to commit and validator when height is nil (#6040)
- Remove max retry attempts from client and add to provider (#6054)
- Remove witnesses in order of decreasing index (#6065)
- Create provider options struct (#6064)
- Improve timeout functionality (#6145)
- Improve provider handling (#6053)
- Handle too high errors correctly (#6346)
- Handle too high errors correctly (backport #6346) (#6351)
- Ensure trust level is strictly less than 1 (#6447)
- Spec alignment on verify skipping (#6474)

### Light/evidence

- Handle FLA backport (#6331)

### Light/provider/http

- Fix Validators (#6022)
- Fix Validators (#6024)

### Light/rpc

- Fix ABCIQuery (#5375)
- Fix ABCIQuery (#5375)

### Lint

- Golint issue fixes (#4258)
- Add review dog (#4652)
- Enable nolintlinter, disable on tests
- Various fixes
- Errcheck  (#5091)
- Add markdown linter (#5254)
- Add errchecks (#5316)
- Enable errcheck (#5336)
- Run gofmt and goimports  (#13)
- Fix lint errors (#301)

### Linter

- (1/2) enable errcheck (#5064)
- Fix some bls-related linter issues (#14)
- Fix nolintlint warnings (#6257)

### Linters

- Enable scopelint (#3963)
- Modify code to pass maligned and interfacer (#3959)
- Enable stylecheck (#4153)

### Linting

- Remove unused variable

### Lite

- Follow up from #3989 (#4209)
- Modified bisection to loop (#4400)
- Add helper functions for initiating the light client (#4486)
- Fix HTTP provider error handling

### Lite2

- Light client with weak subjectivity (#3989)
- Move AutoClient into Client (#4326)
- Improve auto update (#4334)
- Add Start, TrustedValidatorSet funcs (#4337)
- Rename alternative providers to witnesses (#4344)
- Refactor cleanup() (#4343)
- Batch save & delete operations in DB store (#4345)
- Panic if witness is on another chain (#4356)
- Make witnesses mandatory (#4358)
- Replace primary provider with alternative when unavailable (#4354)
- Fetch missing headers (#4362)
- Validate TrustOptions, add NewClientFromTrustedStore (#4374)
- Return if there are no headers in RemoveNoLongerTrustedHeaders (#4378)
- Manage witness dropout (#4380)
- Improve string output of all existing providers (#4387)
- Modified sequence method to match bisection (#4403)
- Disconnect from bad nodes (#4388)
- Divide verify functions (#4412)
- Return already verified headers and verify earlier headers (#4428)
- Don't save intermediate headers (#4452)
- Store current validator set (#4472)
- Cross-check first header and update tests (#4471)
- Remove expiration checks on functions that don't require them (#4477)
- Prune-headers (#4478)
- Return height as 2nd return param in TrustedValidatorSet (#4479)
- Actually run example tests + clock drift (#4487)
- Fix tendermint lite sub command (#4505)
- Remove auto update (#4535)
- Indicate success/failure of Update (#4536)
- Replace primary when providing invalid header (#4523)
- Add benchmarking tests (#4514)
- Cache headers in bisection (#4562)
- Use bisection for some of backward verification (#4575)
- Make maxClockDrift an option (#4616)
- Prevent falsely returned double voting error (#4620)
- Default to http scheme in provider.New (#4649)
- Verify ConsensusHash in rpc client
- Fix pivot height during bisection (#4850)
- Correctly return the results of the "latest" block (#4931)
- Allow bigger requests to LC proxy (#4930)
- Check header w/ witnesses only when doing bisection (#4929)
- Compare header with witnesses in parallel (#4935)

### Lite2/http

- Fix provider test by increasing the block retention value (#4890)

### Lite2/rpc

- Verify block results and validators (#4703)

### Localnet

- Fix node starting issue with --proxy-app flag (#5803)
- Expose 6060 (pprof) and 9090 (prometheus) on node0
- Use 27000 port for prometheus (#5811)
- Fix localnet by excluding self from persistent peers list (#6209)

### Logger

- Refactor Tendermint logger by using zerolog (#6534)

### Logging

- Print string instead of callback (#6177)
- Print string instead of callback (#6178)
- Shorten precommit log message (#6270)
- Shorten precommit log message (#6270) (#6274)

### Logs

- Cleanup (#6198)
- Cleanup (#6198)

### Make

- Add back tools cmd (#4281)
- Remove sentry setup cmds (#4383)

### Makefile

- Minor cleanup (#3994)
- Place phony markers after targets (#4408)
- Add options for other DBs (#5357)
- Remove call to tools (#6104)

### Markdownlint

- Ignore .github directory (#5351)

### Maverick

- Reduce some duplication (#6052)
- Reduce some duplication (#6052)

### Mempool

- Fix broadcastTxRoutine leak (#3478)
- Add a safety check, write tests for mempoolIDs (#3487)
- Move interface into mempool package (#3524)
- Remove only valid (Code==0) txs on Update (#3625)
- Make max_msg_bytes configurable (#3826)
- Make `max_tx_bytes` configurable instead of `max_msg_bytes` (#3877)
- Fix memory loading error on 32-bit machines (#3969)
- Moved TxInfo parameter into Mempool.CheckTx() (#4083)
- Reserve IDs in InitPeer instead of AddPeer
- Move mock into mempool directory
- Allow ReapX and CheckTx functions to run in parallel
- Do not launch broadcastTxRoutine if Broadcast is off
- Make it clear overwriting of pre/postCheck filters is intent… (#5054)
- Use oneof (#5063)
- Add RemoveTxByKey function (#5066)
- Return an error when WAL fails (#5292)
- Batch txs per peer in broadcastTxRoutine (#5321)
- Fix nil pointer dereference (#5412)
- Fix nil pointer dereference (#5412)
- Length prefix txs when getting them from mempool (#5483)
- Introduce KeepInvalidTxsInCache config option (#5813)
- Disable MaxBatchBytes (#5800)
- Introduce KeepInvalidTxsInCache config option (#5813)
- Disable MaxBatchBytes (#5800)
- P2p refactor (#5919)
- Fix reactor tests (#5967)
- Fix TestReactorNoBroadcastToSender (#5984)
- Fix mempool tests timeout (#5988)
- Don't return an error on checktx with the same tx (#6199)
- Remove vestigal mempool wal (#6396)
- Benchmark improvements (#6418)
- Add duplicate transaction and parallel checktx benchmarks (#6419)
- V1 implementation (#6466)

### Mempool/reactor

- Fix reactor broadcast test (#5362)

### Mempool/rpc

- Log grooming (#6201)
- Log grooming (bp #6201) (#6203)

### Mergify

- Remove unnecessary conditions (#4501)
- Use strict merges (#4502)
- Use PR title and body for squash merge commit (#4669)

### Merkle

- Return hashes for empty merkle trees (#5193)

### Metrics

- Only increase last_signed_height if commitSig for block (#4283)
- Switch from gauge to histogram (#5326)
- Change blocksize to a histogram (#6549)

### Mocks

- Update with 2.2.1 (#5294)

### Mod

- Go mod tidy

### Networks/remote

- Turn on GO111MODULE and use git clone instead of go get (#4203)

### Node

- Refactor node.NewNode (#3456)
- Fix a bug where `nil` is recorded as node's address (#3740)
- Run whole func in goroutine, not just logger.Error fn (#3743)
- Allow registration of custom reactors while creating node (#3771)
- Use GRPCMaxOpenConnections when creating the gRPC server (#4349)
- Don't attempt fast sync when InitChain sets self as only validator (#5211)
- Fix genesis state propagation to state sync (#5302)
- Improve test coverage on proposal block (#5748)
- Feature flag for legacy p2p support (#6056)
- Implement tendermint modes (#6241)
- Remove mode defaults. Make node mode explicit (#6282)
- Use db provider instead of mem db (#6362)
- Cleanup pex initialization (#6467)
- Change package interface (#6540)

### Node/state

- Graceful shutdown in the consensus state (#6370)

### Node/tests

- Clean up use of genesis doc and surrounding tests (#6554)

### Note

- Add nondeterministic note to events (#6220)
- Add nondeterministic note to events (#6220) (#6225)

### Os

- Simplify EnsureDir() (#5871)
- Simplify EnsureDir() (#5871)

### P2p

- Refactor GetSelectionWithBias for addressbook (#3475)
- Seed mode refactoring (#3011)
- Do not log err if peer is private (#3474)
- (seed mode) limit the number of attempts to connect to a peer (#3573)
- Session should terminate on nonce wrapping (#3531) (#3609)
- Make persistent prop independent of conn direction (#3593)
- PeerBehaviour implementation (#3539)  (#3552)
- Peer state init too late and pex message too soon (#3634)
- Per channel metrics (#3666) (#3677)
- Remove NewNetAddressStringWithOptionalID (#3711)
- Peerbehaviour follow up (#3653) (#3663)
- Refactor Switch#OnStop (#3729)
- Dial addrs which came from seed instead of calling ensurePeers (#3762)
- Extract ID validation into a separate func (#3754)
- Fix error logging for connection stop (#3824)
- Do not write 'Couldn't connect to any seeds' if there are no seeds (#3834)
- Only allow ed25519 pubkeys when connecting
- Log as debug msg when address dialing is already connected (#4082)
- Make SecretConnection non-malleable (#3668)
- Add `unconditional_peer_ids` and `persistent_peers_max_dial_period` (#4176)
- Extract maxBackoffDurationForPeer func and remove 1 test  (#4218)
- Merlin based malleability fixes (#72)
- Use curve25519.X25519() instead of ScalarMult() (#4449)
- PEX message abuse should ban as well as disconnect (#4621)
- Limit the number of incoming connections
- Set RecvMessageCapacity to maxMsgSize in all reactors
- Return err on `signChallenge` (#4795)
- Return masked IP (not the actual IP) in addrbook#groupKey
- TestTransportMultiplexAcceptNonBlocking and TestTransportMultiplexConnFilterTimeout (#4868)
- Remove nil guard (#4901)
- Expose SaveAs on NodeKey (#4981)
- Proto leftover (#4995)
- Remove data race bug in netaddr stringer (#5048)
- Ensure peers can't change IP of known nodes (#5136)
- Reduce log severity (#5338)
- Remove p2p.FuzzedConnection and its config settings (#5598)
- Remove unused MakePoWTarget() (#5684)
- State sync reactor refactor (#5671)
- Implement new Transport interface (#5791)
- Remove `NodeInfo` interface and rename `DefaultNodeInfo` struct (#5799)
- Do not format raw msg bytes
- Update frame size (#235)
- Fix data race in MakeSwitch test helper (#5810)
- Add MemoryTransport, an in-memory transport for testing (#5827)
- Rename ID to NodeID
- Add NodeID.Validate(), replaces validateID()
- Replace PeerID with NodeID
- Rename NodeInfo.DefaultNodeID to NodeID
- Rename PubKeyToID to NodeIDFromPubKey
- Fix IPv6 address handling in new transport API (#5853)
- Fix MConnection inbound traffic statistics and rate limiting (#5868)
- Fix MConnection inbound traffic statistics and rate limiting (#5868) (#5870)
- Add Router prototype (#5831)
- Add prototype peer lifecycle manager (#5882)
- Revise shim log levels (#5940)
- Improve PeerManager prototype (#5936)
- Make PeerManager.DialNext() and EvictNext() block (#5947)
- Improve peerStore prototype (#5954)
- Simplify PeerManager upgrade logic (#5962)
- Add PeerManager.Advertise() (#5957)
- Add prototype PEX reactor for new stack (#5971)
- Resolve PEX addresses in PEX reactor (#5980)
- Use stopCtx when dialing peers in Router (#5983)
- Clean up new Transport infrastructure (#6017)
- Tighten up and test Transport API (#6020)
- Add tests and fix bugs for `NodeAddress` and `NodeID` (#6021)
- Tighten up and test PeerManager (#6034)
- Tighten up Router and add tests (#6044)
- Enable scheme-less parsing of IPv6 strings (#6158)
- Links (#268)
- Revised router message scheduling (#6126)
- Metrics (#6278)
- Simple peer scoring (#6277)
- Rate-limit incoming connections by IP (#6286)
- Fix "Unknown Channel" bug on CustomReactors (#6297)
- Connect max inbound peers configuration to new router (#6296)
- Filter peers by IP address and ID (#6300)
- Improve router test stability (#6310)
- Extend e2e tests for new p2p framework (#6323)
- Make peer scoring test more resilient (#6322)
- Fix using custom channels (#6339)
- Minor cleanup + update router options (#6353)
- Fix network update test (#6361)
- Update state sync messages for reverse sync (#285)
- Improve PEX reactor (#6305)
- Support private peer IDs in new p2p stack (#6409)
- Wire pex v2 reactor to router (#6407)
- Add channel descriptors to open channel (#6440)
- Revert change to routePeer (#6475)
- Limit rate of dialing new peers (#6485)
- Renames for reactors and routing layer internal moves (#6547)

### P2p/conn

- Add Bufferpool (#3664)
- Simplify secret connection handshake malleability fix with merlin (#4185)
- Add a test for MakeSecretConnection (#4829)
- Migrate to Protobuf (#4990)
- Check for channel id overflow before processing receive msg (#6522)
- Check for channel id overflow before processing receive msg (backport #6522) (#6528)

### P2p/pex

- Consult seeds in crawlPeersRoutine (#3647)
- Fix DATA RACE
- Migrate to Protobuf (#4973)
- Fix flaky tests (#5733)
- Cleanup to pex internals and peerManager interface (#6476)
- Reuse hash.Hasher per addrbook for speed (#6509)

### P2p/test

- Wait for listener to get ready (#4881)
- Fix Switch test race condition (#4893)

### Params

- Remove block timeiota (#248)
- Remove blockTimeIota (#5987)

### Pex

- Dial seeds when address book needs more addresses (#3603)
- Various follow-ups (#3605)
- Use highwayhash for pex bucket
- Fix send requests too often test (#6437)

### Privval

- Increase timeout to mitigate non-deterministic test failure (#3580)
- Remove misplaced debug statement (#4103)
- Add `SignerDialerEndpointRetryWaitInterval` option (#4115)
- Return error on getpubkey (#4534)
- Remove deprecated `OldFilePV`
- Retry GetPubKey/SignVote/SignProposal N times before
- Migrate to protobuf (#4985)
- If remote signer errors, don't retry (#5140)
- Add chainID to requests (#5239)
- Allow passing options to NewSignerDialerEndpoint (#5434)
- Allow passing options to NewSignerDialerEndpoint (#5434) (#5437)
- Fix ping message encoding (#5441)
- Fix ping message encoding (#5442)
- Make response values non nullable (#5583)
- Make response values non nullable (#5583)
- Increase read/write timeout to 5s and calculate ping interval based on it (#5638)
- Reset pingTimer to avoid sending unnecessary pings (#5642)
- Increase read/write timeout to 5s and calculate ping interva… (#5666)
- Reset pingTimer to avoid sending unnecessary pings (#5642) (#5668)
- Duplicate SecretConnection from p2p package (#5672)
- Add grpc (#5725)
- Query validator key (#5876)
- Return errors on loadFilePV (#6185)
- Add ctx to privval interface (#6240)

### Prometheus/metrics

- Three new metrics for consensus (#4263)

### Proto

- Add buf and protogen script (#4369)
- Minor linting to proto files (#4386)
- Use docker to generate stubs (#4615)
- Bring over proto types & msgs (#4718)
- Regenerate proto (#4730)
- Remove test files
- Add proto files for ibc unblock (#4853)
- Add more to/from (#4956)
- Change to use gogofaster (#4957)
- Remove amino proto tests (#4982)
- Move keys to oneof (#4983)
- Leftover amino (#4986)
- Move all proto dirs to /proto (#5012)
- Folder structure adhere to buf (#5025)
- Increase lint level to basic and fix lint warnings (#5096)
- Improve enums (#5099)
- Reorganize Protobuf schemas (#5102)
- Minor cleanups (#5105)
- Change type + a cleanup (#5107)
- Add a comment for Validator#Address (#5144)
- Buf for everything (#5650)
- Bump gogoproto (1.3.2) (#5886)
- Bump gogoproto (1.3.2) (#5886)
- Docker deployment (#5931)
- Seperate native and proto types (#5994)
- Add files (#246)
- Docker deployment (#5931)
- Modify height int64 to uint64 (#253)

### Proto/p2p

- Rename PEX messages and fields (#5974)

### Proto/tendermint/abci

- Fix Request oneof numbers (#5116)

### Proxy

- Improve ABCI app connection handling (#5078)

### Reactors

- Omit incoming message bytes from reactor logs (#5743)
- Omit incoming message bytes from reactor logs (#5743)
- Remove bcv1 (#241)

### Reactors/pex

- Specify hash function (#94)
- Masked IP is used as group key (#96)

### Readme

- Fix link to original paper (#4391)
- Add discord to readme (#4533)
- Add badge for git tests (#4732)
- Add source graph badge (#4980)
- Remover circleci badge (#5729)
- Add links to job post (#5785)
- Update discord link (#5795)
- Add security mailing list (#5916)
- Add security mailing list (#5916)
- Cleanup (#262)

### Relase_notes

- Add release notes for v0.34.0

### Release

- Minor release 0.33.1 (#4401)

### Removal

- Remove build folder (#4565)

### Rfc

- Add end-to-end testing RFC (#5337)

### Rpc

- Client disable compression (#3430)
- Support tls rpc (#3469)
- Fix response time grow over time (#3537)
- Add support for batched requests/responses (#3534)
- /dial_peers: only mark peers as persistent if flag is on (#3620)
- Use Wrap instead of Errorf error (#3686)
- Make max_body_bytes and max_header_bytes configurable (#3818)
- /broadcast_evidence (#3481)
- Return err if page is incorrect (less than 0 or greater than tot… (#3825)
- Protect subscription access from race condition (#3910)
- Allow using a custom http client in rpc client (#3779)
- Remove godoc comments in favor of swagger docs (#4126)
- /block_results fix docs + write test + restructure response (#3615)
- Remove duplication of data in `ResultBlock ` (#3856)
- Add pagination to /validators (#3993)
- Update swagger docs to openapi 3.0 (#4223)
- Added proposer in consensus_state (#4250)
- Pass `outCapacity` to `eventBus#Subscribe` when subscribing using a l… (#4279)
- Add method block_by_hash (#4257)
- Modify New* functions to return error (#4274)
- Check nil blockmeta (#4320)
- PR#4320 follow up (#4323)
- Add sort_order option to tx_search (#4342)
- Fix issue with multiple subscriptions (#4406)
- Fix tx_search pagination with ordered results (#4437)
- Fix txsearch tests (#4438)
- Fix TxSearch test nits (#4446)
- Stop txSearch result processing if context is done (#4418)
- Keep the original subscription "id" field when new RPCs come in (#4493)
- Remove BlockStoreRPC in favor of BlockStore (#4510)
- Create buffered subscriptions on /subscribe (#4521)
- Fix panic when `Subscribe` is called (#4570)
- Add codespace to ResultBroadcastTx (#4611)
- Handle panics during panic handling
- Use a struct to wrap all the global objects
- Refactor lib folder (#4836)
- Increase waitForEventTimeout to 8 seconds (#4917)
- Add BlockByHash to Client (#4923)
- Replace Amino with new JSON encoder (#4968)
- Support EXISTS operator in /tx_search query (#4979)
- Add /check_tx endpoint (#5017)
- Move docs from doc.go to swagger.yaml (#5044)
- /broadcast_evidence nil evidence check (#5109)
- Make gasWanted/Used snake_case (#5137)
- Add private & unconditional to /dial_peer (#5293)
- Fix openapi spec syntax error (#5358)
- Fix test data races (#5363)
- Revert JSON-RPC/WebSocket response batching (#5378)
- Fix content-type header (#5661)
- Fix content-type header
- Standardize error codes (#6019)
- Change default sorting to desc for `/tx_search` results (#6168)
- Index block events to support block event queries (#6226)
- Index block events to support block event queries (bp #6226) (#6261)
- Define spec for RPC (#276)
- Remove global environment (#6426)
- Clean up client global state in tests (#6438)
- Add chunked rpc interface (#6445)
- Clarify timestamps (#304)
- Add chunked genesis endpoint (#299)
- Decouple test fixtures from node implementation (#6533)

### Rpc/client

- Include NetworkClient interface into Client interface (#3473)
- Add basic authentication (#4291)
- Split out client packages (#4628)
- Take context as first param (#5347)

### Rpc/client/http

- Log error (#5182)
- Do not drop events even if the `out` channel is full (#6163)
- Drop endpoint arg from New and add WSOptions (#6176)

### Rpc/core

- Do not lock ConsensusState mutex
- Return an error if `page=0` (#4947)
- More docs and a test for /blockchain endpoint (#5417)

### Rpc/jsonrpc

- Unmarshal RPCRequest correctly (#6191)
- Unmarshal RPCRequest correctly (bp #6191) (#6193)

### Rpc/jsonrpc/server

- Merge WriteRPCResponseHTTP and WriteRPCResponseAr (#5141)
- Ws server optimizations (#5312)
- Return an error in WriteRPCResponseHTTP(Error) (#6204)
- Return an error in WriteRPCResponseHTTP(Error) (bp #6204) (#6230)

### Rpc/lib

- Write a test for TLS server (#3703)
- Fix RPC client, which was previously resolving https protocol to http (#4131)

### Rpc/swagger

- Add numtxs to blockmeta (#4139)

### Rpc/test

- Fix test race in TestAppCalls (#4894)
- Wait for mempool CheckTx callback (#4908)
- Wait for subscription in TestTxEventsSentWithBroadcastTxAsync (#4907)

### Scripts

- Remove install scripts (#4242)
- Move build.sh into scripts
- Make linkifier default to 'pull' rather than 'issue' (#5689)

### Security

- Update policy after latest security release (#6336)

### Spec

- Update spec with tendermint updates (#62)
- Add ProofTrialPeriod to EvidenceParam (#99)
- Modify Header.LastResultsHash (#97)
- Link to abci server implementations (#100)
- Update evidence in blockchain.md (#108)
- Revert event hashing (#132)
- Update abci events (#151)
- Extract light-client to its own directory (#152)
- Remove evidences (#153)
- Light client attack detector (#164)
- Protobuf changes (#156)
- Update light client verification to match supervisor (#171)
- Remove reactor section (#242)
- Merge rust-spec (#252)

### Spec/abci

- Expand on Validator#Address (#118)

### Spec/consensus

- Canonical vs subjective commit

### Spec/consensus/signing

- Add more details about nil and amnesia (#54)

### Spec/reactors/mempool

- Batch txs per peer (#155)

### State

- Use last height changed if validator set is empty (#3560)
- Add more tests for block validation (#3674)
- Txindex/kv: fsync data to disk immediately after receiving it  (#4104)
- Txindex/kv: return an error if there's one (#4095)
- Export InitStateVersion
- Proto migration (#4951)
- Proto migration (#4972)
- Revert event hashing (#5159)
- Don't save genesis state in database when loaded (#5231)
- Define interface for state store (#5348)
- More test cases for block validation (#5415)
- Prune states using an iterator (#5864)
- Save in batches within the state store (#6067)
- Cleanup block indexing logs and null (#6263)
- Fix block event indexing reserved key check (#6314)
- Fix block event indexing reserved key check (#6314) (#6315)
- Keep a cache of block verification results (#6402)

### State/indexer

- Reconstruct indexer, move txindex into the indexer package (#6382)

### State/store

- Remove extra `if` statement (#3774)

### Statesync

- Use Protobuf instead of Amino for p2p traffic (#4943)
- Fix valset off-by-one causing consensus failures (#5311)
- Broadcast snapshot request to all peers on startup (#5320)
- Fix the validator set heights (again) (#5330)
- Check all necessary heights when adding snapshot to pool (#5516)
- Check all necessary heights when adding snapshot to pool (#5516) (#5518)
- Do not recover panic on peer updates (#5869)
- Improve e2e test outcomes (#6378)
- Improve e2e test outcomes (backport #6378) (#6380)
- Sort snapshots by commonness (#6385)
- Fix unreliable test (#6390)
- Ranking test fix (#6415)

### Store

- Register block amino, not just crypto (#3894)
- Proto migration (#4974)
- Order-preserving varint key encoding (#5771)
- Use db iterators for pruning and range-based queries (#5848)
- Fix deadlock in pruning (#6007)
- Use a batch instead of individual writes in SaveBlock (#6018)

### Swagger

- Update swagger port (#4498)
- Remove duplicate blockID 
- Define version (#4952)
- Update (#5257)

### Sync

- Move closer to separate file (#6015)

### Template

- Add labels to pr template

### Tm-bench

- Add deprecation warning (#3992)

### Tm-monitor

- Update build-docker Makefile target (#3790)
- Add Context to RPC handlers (#3792)

### Toml

- Make sections standout (#4993)

### Tool

- Add Mergify (#4490)

### Tooling

- Remove tools/Makefile (#6102)
- Remove tools/Makefile (bp #6102) (#6106)

### Tools

- Remove need to install buf (#4605)
- Update gogoproto get cmd (#5007)
- Use os home dir to instead of the hardcoded PATH (#6498)

### Tools.mk

- Use tags instead of revisions where possible
- Install protoc

### Tools/build

- Delete stale tools (#4558)

### Tools/tm-bench

- Remove tm-bench in favor of tm-load-test (#4169)

### Tools/tm-signer-harness

- Fix listener leak in newTestHarnessListener() (#5850)
- Fix listener leak in newTestHarnessListener() (#5850)

### Tx

- Reduce function to one parameter (#5493)

### Txindexer

- Refactor Tx Search Aggregation (#3851)

### Types

- CommitVotes struct as last step towards #1648 (#3298)
- Do not ignore errors returned by PublishWithEvents (#3722)
- Move MakeVote / MakeBlock functions (#3819)
- Add test for block commits with votes for the wrong blockID (#3936)
- Prevent temporary power overflows on validator updates  (#4165)
- Change number_txs to num_txs json tag in BlockMeta
- Remove dots from errors in SignedHeader#ValidateBasic
- Change `Commit` to consist of just signatures (#4146)
- Prevent spurious validator power overflow warnings when changing the validator set (#4183)
- VerifyCommitX return when +2/3 sigs are verified (#4445)
- Implement Header#ValidateBasic (#4638)
- Return an error if voting power overflows
- Sort validators by voting power
- Simplify VerifyCommitTrusting
- Remove extra validation in VerifyCommit
- Assert specific error in TestValSetUpdateOverflowRelated
- Remove unnecessary sort call (#4876)
- Create ValidateBasic() funcs for validator and validator set (#4905)
- Remove VerifyFutureCommit (#4961)
- Migrate params to protobuf (#4962)
- Remove duplicated validation in VerifyCommit (#4991)
- Add tests for blockmeta (#5013)
- Remove pubkey options (#5016)
- More test cases for TestValidatorSet_VerifyCommit (#5018)
- Rename partsheader to partsetheader (#5029)
- Fix evidence timestamp calculation (#5032)
- Add AppVersion to ConsensusParams (#5031)
- Reject blocks w/ ConflictingHeadersEvidence (#5041)
- Simplify safeMul (#5061)
- Verify commit fully
- Validatebasic on from proto (#5152)
- Check if nil or empty valset (#5167)
- Comment on need for length prefixing (#5283)
- Rename json parts to part_set_header (#5523)
- Move `MakeBlock` to block.go (#5573)
- Cleanup protobuf.go (#6023)
- Refactor EventAttribute (#6408)
- Fix verify commit light / trusting bug (#6414)
- Revert breaking change (#6538)

### Types/test

- Remove slow test cases in TestValSetUpdatePriorityOrderTests (#4903)

### Upgrading

- Add note on rpc/client subpackages (#4636)
- State store change (#5364)
- Update 0.34 instructions with updates since RC4 (#5685)
- Update 0.34 instructions with updates since RC4 (#5686)

### Ux

- Use docker to format proto files (#5384)

### Version

- Bump version numbers (#5173)
- Add abci version to handshake (#5706)
- Revert version through ldflag only (#6494)

### Ws

- Parse remote addrs with trailing dash (#6537)

## [0.0.1] - 2019-03-19

### .github

- Split the issue template into two seperate templates (#2073)

### ADR

- Fix malleability problems in Secp256k1 signatures

### ADR-016

- Add versions to Block and State (#2644)
- Add protocol Version to NodeInfo (#2654)
- Update ABCI Info method for versions (#2662)

### BROKEN

- Attempt to replace go-wire.JSON with json.Unmarshall in rpc

### Bech32

- Wrap error messages

### CHANGELOG

- Update release date
- Update release date

### CRandHex

- Fix up doc to mention length of digits

### Connect2Switches

- Panic on err

### Docs

- Update description of seeds and persistent peers

### Documentation

- Go-events -> tmlibs/events
- Update for 0.10.0 [ci skip]"
- Add docs from website
- Tons of minor improvements
- Add conf.py
- Test
- Add sphinx Makefile & requirements
- Consolidate ADRs
- Convert markdown to rst
- Organize the specification
- Rpc docs to be slate, see #526, #629
- Use maxdepth 2 for spec
- Port website's intro for rtd
- Rst-ify the intro
- Fix image links
- Link fixes
- Clean a bunch of stuff up
- Logo, add readme, fixes
- Pretty-fy
- Give index a Tools section
- Update and clean up adr
- Use README.rst to be pulled from tendermint
- Re-add the images
- Add original README's from tools repo
- Convert from md to rst
- Update index.rst
- Move images in from tools repo
- Harmonize headers for tools docs
- Add kubes docs to mintnet doc, from tools
- Add original tm-bench/monitor files
- Organize tm-bench/monitor description
- Pull from tools on build
- Finish pull from tools
- Organize the directory, #656
- Add software.json from website (ecosystem)
- Rename file
- Add and re-format the ecosystem from website
- Pull from tools' master branch
- Using ABCI-CLI
- Remove last section from ecosystem
- Organize install a bit better
- Add ABCI implementations
- Added passchain to the ecosystem.rst in the applications section;
- Fix build warnings
- Add stratumn
- Add py-tendermint to abci-servers
- Remove mention of type byte
- Add info about tm-migrate
- Update abci example details [ci skip]
- Typo
- Smaller logo (200px)
- Comb through step by step
- Fixup abci guide
- Fix links, closes #860
- Add note about putting GOPATH/bin on PATH
- Correction, closes #910
- Update ecosystem.rst (#1037)
- Add abci spec
- Add counter/dummy code snippets
- Updates from review (#1076)
- Tx formats: closes #1083, #536
- Fix tx formats [ci skip]
- Update getting started [ci skip]
- Add document 'On Determinism'
- Wrong command-line flag
- The character for 1/3 fraction could not be rendered in PDF on readthedocs. (#1326)
- Update quick start guide (#1351)
- Build updates
- Add diagram, closes #1565 (#1577)
- Lil fixes
- Update install instructions, closes #1580
- Blockchain and consensus dirs
- Fix dead links, closes #1608
- Use absolute links (#1617)
- Update ABCI output (#1635)
- A link to quick install script
- Add BSD install script
- Start move back to md
- Cleanup/clarify build process
- Pretty fixes
- Some organizational cleanup
- DuplicateVoteEvidence
- Update abci links (#1796)
- Update js-abci example
- Minor fix for abci query peer filter
- Update address spec to sha2 for ed25519
- Remove node* files
- Update getting started and remove old script (now in scripts/install)
- Md fixes & latest tm-bench/monitor
- Modify blockchain spec to reflect validator set changes (#2107)
- Fix links & other imrpvoements
- Note max outbound peers excludes persistent
- Fix img links, closes #2214 (#2282)
- Deprecate RTD (#2280)
- Fix indentation for genesis.validators
- Remove json tags, dont use HexBytes
- Update vote, signature, time
- Fix encoding JSON
- Bring blockchain.md up-to-date
- Specify consensus params in state.md
- Fix note about ChainID size
- Remove tags from result for now
- Move app-dev/abci-spec.md to spec/abci/abci.md
- Update spec
- Refactor ABCI docs
- Fixes and more from #2249
- Add abci spec to config.js
- Improve docs on AppHash (#2363)
- Update link to rpc (#2361)
- Update README (#2393)
- Update secure-p2p doc to match the spec + current implementation
- Add missing changelog entry and comment (#2451)
- Add assets/instructions for local docs build (#2453)
- Consensus params and general merkle (#2524)
- Update config: ref #2800 & #2837
- Prepend cp to /usr/local with sudo (#2885)
- Small improvements (#2933)
- Fix js-abci example (#2935)
- Add client.Start() to RPC WS examples (#2936)
- Update ecosystem.json: add Rust ABCI (#2945)
- Add client#Start/Stop to examples in RPC docs (#2939)
- Relative links in docs/spec/readme.md, js-amino lib (#2977)
- Fixes from 'first time' review (#2999)
- Enable full-text search (#3004)
- Add edit on Github links (#3014)
- Update DOCS_README (#3019)
- Networks/docker-compose: small fixes (#3017)
- Add rpc link to docs navbar and re-org sidebar (#3041)
- Fix p2p readme links (#3109)
- Update link for rpc docs (#3129)
- Fix broken link (#3142)
- Fix RPC links (#3141)
- Explain how someone can run his/her own ABCI app on localnet (#3195)
- Update pubsub ADR (#3131)
- Fix lite client formatting (#3198)
- Fix links (#3220)
- Fix rpc Tx() method docs (#3331)
- Fix typo (#3373)
- Fix the reverse of meaning in spec (#3387)

### Fix

- Ansible playbook to deploy tendermint

### GroupReader#Read

- Return io.EOF if file is empty

### Makefile

- Go test --race
- Add gmt and lint
- Add 'build' target
- Add megacheck & some additional fixes
- Remove redundant lint
- Fix linter

### Optimize

- Using parameters in func (#2845)

### Proposal

- New Makefile standard template (#168)

### PubKeyFromBytes

- Return zero value PubKey on error

### Query

- Height -> LastHeight
- LastHeight -> Height :)

### R4R

- Add timeouts to http servers (#2780)
- Swap start/end in ReverseIterator (#2913)
- Split immutable and mutable parts of priv_validator.json (#2870)
- Config TestRoot modification for LCD test (#3177)

### README

- Add godoc instead of tedious MD regeneration
- Document the minimum Go version

### ResponseEndBlock

- Ensure Address matches PubKey if provided

### Security

- Use bytes.Equal for key comparison
- Remove RipeMd160.
- Implement PeerTransport

### Testing

- More verbosity
- Unexport internal function.
- Check err on cmd.Wait
- Test_libs all use Makefile
- Jq .result[1] -> jq .result
- P2p.seeds and p2p.pex
- Add simple client/server test with no addr prefix
- Update for abci-cli consolidation. shell formatting
- Sunset tmlibs/process.Process
- Wait for node heights before checking app hash
- Fix ensureABCIIsUp
- Fix test/app/counter_test.sh
- Longer timeout
- Add some timeouts
- Use shasum to avoid rarer dependency
- Less bash
- More smoothness
- Test itr.Value in checkValuePanics (#2580)

### Update

- JTMSP -> jABCI

### ValidatorSet#GetByAddress

- Return -1 if no validator was found

### WAL

- Better errors and new fail point (#3246)

### WIP

- Begin parallel refactoring with go-wire Write methods and MConnection
- Fix rpc/core
- More empty struct examples

### [docs

- Typo fix] remove misplaced "the"
- Typo fix] add missing "have"

### Abci

- Remove old repo docs
- Remove nested .gitignore
- Remove LICENSE
- Add comment for doc update
- Remove fee (#2043)
- Change validators to last_commit_info in RequestBeginBlock (#2074)
- Update readme for building protoc (#2124)
- Add next_validators_hash to header
- VoteInfo, ValidatorUpdate. See ADR-018
- Move round back from votes to commit
- Codespace (#2557)
- LocalClient improvements & bugfixes & pubsub Unsubscribe issues (#2748)

### Abci-cli

- Print OK if code is 0
- Prefix flag variables with flag

### Absent_validators

- Repeated int -> repeated bytes

### Addrbook

- Toggle strict routability

### Addrbook_test

- Preallocate memory for bookSizes (#3268)

### Adr

- Add 005 consensus params
- Update 007 trust metric usage
- Amend decisions for PrivValidator
- Update readme
- PeerTransport (#2069)
- Encoding for cryptography at launch (#2121)
- Protocol versioning
- Chain-versions
- Style fixes (#3206)

### Adr-009

- No pubkeys in beginblock
- Add references

### Adr-016

- Update int64->uint64; add version to ConsensusParams (#2667)

### Adr-018

- Abci validators

### Adr-021

- Note about tag spacers (#2362)

### Adr-029

- Update CheckBlock

### All

- No more anonymous imports
- Fix vet issues with build tags, formatting
- Gofmt (#1743)

### Ansible

- Update tendermint and basecoin versions
- Added option to provide accounts for genesis generation, terraform: added option to secure DigitalOcean servers, devops: added DNS name creation to tendermint terraform

### Appveyor

- Use make

### Arm

- Add install script, fix Makefile (#2824)

### Autofile

- Ensure file is open in Sync

### Batch

- Progress

### Bit_array

- Simplify subtraction

### Blockchain

- Explain isCaughtUp logic
- Fixing reactor tests
- Add comment in AddPeer. closes #666
- Add tests and more docs for BlockStore
- Update store comments
- Updated store docs/comments from review
- Deduplicate store header value tests
- Less fragile and involved tests for blockstore
- Block creator helper for compressing tests as per @ebuchman
- Note about store tests needing simplification ...
- Test fixes
- Update for new state
- Test wip for hard to test functionality [ci skip]
- Fix register concrete name. (#2213)

### Blockchain/pool

- Some comments and small changes

### Blockchain/reactor

- RespondWithNoResponseMessage for missing height

### Blockchain/store

- Comment about panics

### Certifiers

- Test uses WaitForHeight

### Changelog

- Add genesis amount->power
- More review fixes/release/v0.31.0 (#3427)

### Ci

- Setup abci in dependency step
- Move over abci-cli tests
- Reduce log output in test_cover (#2105)

### Circle

- Add metalinter to test
- Fix config.yml
- Add GOCACHE=off and -v to tests
- Save p2p logs as artifacts (#2566)

### Circleci

- Add a job to automatically update docs (#3005)
- Update go version (#3051)
- Removed complexity from docs deployment job  (#3396)

### Cleanup

- Replace common.Exit with log.Crit or log.Fatal

### Cli

- Support --root and --home
- More descriptive naming
- Viper.Set(HomeFlag, rootDir)
- Clean up error handling
- Use cobra's new ExactArgs() feature
- WriteDemoConfig -> WriteConfigVals

### Client

- ResultsCh chan json.RawMessage, ErrorsCh
- Wsc.String()
- Safe error handling
- Use vars for retry intervals

### Clist

- Reduce numTimes in test
- Speedup functions (#2208)
- Speedup Next by removing defers (#2511)

### Cmap

- Remove defers (#2210)

### Cmd

- Fixes for new config
- Query params are flags
- --consensus.no_empty_blocks
- Don't load config for version command. closes #620
- Dont wait for genesis. closes #562
- Make sure to have 'testnet' create the data directory for nonvals (#3409)

### Cmd/abci-cli

- Use a single connection per session
- Implement batch

### Cmd/tendermint

- Fix initialization file creation checks (#991)

### Cmd/tendermint/commands

- Update ParseConfig doc

### Cmd/tendermint/commands/lite

- Add tcp scheme to address URLs (#1297)

### Cmn

- Kill
- Fix race condition in prng
- Fix repeate timer test with manual ticker
- Fix race
- Fix HexBytes.MarshalJSON
- GetFreePort (#3255)

### Commands

- Run -> RunE

### Common

- ProtocolAndAddress
- Fingerprint comment
- WriteFileAtomic use tempfile in current dir
- Comments for Service
- No more relying on math/rand.DefaultSource
- Use names prng and mrand
- Use genius simplification of tests from @ebuchman
- Rand* warnings about cryptographic unsafety
- Fix BitArray.Update to avoid nil dereference
- BitArray: feedback from @adrianbrink to simplify tests
- IsHex should be able to handle 0X prefixed strings
- NewBitArray never crashes on negatives (#170)
- Remove {Left, Right}PadString (#168)
- NewBitArray never crashes on negatives (#170)
- Delete unused functions (#2452)

### Common/BitArray

- Reduce fragility with methods

### Common/IsDirEmpty

- Do not mask non-existance errors

### Common/rand

- Remove exponential distribution functions (#1979)

### Config

- Toggle authenticated encryption
- Pex_reactor -> pex
- Write all default options to config file
- Lil fixes
- Unexpose chainID
- Fix addrbook path to go in config
- Fix private_peer_ids
- Rename skip_upnp to upnp (#1827)
- 10x default send/recv rate (#1978)
- Reduce default mempool size (#2300)
- Add ValidateBasic (#2485)
- Refactor ValidateBasic (#2503)

### Consensus

- Comment about test_data [ci skip]
- Fix tests
- Improve logging for conflicting votes
- Better logging
- More comments
- IsProposer func
- Remove rs from handleMsg
- Log ProposalHeartbeat msg
- Test proposal heartbeat
- Recover panics in receive routine
- Remove support for replay by #HEIGHT. closes #567
- Use filepath for windows compatibility, closes #595
- Kill process on app error
- Ensure prs.ProposalBlockParts is initialized. fixes #810
- Fix for initializing block parts during catchup
- Make mempool_test deterministic
- Fix LastCommit log
- Crank timeout in timeoutWaitGroup
- Fix typo on ticker.go documentation
- Fix makeBlockchainFromWAL
- Remove log stmt. closes #987
- Note about duplicate evidence
- Rename test funcs
- Minor cosmetic
- Fix SetLogger in tests
- Print go routines in failed test
- Return from go-routine in test
- Return from errors sooner in addVote
- Close pubsub channels. fixes #1372
- Only fsync wal after internal msgs
- Link to spec from readme (#1609)
- Fixes #1754
- Fix addProposalBlockPart
- Stop wal
- Wait on stop if not fastsync
- Include evidence in proposed block parts. fixes #2050
- Fix test for blocks with evidence
- Failing test for ProposerAddress
- Wait timeout precommit before starting new round (#2493)
- Add ADR for first stage consensus refactor (#2462)
- Wait for proposal or timeout before prevote (#2540)
- Flush wal on stop (#3297)

### Consensus/WAL

- Benchmark WALDecode across data sizes

### Console

- Fix output, closes #93
- Fix tests

### Contributing

- Use full version from site

### Core

- Apply megacheck vet tool (unused, gosimple, staticcheck)

### Counter

- Fix tx buffer overflow

### Crypto

- Rename last traces of go-crypto (#1786)
- Abstract pubkey / signature size when known to constants (#1808)
- Refactor to move files out of the top level directory
- Remove Ed25519 and Secp256k1 suffix on GenPrivKey
- Fix package imports from the refactor
- Add benchmarking code for signature schemes (#2061)
- Switch hkdfchacha back to xchacha (#2058)
- Add compact bit array for intended usage in the multisig
- Remove interface from crypto.Signature
- Add compact bit array for intended usage in the multisig
- Threshold multisig implementation
- Add compact bit array for intended usage in the multisig (#2140)
- Remove unnecessary prefixes from amino route variable names (#2205)
- Add a way to go from pubkey to route (#2574)
- Use stdlib crypto/rand. ref #2099 (#2669)
- Revert to mainline Go crypto lib (#3027)

### Crypto/amino

- Address anton's comment on PubkeyAminoRoute (#2592)

### Crypto/ed25519

- Update the godocs (#2002)
- Remove privkey.Generate method (#2022)

### Crypto/hkdfchachapoly

- Add testing seal to the test vector

### Crypto/merkle

- Remove byter in favor of plain byte slices (#2595)

### Crypto/random

- Use chacha20, add forward secrecy (#2562)

### Crypto/secp256k1

- Add godocs, remove indirection in privkeys (#2017)
- Fix signature malleability, adopt more efficient en… (#2239)

### Cs

- Prettify logging of ignored votes (#3086)
- Reset triggered timeout precommit (#3310)
- Sync WAL more frequently (#3300)
- Update wal comments (#3334)
- Comment out log.Error to avoid TestReactorValidatorSetChanges timing out (#3401)

### Cs/wal

- Refuse to encode msg that is bigger than maxMsgSizeBytes (#3303)

### Db

- Fix memdb iterator
- Fix MemDB.Close
- Sort keys for memdb iterator
- Some comments in types.go
- Test panic on nil key
- Some test cleanup
- Fixes to fsdb and clevledb
- Memdb iterator
- Goleveldb iterator
- Cleveldb iterator
- Fsdb iterator
- Fix c and go iterators
- Simplify exists check, fix IsKeyInDomain signature, Iterator Close

### Dep

- Pin all deps to version or commit
- Revert updates

### Deps

- Update gogo/protobuf from 1.1.1 to 1.2.1 and golang/protobuf from 1.1.0 to 1.3.0 (#3357)

### Dist

- Dont mkdir in container
- Dont mkdir in container

### Distribution

- Lock binary dependencies to specific commits (#2550)

### Dummy

- Valset changes and tests
- Verify pubkey is go-crypto encoded in DeliverTx. closes #51
- Include app.key tag

### Ebuchman

- Added some demos on how to parse unknown types

### Ed25519

- Use golang/x/crypto fork (#2558)

### Errcheck

- PR comment fixes

### Evidence

- More funcs in store.go
- Store tests and fixes
- Pool test
- Reactor test
- Reactor test
- Dont send evidence to unsynced peers
- Check peerstate exists; dont send old evidence
- Give each peer a go-routine

### Example

- Fix func suffix

### Example/dummy

- Remove iavl dep - just use raw db

### Fmt

- Run 'make fmt'

### Github

- Update PR template to indicate changing pending changelog. (#2059)

### Glide

- Update go-common
- Update for autofile fix
- More external deps locked to versions
- Update grpc version

### Grpcdb

- Better readability for docs and constructor names
- Close Iterator/ReverseIterator after use (#3424)

### Hd

- Optimize ReverseBytes + add tests
- Comments and some cleanup

### Http

- Http-utils added after extraction

### Https

- //github.com/tendermint/tendermint/pull/1128#discussion_r162799294

### Json2wal

- Increase reader's buffer size (#3147)

### Keys

- Transactions.go -> types.go

### Keys/keybase.go

- Comments and fixes

### Libs

- Update BitArray go docs (#2079)
- Make bitarray functions lock parameters that aren't the caller (#2081)
- Remove usage of custom Fmt, in favor of fmt.Sprintf (#2199)
- Handle SIGHUP explicitly inside autofile (#2480)
- Call Flush()  before rename #2428 (#2439)
- Fix event concurrency flaw (#2519)
- Refactor & document events code (#2576)
- Let prefixIterator implements Iterator correctly (#2581)
- Test deadlock from listener removal inside callback (#2588)

### Libs/autofile

- Bring back loops (#2261)

### Libs/autofile/group_test

- Remove unnecessary logging (#2100)

### Libs/cmn

- Remove Tempfile, Tempdir, switch to ioutil variants (#2114)

### Libs/cmn/writefileatomic

- Handle file already exists gracefully (#2113)

### Libs/common

- Refactor tempfile code into its own file

### Libs/common/rand

- Update godocs

### Libs/db

- Add cleveldb.Stats() (#3379)
- Close batch (#3397)
- Close batch (#3397)

### Lint

- Remove dot import (go-common)
- S/common.Fmt/fmt.Sprintf
- S/+=1/++, remove else clauses
- Couple more fixes
- Apply deadcode/unused

### Linter

- Couple fixes
- Add metalinter to Makefile & apply some fixes
- Last fixes & add to circle
- Address deadcode, implement incremental lint testing
- Sort through each kind and address small fixes
- Enable in CI & make deterministic

### Linting

- Cover the basics
- Catch some errors
- Add to Makefile & do some fixes
- Next round  of fixes
- Fixup some stuffs
- Little more fixes
- A few fixes
- Replace megacheck with metalinter
- Apply 'gofmt -s -w' throughout
- Apply misspell
- Apply errcheck part1
- Apply errcheck part2
- Moar fixes
- Few more fixes

### Lite

- MemStoreProvider GetHeightBinarySearch method + fix ValKeys.signHeaders
- < len(v) in for loop check, as per @melekes' recommendation
- TestCacheGetsBestHeight with GetByHeight and GetByHeightBinarySearch
- Comment out iavl code - TODO #1183
- Add synchronization in lite verify (#2396)

### Lite/proxy

- Validation* tests and hardening for nil dereferences
- Consolidate some common test headers into a variable

### Localnet

- Fix $LOG variable (#3423)

### Log

- Tm -> TM

### Make

- Update protoc_abci use of awk

### Makefile

- Remove megacheck
- Fix protoc_libs
- Add `make check_dep` and remove `make ensure_deps` (#2055)
- Lint flags
- Fix build-docker-localnode target (#3122)

### Mempool

- Comments
- Reactor test
- Implement Mempool.CloseWAL
- Return error on cached txs
- Assert -> require in test
- Remove Peer interface. use p2p.Peer
- Cfg.CacheSize and expose InitWAL
- Fix cache_size==0. closes #1761
- Log hashes, not whole tx
- Chan bool -> chan struct{}
- Keep cache hashmap and linked list in sync (#2188)
- Store txs by hash inside of cache (#2234)
- Filter new txs if they have insufficient gas (#2385)
- ErrPreCheck and more log info (#2724)
- Print postCheck error (#2762)
- Add txs from Update to cache
- Add a comment and missing changelog entry (#2996)
- NotifyTxsAvailable if there're txs left, but recheck=false (#2991)
- Move tx to back, not front (#3036)
- Move tx to back, not front (#3036)
- Enforce maxMsgSize limit in CheckTx (#3168)
- Correct args order in the log msg (#3221)

### Merkle

- Go-common -> tmlibs
- Remove go-wire dep by copying EncodeByteSlice
- Remove unused funcs. unexport simplemap. improv docs
- Use amino for byteslice encoding

### Metalinter

- Add linter to Makefile like tendermint

### Metrics

- Add additional metrics to p2p and consensus (#2425)

### Nano

- Update comments

### Networks

- Update readmes

### Node

- ConfigFromViper
- NewNode takes DBProvider and GenDocProvider
- Clean makeNodeInfo
- Remove dup code from rebase
- Remove commented out trustMetric
- Respond always to OS interrupts (#2479)
- Refactor privValidator ext client code & tests (#2895)

### P2p

- Use cmn instead of .
- Fix race by peer.Start() before peers.Add()
- Fix test
- Sw.peers.List() is empty in sw.OnStart
- Put maxMsgPacketPayloadSize, recvRate, sendRate in config
- Test fix
- Fully test PeerSet, more docs, parallelize PeerSet tests
- Minor comment fixes
- Delete unused and untested *IPRangeCount functions
- Sw.AddPeer -> sw.addPeer
- Allow listener with no external connection
- Update readme, some minor things
- Some fixes re @odeke-em issues #813,#816,#817
- Comment on the wg.Add before go saveRoutine()
- Peer should respect errors from SetDeadline
- Use fake net.Pipe since only >=Go1.10 implements SetDeadline
- NetPipe for <Go1.10 in own file with own build tag
- Fix non-routable addr in test
- Fix comment on addPeer (thanks @odeke-em)
- Make Switch.DialSeeds use a new PRNG per call
- Disable trustmetric test while being fixed
- Exponential backoff on reconnect. closes #939
- PrivKey need not be Ed25519
- Reorder some checks in addPeer; add comments to NodeInfo
- Peer.Key -> peer.ID
- Add ID to NetAddress and use for AddrBook
- Support addr format ID@IP:PORT
- Authenticate peer ID
- Remove deprecated Dockerfile
- Seed mode fixes from rebase and review
- Seed disconnects after sending addrs
- Add back lost func
- Use sub dirs
- Tmconn->conn and types->p2p
- Use conn.Close when peer is nil
- Notes about ListenAddr
- AddrBook.Save() on DialPeersAsync
- Add Channels to NodeInfo and don't send for unknown channels
- Fix tests for required channels
- Fix break in double loop
- Introduce peerConn to simplify peer creation (#1226)
- Keep reference to connections in test peer
- Persistent - redial if first dial fails
- Switch - reconnect only if persistent
- Don't use dial funcn in peerconfig
- NodeInfo.Channels is HexBytes
- Dont require minor versions to match in handshake
- Explicit netaddress errors
- Some comments and a log line
- MinNumOutboundPeers. Closes #1501
- Change some logs from Error to Debug. #1476
- Small lint
- Prevent connections from same ip
- External address
- Reject addrs coming from private peers (#2032)
- Fix conn leak. part of #2046
- Connect to peers from a seed node immediately (#2115)
- Add test vectors for deriving secrets (#2120)
- Integrate new Transport
- Add RPCAddress to NodeInfoOther.String() (#2442)
- NodeInfo is an interface; General cleanup (#2556)
- Restore OriginalAddr (#2668)
- Peer-id -> peer_id (#2771)
- AddressBook requires addresses to have IDs; Do not close conn immediately after sending pex addrs in seed mode (#2797)
- Re-check after sleeps (#2664)
- Log 'Send failed' on Debug (#2857)
- NewMultiplexTransport takes an MConnConfig (#2869)
- Panic on transport error (#2968)
- Fix peer count mismatch #2332 (#2969)
- Set MConnection#created during init (#2990)
- File descriptor leaks (#3150)
- Fix infinite loop in addrbook (#3232)
- Check secret conn id matches dialed id (#3321)
- Fix comment in secret connection (#3348)
- Do not panic when filter times out (#3384)

### P2p/addrbook

- Comments
- AddrNew/Old -> bucketsNew/Old
- Simplify PickAddress
- AddAddress returns error. more defensive PickAddress
- Add non-terminating test
- Fix addToOldBucket
- Some comments

### P2p/conn

- Better handling for some stop conditions
- FlushStop. Use in pex. Closes #2092 (#2802)
- Don't hold stopMtx while waiting (#3254)

### P2p/connetion

- Remove panics, test error cases

### P2p/pex

- Simplify ensurePeers
- Wait to connect to all peers in reactor test
- Minor cleanup and comments
- Some addrbook fixes
- Allow configured seed nodes to not be resolvable over DNS (#2129)
- Fix mismatch between dialseeds and checkseeds. (#2151)

### P2p/secret_connection

- Switch salsa usage to hkdf + chacha

### P2p/trust

- Split into multiple files and improve function order
- Lock on Copy()
- Remove extra channels
- Fix nil pointer error on TrustMetric Copy() (#1819)

### P2p/trustmetric

- Non-deterministic test

### Premerge2

- Rpc -> rpc/tendermint

### Priv-val

- Fix timestamp for signing things that only differ by timestamp

### PrivVal

- Improve SocketClient network code (#1315)

### Privval

- Switch to amino encoding in SignBytes (#2459)
- Set deadline in readMsg (#2548)
- Add IPCPV and fix SocketPV (#2568)
- Fixes from review (#3126)
- Improve Remote Signer implementation (#3351)

### Protoc

- "//nolint: gas" directive after pb generation (#164)

### Proxy

- Remove Handshaker from proxy pkg (#2437)

### Pubsub

- Comments
- Fixes after Ethan's review (#3212)

### Readme

- Js-tmsp -> js-abci
- Update install instruction (#100)
- Re-organize & update docs links

### Remotedb

- A client package implementing the db.DB interface

### Repeat_timer

- Drain channel in Stop; done -> wg

### Rpc

- Use HTTP error codes
- Repsonse types use data.Bytes
- Response types use Result instead of pb Response
- Fix tests
- Decode args without wire
- Cleanup some comments [ci skip]
- Fix tests
- SetWriteDeadline for ws ping. fixes #553
- Move grpc_test from test/ to grpc/
- Typo fixes
- Comments
- Historical validators
- Block and Commit take pointers; return latest on nil
- Fix client websocket timeout (#687)
- Subscribe on reconnection (#689)
- Use /iavl repo in test (#713)
- Wait for rpc servers to be available in tests
- Fix tests
- Make time human readable. closes #926
- GetHeight helper function
- Fix getHeight
- Lower_case peer_round_states, use a list, add the node_address
- Docs/comments
- Add n_peers to /net_info
- Add voting power totals to vote bitarrays
- /consensus_state for simplified output
- Break up long lines
- Test Validator retrevial timeout
- Fix /blockchain OOM #2049
- Validate height in abci_query
- Log error when we timeout getting validators from consensus (#2045)
- Improve slate for Jenkins (#2070)
- Transform /status result.node_info.other into map (#2417)
- Add /consensus_params endpoint  (#2415)
- Fix tx.height range queries (#2899)
- Include peer's remote IP in `/net_info` (#3052)

### Rpc/client

- Use compile time assertions instead of methods

### Rpc/core

- Ints are strings in responses, closes #1896

### Rpc/lib

- No Result wrapper
- Test tcp and unix
- Set logger on ws conn
- Remove dead files, closes #710

### Rpc/lib/client

- Add jitter for exponential backoff of WSClient
- Jitter test updates and only to-be run on releases

### Rpc/lib/server

- Add handlers tests
- Update with @melekes and @ebuchman feedback
- Separate out Notifications test
- Minor changes to test
- Add test for int parsing

### Rpc/lib/types

- RPCResponse.Result is not a pointer

### Rpc/libs/doc

- Formatting for godoc, closes #2420

### Rpc/net_info

- Change RemoteIP type from net.IP to String (#3309)

### Rpc/wsevents

- Small cleanup

### Rtd

- Build fixes

### Scripts

- Quickest/easiest fresh install

### Scripts/txs

- Add 0x and randomness

### Secp256k1

- Use compressed pubkey, bitcoin-style address
- Change build tags (#3277)

### Server

- Allow multiple connections
- Return result with error
- Use cmn.ProtocolAndAddress
- Minor refactor

### Service

- Start/stop logs are info, ignored are debug
- Reset() for restarts

### Shame

- Forgot to add new code pkg

### SocketClient

- Fix and test for StopForError deadlock

### Spec

- Fixes from review
- Convert to rst
- Typos & other fixes
- Remove notes, see #1152
- More fixes
- Minor fixes
- Update encoding.md
- Note on byte arrays, clean up bitarrays and more, add merkle proof, add crypto.go script
- Add Address spec. notes about Query
- Pex update
- Abci notes. closes #1257
- Move to final location (#1576)
- Add missing field to NodeInfoOther (#2426)

### State

- Comments; use wire.BinaryBytes
- Persist validators
- Minor comment fixes
- Return to-be-used function
- TestValidateBlock
- Move methods to funcs
- BlockExecutor
- Re-order funcs. fix tests
- Send byzantine validators in BeginBlock
- Builds
- Fix txResult issue with UnmarshalBinary into ptr
- S -> state
- B -> block
- Err if 0 power validator is added to the validator set
- Format panics
- Require block.Time of the fist block to be genesis time (#2594)

### Throttle_timer

- Fix race, use mtx instead of atomic

### Tm-bench

- Improve code shape
- Update dependencies, add total metrics

### Tm-monitor

- Update health after we added / removed node (#2694)

### Tmbench

- Fix iterating through the blocks, update readme
- Make tx size configurable
- Update dependencies to use tendermint's master
- Make sendloop act in one second segments (#110)
- Make it more resilient to WSConn breaking (#111)

### Tmhash

- Add Sum function

### Tmtime

- Canonical, some comments (#2312)

### Tools

- Remove redundant grep -v vendors/ (#1996)
- Clean up Makefile and remove LICENSE file (#2042)
- Refactor tm-bench (#2570)

### Tools/tm-bench

- Don't count the first block if its empty
- Remove testing flags from help (#1949)
- Don't count the first block if its empty (#1948)
- Bounds check for txSize and improving test cases (#2410)

### Tools/tmbench

- Fix the end time being used for statistics calculation
- Improve accuracy with large tx sizes.
- Move statistics to a seperate file

### Types

- Pretty print validators
- Update LastBlockInfo and ConfigInfo
- []byte -> data.Bytes
- Result and Validator use data.Bytes
- Methods convert pb types to use data.Bytes
- Block comments
- Remove redundant version file
- PrivVal.Sign returns an error
- More . -> cmn
- Comments
- ConsensusParams test + document the ranges/limits
- ConsensusParams: add feedback from @ebuchman and @melekes
- Unexpose valset.To/FromBytes
- Add gas and fee fields to CheckTx
- Use data.Bytes directly in type.proto via gogo/protobuf. wow
- Consolidate some file
- Add note about ReadMessage having no cap
- RequestBeginBlock includes absent and byzantine validators
- Drop uint64 from protobuf.go
- IsOK()
- Int32 with gogo int
- Fix for broken customtype int in gogo
- Add MarshalJSON funcs for Response types with a Code
- Add UnmarshalJSON funcs for Response types
- Compile type assertions to avoid sneaky runtime surprises
- Check ResponseCheckTx too
- Update String() test to assert Prevote type
- Rename exampleVote to examplePrecommit on vote_test
- Add test for IsVoteTypeValid
- Params.Update()
- Comments; compiles; evidence test
- Evidences for merkle hashing; Evidence.String()
- Tx.go comments
- Evidence cleanup
- Better error messages for votes
- Check bufio.Reader
- TxEventBuffer.Flush now uses capacity preserving slice clearing idiom
- RequestInitChain.AppStateBytes
- Update for new go-wire. WriteSignBytes -> SignBytes
- Remove dep on p2p
- Tests build
- Builds
- Revert to old wire. builds
- Working on tests...
- P2pID -> P2PID
- Fix validator_set_test issue with UnmarshalBinary into ptr
- Bring back json.Marshal/Unmarshal for genesis/priv_val
- TestValidatorSetVerifyCommit
- Uncomment some tests
- Hash invoked for nil Data and Header should not panic
- Compile time assert to, and document sort.Interface
- Revert CheckTx/DeliverTx changes. make them the same
- Fix genesis.AppStateJSON
- Lock block on MakePartSet
- Fix formatting when printing signatures
- Allow genesis file to have 0 validators (#2148)
- Remove pubkey from validator hash (#2512)
- Cap evidence in block validation (#2560)
- Remove Version from CanonicalXxx (#2666)
- Dont use SimpleHashFromMap for header. closes #1841 (#2670)
- First field in Canonical structs is Type (#2675)
- Emit tags from BeginBlock/EndBlock (#2747)
- NewValidatorSet doesn't panic on empty valz list (#2938)
- ValidatorSet.Update preserves Accum (#2941)
- Comments on user vs internal events
- Validator set update tests (#3284)
- Followup after validator set changes (#3301)
- Remove check for priority order of existing validators (#3407)
- Refactor PB2TM.ConsensusParams to take BlockTimeIota as an arg (#3442)

### Types/heartbeat

- Test all Heartbeat functions

### Types/params

- Introduce EvidenceParams

### Types/priv_validator

- Fixes for latest p2p and cmn

### Types/time

- Add note about stripping monotonic part

### Types/validator_set_test

- Move funcs around

### Upnp

- Keep a link

### Version

- Add and bump abci version
- Types

### Wip

- Tendermint specification
- Priv val via sockets
- Comment types
- Fix code block in ADR
- Fix nil pointer deference
- Avoid underscore in var name
- Check error of wire read

### Wire

- No codec yet

### Ws

- Small comment

### WsConnection

- Call onDisconnect

