## [0.8.0-dev.5] - 2022-06-13

### Bug Fixes

- Consolidate all prerelease changes in latest full release changelog
- Mishandled pubkey read errors
- Fix dependencies in e2e tests
- Install libpcap-dev before running go tests
- Install missing dependencies for linter
- Fix race conditions in reactor

### Documentation

- Abcidump documentation

### Features

- Abci protocol parser
- Abci protocol parser - packet capture
- Parse CBOR messages

### Miscellaneous Tasks

- Don't fail due to missing bodyclose in go 1.18
- Cleanup during self-review
- Remove duplicate test
- Update go.mod

### Refactor

- Refactor cbor and apply review feedback
- Move abcidump from scripts/ to cmd/

### Security

- Merge result of tendermint/master with v0.8-dev (#376)

### Testing

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

### Build

- Bump google.golang.org/grpc from 1.45.0 to 1.46.0 (#8408)

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
- Bump github.com/golangci/golangci-lint from 1.45.0 to 1.45.2 (#8192)
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
- Remove option c form linux build (#305)

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
- Bump github.com/vektra/mockery/v2 from 2.9.4 to 2.10.0 (#7685)
- Bump github.com/prometheus/client_golang from 1.12.0 to 1.12.1 (#7732)
- Bump golangci/golangci-lint-action from 2.5.2 to 3.1.0 (#8026)
- Bump actions/checkout from 2.4.0 to 3 (#8076)
- Bump docker/login-action from 1.13.0 to 1.14.1 (#8075)
- Bump golangci/golangci-lint-action from 2.5.2 to 3.1.0 (#8074)
- Bump github.com/stretchr/testify from 1.7.0 to 1.7.1 (#8131)
- Bump gaurav-nelson/github-action-markdown-link-check from 1.0.13 to 1.0.14 (#8166)
- Bump docker/build-push-action from 2.9.0 to 2.10.0 (#8167)

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
- Bump docker/build-push-action from 2.7.0 to 2.8.0 (#7679)
- Bump vuepress-theme-cosmos from 1.0.182 to 1.0.183 in /docs (#7680)
- Bump github.com/vektra/mockery/v2 from 2.9.4 to 2.10.0 (#7684)
- Bump google.golang.org/grpc from 1.43.0 to 1.44.0 (#7693)
- Bump github.com/golangci/golangci-lint from 1.43.0 to 1.44.0 (#7692)
- Bump github.com/golangci/golangci-lint (#7696)
- Bump google.golang.org/grpc from 1.43.0 to 1.44.0 (#7695)
- Bump github.com/prometheus/client_golang (#7731)
- Bump docker/build-push-action from 2.8.0 to 2.9.0 (#397)
- Bump docker/build-push-action from 2.8.0 to 2.9.0 (#7780)
- Bump github.com/gorilla/websocket from 1.4.2 to 1.5.0 (#7830)
- Bump github.com/gorilla/websocket from 1.4.2 to 1.5.0 (#7829)
- Bump docker/build-push-action from 2.7.0 to 2.9.0
- Bump github.com/gorilla/websocket from 1.4.2 to 1.5.0
- Bump github.com/rs/zerolog from 1.26.0 to 1.26.1
- Bump actions/github-script from 5 to 6
- Bump docker/login-action from 1.10.0 to 1.12.0
- Bump url-parse from 1.5.4 to 1.5.7 in /docs (#7855)
- Bump github.com/golangci/golangci-lint from 1.44.0 to 1.44.2 (#7854)
- Bump github.com/golangci/golangci-lint (#7853)
- Bump docker/login-action from 1.12.0 to 1.13.0
- Bump docker/login-action from 1.12.0 to 1.13.0 (#7890)
- Bump prismjs from 1.26.0 to 1.27.0 in /docs (#8022)
- Bump url-parse from 1.5.7 to 1.5.10 in /docs (#8023)
- Bump docker/login-action from 1.13.0 to 1.14.1
- Bump golangci/golangci-lint-action from 2.5.2 to 3.1.0
- Bump google.golang.org/grpc from 1.44.0 to 1.45.0 (#8104)
- Bump github.com/spf13/cobra from 1.3.0 to 1.4.0 (#8109)
- Bump github.com/golangci/golangci-lint from 1.44.2 to 1.45.0 (#8169)

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
- Bump github.com/lib/pq from 1.10.3 to 1.10.4 (#7261)
- Bump github.com/tendermint/tm-db from 0.6.4 to 0.6.6 (#7287)
- Bump actions/cache from 2.1.6 to 2.1.7 (#7334)
- Bump watchpack from 2.2.0 to 2.3.0 in /docs (#7335)
- Bump github.com/adlio/schema from 1.1.14 to 1.1.15 (#7407)
- Bump github.com/adlio/schema from 1.2.2 to 1.2.3 (#7432)
- Bump github.com/spf13/viper from 1.9.0 to 1.10.0 (#7434)
- Bump github.com/spf13/cobra from 1.2.1 to 1.3.0 (#7456)
- Bump github.com/spf13/viper from 1.10.0 to 1.10.1 (#7470)

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
- Bump google.golang.org/grpc from 1.41.0 to 1.42.0 (#7200)
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
- Bump google.golang.org/grpc from 1.42.0 to 1.43.0 (#7455)
- Bump github.com/spf13/cobra from 1.2.1 to 1.3.0 (#7457)
- Bump github.com/rs/zerolog from 1.26.0 to 1.26.1 (#7467)
- Bump github.com/rs/zerolog from 1.26.0 to 1.26.1 (#7469)
- Bump github.com/spf13/viper from 1.10.0 to 1.10.1 (#7468)
- Bump docker/login-action from 1.10.0 to 1.11.0 (#378)
- Bump github.com/rs/cors from 1.8.0 to 1.8.2
- Bump docker/login-action from 1.11.0 to 1.12.0 (#380)
- Bump github.com/rs/cors from 1.8.0 to 1.8.2 (#7484)
- Bump docker/login-action from 1.10.0 to 1.12.0 (#7494)
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

- Move mergify config
- Move codecov.yml into .github
- Move codecov config into .github
- Fix fuzz-nightly job (#5965)
- Archive crashers and fix set-crashers-count step (#5992)
- Rename crashers output (fuzz-nightly-test) (#5993)
- Clean up PR template (#6050)
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
- Make core team codeowners (#6383)
- Remove tessr and bez from codeowners (#7028)

### .github/codeowners

- Add alexanderbez (#5913)

### .github/workflows

- Enable manual dispatch for some workflows (#5929)
- Try different e2e nightly test set (#6036)
- Separate e2e workflows for 0.34.x and master (#6041)
- Fix whitespace in e2e config file (#6043)
- Cleanup yaml for e2e nightlies (#6049)

### .gitignore

- Sort (#5690)

### .golangci

- Set locale to US for misspell linter (#6038)

### .goreleaser

- Add windows, remove arm (32 bit) (#5692)

### .vscode

- Remove directory (#5626)

### ABCI

- Update readme to fix broken link to proto (#5847)
- Fix ReCheckTx for Socket Client (#6124)

### ADR-062

- Update with new P2P core implementation (#6051)

### Bug Fixes

- Make p2p evidence_pending test not timing dependent (#6252)
- Avoid race with a deeper copy (#6285)
- Jsonrpc url parsing and dial function (#6264)
- Theoretical leak in clisit.Init (#6302)
- Test fixture peer manager in mempool reactor tests (#6308)
- Benchmark single operation in parallel benchmark not b.N (#6422)
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

### CHANGELOG

- Update to reflect v0.34.0-rc6 (#5622)
- Add breaking Version name change (#5628)

### CHANGELOG_PENDING

- Update changelog for changes to American spelling (#6100)

### CODEOWNERS

- Remove erikgrinaker (#6057)

### CONTRIBUTING

- Update to match the release flow used for 0.34.0 (#5697)

### CONTRIBUTING.md

- Update testing section (#5979)

### Core

- Move validation & data structures together (#176)

### Documentation

- Update specs to remove cmn (#77)
- Add sections to abci (#150)
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
- Fix broken redirect links (#5881)
- Update package-lock.json (#5928)
- Change v0.33 version (#5950)
- Dont login when in PR (#5961)
- Release Linux/ARM64 image (#5925)
- Log level docs (#5945)
- Fix typo in state sync example (#5989)
- External address (#6035)
- Reword configuration (#6039)
- Fix proto file names (#6112)
- How to add tm version to RPC (#6151)
- Add preallocated list of security vulnerability names (#6167)
- Fix sample code (#6186)
- Bump vuepress-theme-cosmos (#6344)
- Remove RFC section and s/RFC001/ADR066 (#6345)
- Adr-65 adjustments (#6401)
- Adr cleanup (#6489)
- Hide security page (second attempt) (#6511)
- Rename tendermint-core to system (#6515)
- Logger updates (#6545)
- Update events (#6658)
- Add sentence about windows support (#6655)
- Add docs file for the peer exchange (#6665)
- Fix broken links (#6719)
- Fix typo (#6789)
- Fix a typo in the genesis_chunked description (#6792)
- Upgrade documentation for custom mempools (#6794)
- Fix typos in /tx_search and /tx. (#6823)
- Add package godoc for indexer (#6839)
- Remove return code in normal case from go built-in example (#6841)
- Fix a typo in the indexing section (#6909)
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

### Makefile

- Use git 2.20-compatible branch detection (#5778)
- Always pull image in proto-gen-docker. (#5953)

### Miscellaneous Tasks

- Bump version to 0.6.0 (#185)

### P2P

- Evidence Reactor Test Refactor (#6238)

### README

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
- Mark grpc as deprecated (#6725)

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

### Security

- Bump github.com/golang/protobuf from 1.4.2 to 1.4.3 (#5506)
- Bump github.com/prometheus/client_golang from 1.7.1 to 1.8.0 (#5515)
- Bump vuepress-theme-cosmos from 1.0.176 to 1.0.177 in /docs (#5746)
- Bump vuepress-theme-cosmos from 1.0.177 to 1.0.178 in /docs (#5754)
- Bump github.com/prometheus/client_golang from 1.8.0 to 1.9.0 (#5807)
- Bump github.com/cosmos/iavl from 0.15.2 to 0.15.3 (#5814)
- Bump github.com/stretchr/testify from 1.6.1 to 1.7.0 (#5897)
- Bump vuepress-theme-cosmos from 1.0.179 to 1.0.180 in /docs (#5915)
- Bump watchpack from 2.1.0 to 2.1.1 in /docs (#6063)
- Bump github.com/tendermint/tm-db from 0.6.3 to 0.6.4 (#6073)
- Bump vuepress-theme-cosmos from 1.0.180 to 1.0.181 in /docs (#6266)
- Bump github.com/minio/highwayhash from 1.0.1 to 1.0.2 (#6280)
- Bump github.com/confio/ics23/go from 0.6.3 to 0.6.6 (#6374)
- Bump github.com/grpc-ecosystem/go-grpc-middleware from 1.2.2 to 1.3.0 (#6387)
- Bump github.com/lib/pq from 1.10.1 to 1.10.2 (#6505)
- Bump github.com/spf13/viper from 1.8.0 to 1.8.1 (#6622)
- Bump github.com/rs/cors from 1.7.0 to 1.8.0 (#6635)
- Bump github.com/spf13/cobra from 1.2.0 to 1.2.1 (#6650)
- Bump github.com/rs/zerolog from 1.24.0 to 1.25.0 (#6923)

### Testing

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
- Fix handling of start height in generated E2E testnets (#5563)
- Disable E2E misbehaviors due to bugs (#5569)
- Fix various E2E test issues (#5576)
- Fix secp failures (#5649)
- Enable v1 and v2 blockchains (#5702)
- Fix TestByzantinePrevoteEquivocation flake (#5710)
- Disable abci/grpc and blockchain/v2 due to flake (#5854)
- Add conceptual overview (#5857)
- Improve WaitGroup handling in Byzantine tests (#5861)
- Tolerate up to 2/3 missed signatures for a validator (#5878)
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
- Add current fuzzing to oss-fuzz-build script (#6576)
- Fix wrong compile fuzzer command (#6579)
- Fix wrong path for some p2p fuzzing packages (#6580)
- Fix non-deterministic backfill test (#6648)
- Add test to reproduce found fuzz errors (#6757)
- Add mechanism to reproduce found fuzz errors (#6768)
- Install abci-cli when running make tests_integrations (#6834)
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

### UX

- Version configuration (#5740)

### Abci

- Add basic description of ABCI Commit.ResponseHeight (#85)
- Add MaxAgeNumBlocks/MaxAgeDuration to EvidenceParams (#87)
- Update MaxAgeNumBlocks & MaxAgeDuration docs (#88)
- Add AppVersion to ConsensusParams (#106)
- Tweak node sync estimate (#115)
- Add ResponseInitChain.app_hash (#140)
- Remove setOption (#5447)
- Lastcommitinfo.round extra sentence (#221)
- Add abci_version to requestInfo (#223)
- Modify Client interface and socket client (#5673)
- Use protoio for length delimitation (#5818)
- Rewrite to proto interface (#237)
- Note on concurrency (#258)
- Change client to use multi-reader mutexes (#6306)
- Reorder sidebar (#282)
- Fix gitignore abci-cli (#6668)
- Remove counter app (#6684)
- Add changelog entry for mempool_error field (#6770)
- Clarify what abci stands for (#336)
- Clarify connection use in-process (#337)
- Change client to use multi-reader mutexes (backport #6306) (#6873)
- Flush socket requests and responses immediately. (#6997)
- Change client to use multi-reader mutexes (backport #6306) (#6873)

### Abci/grpc

- Return async responses in order (#5520)
- Fix ordering of sync/async callback combinations (#5556)
- Fix invalid mutex handling in StopForError() (#5849)

### Add

- Update e2e doc

### Adr

- Privval gRPC (#5712)
- Batch verification (#6008)
- ADR 065: Custom Event Indexing (#6307)
- Node initialization (#6562)

### Backports

- Mergify (#6107)

### Block

- Use commit sig size instead of vote size (#5490)
- Fix max commit sig size (#5567)

### Blockchain

- Change validator set sorting method (#91)
- Rename to core (#123)
- Remove duplicate evidence sections (#124)
- Remove duplication of validate basic (#5418)
- Error on v2 selection (#6730)
- Rename to blocksync service (#6755)

### Blockchain/v0

- Relax termination conditions and increase sync timeout (#5741)
- Stop tickers on poolRoutine exit (#5860)
- Fix data race in blockchain channel (#6518)

### Blockchain/v1

- Add noBlockResponse handling  (#5401)
- Handle peers without blocks (#5701)
- Fix deadlock (#5711)
- Remove in favor of v2 (#5728)

### Blockchain/v2

- Fix "panic: duplicate block enqueued by processor" (#5499)
- Fix panic: processed height X+1 but expected height X (#5530)
- Make the removal of an already removed peer a noop (#5553)
- Remove peers from the processor  (#5607)
- Send status request when new peer joins (#5774)
- Fix missing mutex unlock (#5862)
- Internalize behavior package (#6094)

### Blockstore

- Save only the last seen commit (#6212)
- Fix problem with seen commit (#6782)

### Blocksync

- Complete transition from Blockchain to BlockSync (#6847)
- Fix shutdown deadlock issue (#7030)

### Blocksync/v2

- Remove unsupported reactor (#7046)

### Buf

- Modify buf.yml, add buf generate (#5653)

### Build

- Bump gaurav-nelson/github-action-markdown-link-check from 0.6.0 to 1.0.7 (#149)
- Bump watchpack from 1.7.4 to 2.0.0 in /docs (#5470)
- Bump actions/cache from v2.1.1 to v2.1.2 (#5487)
- Bump golangci/golangci-lint-action from v2.2.0 to v2.2.1 (#5486)
- Bump technote-space/get-diff-action from v3 to v4 (#5485)
- Bump github.com/spf13/cobra from 1.0.0 to 1.1.0 (#5505)
- Bump github.com/spf13/cobra from 1.1.0 to 1.1.1 (#5526)
- Bump codecov/codecov-action from v1.0.13 to v1.0.14 (#5525)
- Bump gaurav-nelson/github-action-markdown-link-check from 1.0.7 to 1.0.8 (#5543)
- Bump gaurav-nelson/github-action-markdown-link-check from 1.0.7 to 1.0.8 (#188)
- Bump google.golang.org/grpc from 1.32.0 to 1.33.1 (#5544)
- Bump golangci/golangci-lint-action from v2.2.1 to v2.3.0 (#5571)
- Bump codecov/codecov-action from v1.0.13 to v1.0.14 (#5582)
- Bump watchpack from 2.0.0 to 2.0.1 in /docs (#5605)
- Bump actions/cache from v2.1.2 to v2.1.3 (#5633)
- Bump google.golang.org/grpc from 1.33.1 to 1.33.2 (#5635)
- Bump github.com/tendermint/tm-db from 0.6.2 to 0.6.3
- Bump rtCamp/action-slack-notify from e9db0ef to 2.1.1
- Bump codecov/codecov-action from v1.0.14 to v1.0.15 (#5676)
- Bump github.com/cosmos/iavl from 0.15.0-rc5 to 0.15.0 (#5708)
- Bump vuepress-theme-cosmos from 1.0.175 to 1.0.176 in /docs (#5727)
- Bump google.golang.org/grpc from 1.33.2 to 1.34.0 (#5737)
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
- Bump google.golang.org/grpc from 1.34.0 to 1.35.0 (#5902)
- Bump actions/cache from v2.1.3 to v2.1.4 (#6055)
- Bump JamesIves/github-pages-deploy-action (#6062)
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
- Bump google.golang.org/grpc from 1.36.1 to 1.37.0 (#6330)
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
- Bump google.golang.org/grpc from 1.37.0 to 1.37.1 (#6461)
- Bump actions/stale from 3.0.18 to 3.0.19 (#6477)
- Bump actions/stale from 3 to 3.0.18 (#300)
- Bump watchpack from 2.1.1 to 2.2.0 in /docs (#6482)
- Bump actions/stale from 3.0.18 to 3.0.19 (#302)
- Bump browserslist from 4.16.4 to 4.16.6 in /docs (#6487)
- Bump google.golang.org/grpc from 1.37.1 to 1.38.0 (#6483)
- Bump docker/build-push-action from 2.4.0 to 2.5.0 (#6496)
- Bump dns-packet from 1.3.1 to 1.3.4 in /docs (#6500)
- Bump actions/cache from 2.1.5 to 2.1.6 (#6504)
- Bump rtCamp/action-slack-notify from 2.1.3 to 2.2.0 (#6543)
- Bump github.com/prometheus/client_golang (#6552)
- Bump github.com/btcsuite/btcd (#6560)
- Bump codecov/codecov-action from 1.5.0 to 1.5.2 (#6559)
- Bump github.com/rs/zerolog from 1.22.0 to 1.23.0 (#6575)
- Bump github.com/spf13/viper from 1.7.1 to 1.8.0 (#6586)
- Bump docker/login-action from 1.9.0 to 1.10.0 (#6614)
- Bump docker/setup-buildx-action from 1.3.0 to 1.4.0 (#6629)
- Bump docker/setup-buildx-action from 1.4.0 to 1.4.1 (#6632)
- Bump google.golang.org/grpc from 1.38.0 to 1.39.0 (#6633)
- Bump github.com/spf13/cobra from 1.1.3 to 1.2.0 (#6640)
- Bump docker/build-push-action from 2.5.0 to 2.6.1 (#6639)
- Bump docker/setup-buildx-action from 1.4.1 to 1.5.0 (#6649)
- Bump github.com/go-kit/kit from 0.10.0 to 0.11.0 (#6651)
- Bump gaurav-nelson/github-action-markdown-link-check (#6679)
- Bump github.com/golangci/golangci-lint (#6686)
- Bump gaurav-nelson/github-action-markdown-link-check (#313)
- Bump github.com/google/uuid from 1.2.0 to 1.3.0 (#6708)
- Bump actions/stale from 3.0.19 to 4 (#319)
- Bump actions/stale from 3.0.19 to 4 (#6726)
- Bump codecov/codecov-action from 1.5.2 to 2.0.1 (#6739)
- Bump codecov/codecov-action from 2.0.1 to 2.0.2 (#6764)
- Bump styfle/cancel-workflow-action from 0.9.0 to 0.9.1 (#6786)
- Bump technote-space/get-diff-action from 4 to 5 (#6788)
- Bump github.com/BurntSushi/toml from 0.3.1 to 0.4.1 (#6796)
- Bump google.golang.org/grpc from 1.39.0 to 1.39.1 (#6801)
- Bump google.golang.org/grpc from 1.39.1 to 1.40.0 (#6819)
- Bump github.com/golangci/golangci-lint (#6837)
- Bump docker/build-push-action from 2.6.1 to 2.7.0 (#6845)
- Bump codecov/codecov-action from 2.0.2 to 2.0.3 (#6860)
- Bump github.com/rs/zerolog from 1.23.0 to 1.24.0 (#6874)
- Bump github.com/lib/pq from 1.10.2 to 1.10.3 (#6890)
- Bump docker/setup-buildx-action from 1.5.0 to 1.6.0 (#6903)
- Bump github.com/golangci/golangci-lint (#6907)
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

### Bytes

- Clean up and simplify encoding of HexBytes (#6810)

### Changelog

- Add missing date to v0.33.5 release, fix indentation (#5454)
- Squash changelog from 0.34 RCs into one (#5691)
- Add entry back (#5738)
- Update with changes released in 0.34.1 (#5875)
- Update changelogs to reflect changes released in 0.34.2
- Update changelog for v0.34.3 (#5927)
- Update to reflect v0.34.4 release (#6105)
- Update 0.34.3 changelog with details on security vuln (#6108)
- Update with changes from 0.34.7 (and failed 0.34.5, 0.34.6) (#6150)
- Update for 0.34.8 (#6183)
- Update to reflect 0.34.9 (#6334)
- Update for 0.34.10 (#6358)
- Have a single friendly bug bounty reminder (#6600)
- Update and regularize changelog entries (#6594)
- Prepare for v0.34.12 (#6831)
- Update to reflect 0.34.12 release (#6833)
- Linkify the 0.34.11 release notes (#6836)
- Add entry for interanlizations (#6989)
- Add 0.34.14 updates (#7117)

### Changelog_pending

- Add missing item (#6829)
- Add missing entry (#6830)

### Ci

- Add markdown linter (#146)
- Add dependabot config (#148)
- Docker remvoe circleci and add github action (#5420)
- Add goreleaser (#5527)
- Tests (#5577)
- Use gh pages (#5609)
- Remove `add-path` (#5674)
- Remove circle (#5714)
- Build for 32 bit, libs: fix overflow (#5700)
- Make timeout-minutes 8 for golangci (#5821)
- Run `goreleaser build` (#5824)
- Add janitor (#6292)
- Drop codecov bot (#6917)
- Tweak code coverage settings (#6920)
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

### Ci/e2e

- Avoid running job when no go files are touched (#5471)

### Circleci

- Remove Gitian reproducible_builds job (#5462)

### Cleanup

- Remove redundant error plumbing (#6778)
- Fix order of linters in the golangci-lint config (#6910)
- Reduce and normalize import path aliasing. (#6975)
- Remove not needed binary test/app/grpc_client

### Cli

- Light home dir should default to where the full node default is (#5392)
- Allow node operator to rollback last state (#7033)
- Allow node operator to rollback last state (backport #7033) (#7081)

### Cli/indexer

- Reindex events (#6676)

### Clist

- Add a few basic clist tests (#6727)
- Add simple property tests (#6791)

### Cmd

- Add support for --key (#5612)
- Modify `gen_node_key` to print key to STDOUT (#5772)
- Hyphen case cli and config (#5777)
- Ignore missing wal in debug kill command (#6160)
- Remove deprecated snakes (#6854)

### Cmd/tendermint/commands

- Replace $HOME/.some/test/dir with t.TempDir (#6623)

### Codecov

- Disable annotations (#5413)
- Validate codecov.yml (#5699)

### Codeowners

- Add code owners (#82)

### Commands

- Add key migration cli (#6790)

### Config

- Set statesync.rpc_servers when generating config file (#5433)
- Increase MaxPacketMsgPayloadSize to 1400
- Fix mispellings (#5914)
- Create `BootstrapPeers` p2p config parameter (#6372)
- Add private peer id /net_info expose information in default config (#6490)
- Seperate priv validator config into seperate section (#6462)
- Add root dir to priv validator (#6585)
- Add example on external_address (#6621)
- Add example on external_address (backport #6621) (#6624)
- Add example on external_address (backport #6621) (#6624)

### Config/docs

- Update and deprecated (#6879)

### Config/indexer

- Custom event indexing (#6411)

### Consensus

- Check block parts don't exceed maximum block bytes (#5431)
- Open target WAL as read/write during autorepair (#5536)
- Fix flaky tests (#5734)
- Change log level to error when adding vote
- Deprecate time iota ms (#5792)
- Groom Logs (#5917)
- P2p refactor (#5969)
- More log grooming (#6140)
- Log private validator address and not struct (#6144)
- Reduce shared state in tests (#6313)
- Add test vector for hasvote (#6469)
- Skip all messages during sync (#6577)
- Avoid unbuffered channel in state test (#7025)
- Wait until peerUpdates channel is closed to close remaining peers (#7058)
- Wait until peerUpdates channel is closed to close remaining peers (#7058) (#7060)

### Contributing

- Include instructions for a release candidate (#5498)
- Simplify our minor release process (#5749)
- Update release instructions to use backport branches (#6827)
- Remove release_notes.md reference (#6846)

### Core

- Update a few sections  (#284)
- Text cleanup (#332)

### Crypto

- Add in secp256k1 support (#5500)
- Adopt zip215 ed25519 verification (#5632)
- Fix infinite recursion in Secp256k1 string formatting (#5707)
- Ed25519 & sr25519 batch verification (#6120)
- Add sr25519 as a validator key (#6376)
- Use a different library for ed25519/sr25519 (#6526)

### Crypto/armor

- Remove unused package (#6963)

### Crypto/merkle

- Pre-allocate data slice in innherHash (#6443)
- Optimize merkle tree hashing (#6513)

### Db

- Migration script for key format change (#6355)

### Dep

- Bump ed25519consensus version (#5760)
- Remove IAVL dependency (#6550)

### Deps

- Remove pkg errors (#6666)
- Run go mod tidy (#6677)

### E2e

- Use ed25519 for secretConn (remote signer) (#5678)
- Releases nightly (#5906)
- Add control over the log level of nodes (#5958)
- Disconnect maverick (#6099)
- Adjust timeouts to be dynamic to size of network (#6202)
- Add benchmarking functionality (#6210)
- Integrate light clients (#6196)
- Fix light client generator (#6236)
- Fix perturbation of seed nodes (#6272)
- Add evidence generation and testing (#6276)
- Tx load to use broadcast sync instead of commit (#6347)
- Split out nightly tests (#6395)
- Prevent non-viable testnets (#6486)
- Fix looping problem while waiting (#6568)
- Allow variable tx size  (#6659)
- Disable app tests for light client (#6672)
- Remove colorized output from docker-compose (#6670)
- Extend timeouts in test harness (#6694)
- Ensure evidence validator set matches nodes validator set (#6712)
- Tweak sleep for pertubations (#6723)
- Avoid systematic key-type variation (#6736)
- Drop single node hybrid configurations (#6737)
- Remove cartesian testing of ipv6 (#6734)
- Run tests in fewer groups (#6742)
- Prevent adding light clients as persistent peers (#6743)
- Longer test harness timeouts (#6728)
- Allow for both v0 and v1 mempool implementations (#6752)
- Avoid starting nodes from the future (#6835)
- Avoid starting nodes from the future (#6835) (#6838)
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

### Encoding

- Add secp, ref zip215, tables (#212)

### Events

- Add block_id to NewBlockEvent (#6478)

### Evidence

- Add time to evidence params (#69)
- Update data structures (#165)
- Use bytes instead of quantity to limit size (#5449)
- Don't gossip consensus evidence too soon (#5528)
- Don't send committed evidence and ignore inbound evidence that is already committed (#5574)
- Structs can independently form abci evidence (#5610)
- Update data structures to reflect added support of abci evidence (#213)
- Omit bytes field (#5745)
- P2p refactor (#5747)
- Buffer evidence from consensus (#5890)
- Fix bug with hashes (#6375)
- Separate abci specific validation (#6473)
- Update ADR 59 and add comments to the use of common height (#6628)
- Add section explaining evidence (#324)

### Fastsync

- Update the metrics during fast-sync (#6590)

### Fastsync/event

- Emit fastsync status event when switching consensus/fastsync (#6619)

### Fastsync/rpc

- Add TotalSyncedTime & RemainingTime to SyncInfo in /status RPC (#6620)

### Fuzz

- Initial support for fuzzing (#6558)

### Genesis

- Explain fields in genesis file (#270)

### Github

- Rename e2e jobs (#5502)
- Add nightly E2E testnet action (#5480)
- Only notify nightly E2E failures once (#5559)
- Issue template for proposals (#190)
- Add @tychoish to code owners (#6273)
- Fix linter configuration errors and occluded errors (#6400)

### Go.mod

- Upgrade iavl and deps (#5657)

### Goreleaser

- Lowercase binary name (#5765)
- Downcase archive and binary names (#6029)

### Improvement

- Update TxInfo (#6529)

### Indexer

- Remove info log (#6194)
- Use INSERT ... ON CONFLICT in the psql eventsink insert functions (#6556)

### Inspect

- Add inspect mode for debugging crashed tendermint node (#6785)
- Remove duplicated construction path (#6966)

### Internal

- Update blockchain reactor godoc (#6749)

### Internal/blockchain/v0

- Prevent all possible race for blockchainCh.Out (#6637)

### Internal/consensus

- Update error log (#6863)
- Update error log (#6863) (#6867)
- Update error log (#6863) (#6867)

### Internal/proxy

- Add initial set of abci metrics (#7115)

### Layout

- Add section titles (#240)

### Libs

- Remove most of libs/rand (#6364)
- Internalize some packages (#6366)

### Libs/CList

- Automatically detach the prev/next elements in Remove function (#6626)

### Libs/bits

- Validate BitArray in FromProto (#5720)

### Libs/clist

- Fix flaky tests (#6453)
- Revert clear and detach changes while debugging (#6731)

### Libs/log

- Format []byte as hexidecimal string (uppercased) (#5960)
- [JSON format] include timestamp (#6174)
- Use fmt.Fprintf directly with *bytes.Buffer to avoid unnecessary allocations (#6503)
- Text logging format changes (#6589)

### Libs/os

- Add test case for TrapSignal (#5646)
- Remove unused aliases, add test cases (#5654)
- EnsureDir now returns IO errors and checks file type (#5852)
- Avoid CopyFile truncating destination before checking if regular file (#6428)

### Libs/time

- Move types/time into libs (#6595)

### Light

- Expand on errors and docs (#5443)
- Cross-check the very first header (#5429)
- Model-based tests (#5461)
- Run detector for sequentially validating light client (#5538)
- Make fraction parts uint64, ensuring that it is always positive (#5655)
- Ensure required header fields are present for verification (#5677)
- Minor fixes / standardising errors (#5716)
- Fix light store deadlock (#5901)
- Fix panic with RPC calls to commit and validator when height is nil (#6026)
- Remove max retry attempts from client and add to provider (#6054)
- Create provider options struct (#6064)
- Improve timeout functionality (#6145)
- Improve provider handling (#6053)
- Handle too high errors correctly (#6346)
- Ensure trust level is strictly less than 1 (#6447)
- Spec alignment on verify skipping (#6474)
- Correctly handle contexts (backport -> v0.34.x) (#6685)
- Correctly handle contexts (#6687)
- Add case to catch cancelled contexts within the detector (backport #6701) (#6720)
- Run examples as integration tests (#6745)
- Improve error handling and allow providers to be added (#6733)
- Wait for tendermint node to start before running example test (#6744)
- Replace homegrown mock with mockery (#6735)
- Fix early erroring (#6905)
- Update initialization description (#320)
- Update links in package docs. (#7099)
- Update links in package docs. (#7099) (#7101)
- Fix early erroring (#6905)

### Light/provider/http

- Fix Validators (#6022)

### Light/rpc

- Fix ABCIQuery (#5375)

### Lint

- Fix lint errors (#301)
- Change deprecated linter (#6861)
- Fix collection of stale errors (#7090)

### Linter

- Fix nolintlint warnings (#6257)
- Linter checks non-ASCII identifiers (#6574)

### Localnet

- Fix node starting issue with --proxy-app flag (#5803)
- Expose 6060 (pprof) and 9090 (prometheus) on node0
- Use 27000 port for prometheus (#5811)
- Fix localnet by excluding self from persistent peers list (#6209)

### Logger

- Refactor Tendermint logger by using zerolog (#6534)

### Logging

- Print string instead of callback (#6177)
- Shorten precommit log message (#6270)

### Logs

- Cleanup (#6198)

### Maverick

- Reduce some duplication (#6052)

### Mempool

- Fix nil pointer dereference (#5412)
- Length prefix txs when getting them from mempool (#5483)
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
- Move errors to be public (#6613)
- Add TTL configuration to mempool (#6715)
- Return mempool errors to the abci client (#6740)

### Mempool,rpc

- Add removetx rpc method (#7047)
- Add removetx rpc method (#7047) (#7065)

### Mempool/rpc

- Log grooming (#6201)

### Mempool/v1

- Test reactor does not panic on broadcast (#6772)

### Metrics

- Change blocksize to a histogram (#6549)

### Network

- Update terraform config (#6901)

### Networks

- Update to latest DigitalOcean modules (#6902)

### Node

- Improve test coverage on proposal block (#5748)
- Feature flag for legacy p2p support (#6056)
- Implement tendermint modes (#6241)
- Remove mode defaults. Make node mode explicit (#6282)
- Use db provider instead of mem db (#6362)
- Cleanup pex initialization (#6467)
- Change package interface (#6540)
- Fix genesis on start up (#6563)
- Minimize hardcoded service initialization (#6798)
- Always close database engine (#7113)

### Node/state

- Graceful shutdown in the consensus state (#6370)

### Node/tests

- Clean up use of genesis doc and surrounding tests (#6554)

### Note

- Add nondeterministic note to events (#6220)

### Os

- Simplify EnsureDir() (#5871)

### P2p

- Merlin based malleability fixes (#72)
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
- Connect max inbound peers configuration to new router (#6296)
- Filter peers by IP address and ID (#6300)
- Improve router test stability (#6310)
- Extend e2e tests for new p2p framework (#6323)
- Make peer scoring test more resilient (#6322)
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
- Increase queue size to 16MB (#6588)
- Avoid retry delay in error case (#6591)
- Address audit issues with the peer manager (#6603)
- Make NodeID and NetAddress public (#6583)
- Reduce buffering on channels (#6609)
- Do not redial peers with different chain id (#6630)
- Track peer channels to avoid sending across a channel a peer doesn't have (#6601)
- Remove annoying error log (#6688)
- Add coverage for mConnConnection.TrySendMessage (#6754)
- Avoid blocking on the dequeCh (#6765)
- Add test for pqueue dequeue full error (#6760)
- Change default to use new stack (#6862)
- Delete legacy stack initial pass (#7035)
- Remove wdrr queue (#7064)
- Cleanup transport interface (#7071)
- Cleanup unused arguments (#7079)
- Rename pexV2 to pex (#7088)
- Fix priority queue bytes pending calculation (#7120)

### P2p/conn

- Check for channel id overflow before processing receive msg (#6522)

### P2p/pex

- Fix flaky tests (#5733)
- Cleanup to pex internals and peerManager interface (#6476)
- Reuse hash.Hasher per addrbook for speed (#6509)

### Params

- Remove block timeiota (#248)
- Remove blockTimeIota (#5987)

### Pex

- Fix send requests too often test (#6437)
- Update pex messages (#352)

### Pkg

- Expose p2p functions (#6627)

### Privval

- Allow passing options to NewSignerDialerEndpoint (#5434)
- Fix ping message encoding (#5441)
- Make response values non nullable (#5583)
- Increase read/write timeout to 5s and calculate ping interval based on it (#5638)
- Reset pingTimer to avoid sending unnecessary pings (#5642)
- Duplicate SecretConnection from p2p package (#5672)
- Add grpc (#5725)
- Query validator key (#5876)
- Return errors on loadFilePV (#6185)
- Add ctx to privval interface (#6240)
- Missing privval type check in SetPrivValidator (#6645)

### Proto

- Buf for everything (#5650)
- Bump gogoproto (1.3.2) (#5886)
- Docker deployment (#5931)
- Seperate native and proto types (#5994)
- Add files (#246)
- Modify height int64 to uint64 (#253)
- Move proto files under the correct directory related to their package name (#344)
- Add tendermint go changes (#349)
- Regenerate code (#6977)

### Proto/p2p

- Rename PEX messages and fields (#5974)

### Proxy

- Move proxy package to internal (#6953)

### Psql

- Close opened rows in tests (#6669)
- Add documentation and simplify constructor API (#6856)

### Pubsub

- Refactor Event Subscription (#6634)
- Unsubscribe locking handling (#6816)
- Improve handling of closed blocking subsciptions. (#6852)

### Reactors

- Omit incoming message bytes from reactor logs (#5743)
- Remove bcv1 (#241)

### Reactors/pex

- Specify hash function (#94)
- Masked IP is used as group key (#96)

### Readme

- Remover circleci badge (#5729)
- Add links to job post (#5785)
- Update discord link (#5795)
- Add security mailing list (#5916)
- Cleanup (#262)
- Update discord links (#6965)

### Release

- Update changelog and version (#6599)

### Rfc

- P2p next steps (#6866)
- Fix link style (#6870)
- Database storage engine (#6897)
- E2e improvements (#6941)
- Add performance taxonomy rfc (#6921)
- Fix a few typos and formatting glitches p2p roadmap (#6960)
- Event system (#6957)

### Router/statesync

- Add helpful log messages (#6724)

### Rpc

- Fix content-type header (#5661)
- Standardize error codes (#6019)
- Change default sorting to desc for `/tx_search` results (#6168)
- Index block events to support block event queries (#6226)
- Define spec for RPC (#276)
- Remove global environment (#6426)
- Clean up client global state in tests (#6438)
- Add chunked rpc interface (#6445)
- Clarify timestamps (#304)
- Add chunked genesis endpoint (#299)
- Decouple test fixtures from node implementation (#6533)
- Fix RPC client doesn't handle url's without ports (#6507)
- Add subscription id to events (#6386)
- Use shorter path names for tests (#6602)
- Add totalGasUSed to block_results response (#308)
- Add max peer block height into /status rpc call (#6610)
- Add `TotalGasUsed` to `block_results` response (#6615)
- Re-index missing events (#6535)
- Add chunked rpc interface (backport #6445) (#6717)
- Add documentation for genesis chunked api (#6776)
- Avoid panics in unsafe rpc calls with new p2p stack (#6817)
- Support new p2p infrastructure (#6820)
- Log update (#6825)
- Log update (backport #6825) (#6826)
- Update peer format in specification in NetInfo operation (#331)
- Fix hash encoding in JSON parameters (#6813)
- Strip down the base RPC client interface. (#6971)
- Implement BroadcastTxCommit without event subscriptions (#6984)
- Add chunked rpc interface (backport #6445) (#6717)
- Move evidence tests to shared fixtures (#7119)
- Remove the deprecated gRPC interface to the RPC service (#7121)
- Fix typo in broadcast commit (#7124)

### Rpc/client/http

- Do not drop events even if the `out` channel is full (#6163)
- Drop endpoint arg from New and add WSOptions (#6176)

### Rpc/core

- More docs and a test for /blockchain endpoint (#5417)

### Rpc/jsonrpc

- Unmarshal RPCRequest correctly (#6191)

### Rpc/jsonrpc/server

- Return an error in WriteRPCResponseHTTP(Error) (#6204)

### Scripts

- Move build.sh into scripts
- Make linkifier default to 'pull' rather than 'issue' (#5689)
- Fix authors script to take a ref (#7051)

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

- More test cases for block validation (#5415)
- Prune states using an iterator (#5864)
- Save in batches within the state store (#6067)
- Cleanup block indexing logs and null (#6263)
- Fix block event indexing reserved key check (#6314)
- Keep a cache of block verification results (#6402)
- Move pruneBlocks from consensus/state to state/execution (#6541)
- Move package to internal (#6964)

### State/indexer

- Reconstruct indexer, move txindex into the indexer package (#6382)
- Close row after query (#6664)

### State/privval

- No GetPubKey retry beyond the proposal/voting window (#6578)
- Vote timestamp fix (#6748)
- Vote timestamp fix (backport #6748) (#6783)

### State/types

- Refactor makeBlock, makeBlocks and makeTxs (#6567)

### Statesync

- Check all necessary heights when adding snapshot to pool (#5516)
- Do not recover panic on peer updates (#5869)
- Improve e2e test outcomes (#6378)
- Sort snapshots by commonness (#6385)
- Fix unreliable test (#6390)
- Ranking test fix (#6415)
- Tune backfill process (#6565)
- Make fetching chunks more robust (#6587)
- Keep peer despite lightblock query fail (#6692)
- Remove outgoingCalls race condition in dispatcher (#6699)
- Use initial height as a floor to backfilling (#6709)
- Increase dispatcher timeout (#6714)
- Dispatcher test uses internal channel for timing (#6713)
- New messages for gossiping consensus params (#328)
- Improve stateprovider handling in the syncer (backport) (#6881)
- Implement p2p state provider (#6807)
- Shut down node when statesync fails (#6944)
- Clean up reactor/syncer lifecylce (#6995)
- Add logging while waiting for peers (#7007)
- Ensure test network properly configured (#7026)
- Remove deadlock on init fail (#7029)
- Improve rare p2p race condition (#7042)
- Improve stateprovider handling in the syncer (backport) (#6881)

### Statesync/event

- Emit statesync start/end event  (#6700)

### Statesync/rpc

- Metrics for the statesync and the rpc SyncInfo (#6795)

### Store

- Order-preserving varint key encoding (#5771)
- Use db iterators for pruning and range-based queries (#5848)
- Fix deadlock in pruning (#6007)
- Use a batch instead of individual writes in SaveBlock (#6018)
- Move pacakge to internal (#6978)

### Sync

- Move closer to separate file (#6015)

### Time

- Make median time library type private (#6853)

### Tooling

- Remove tools/Makefile (#6102)
- Use go version 1.16 as minimum version (#6642)

### Tools

- Use os home dir to instead of the hardcoded PATH (#6498)
- Remove k8s (#6625)
- Move tools.go to subdir (#6689)
- Add mockery to tools.go and remove mockery version strings (#6787)

### Tools/tm-signer-harness

- Fix listener leak in newTestHarnessListener() (#5850)

### Tx

- Reduce function to one parameter (#5493)

### Types

- Rename json parts to part_set_header (#5523)
- Move `MakeBlock` to block.go (#5573)
- Cleanup protobuf.go (#6023)
- Refactor EventAttribute (#6408)
- Fix verify commit light / trusting bug (#6414)
- Revert breaking change (#6538)
- Move NodeInfo from p2p (#6618)
- Move mempool error for consistency (#6875)

### Upgrading

- Update 0.34 instructions with updates since RC4 (#5685)
- Add information into the UPGRADING.md for users of the codebase wishing to upgrade (#6898)

### Version

- Add abci version to handshake (#5706)
- Revert version through ldflag only (#6494)
- Bump for 0.34.12 (#6832)

### Ws

- Parse remote addrs with trailing dash (#6537)

