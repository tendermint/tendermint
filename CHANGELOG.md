## [0.8.0-dev.2] - 2022-04-22

### Bug Fixes

- Don't disconnect already disconnected validators
- Cannot read properties of undefined

### Documentation

- Go tutorial fixed for 0.35.0 version (#7329) (#7330) (#7331)
- Update go ws code snippets (#7486) (#7487)
- Fixup the builtin tutorial  (#7488)
- Go tutorial fixed for 0.35.0 version (#7329) (#7330) (#7331)
- Update go ws code snippets (#7486) (#7487)

### Features

- Improve logging for better elasticsearch compatibility (#220)
- InitChain can set initial core lock height (#222)
- Add empty block on h-1 and h-2 apphash change (#241)
- Inter-validator set communication (#187)
- Add create_proof_block_range config option (#243)

### Miscellaneous Tasks

- Create only 1 proof block by default
- Release script and initial changelog (#250)
- [**breaking**] Bump ABCI version and update release.sh to change TMVersionDefault automatically (#253)
- Update changelog and version to 0.7.0
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
- If the tenderdash source code is not tracked by git then cloning "develop_0.1" branch as fallback scenario to build a project (#356)

### Refactor

- [**breaking**] Replace is-masternode config with mode=validator (#308)
- Add MustPubKeyToProto helper function (#311)
- Implementing LLMQ generator (#310)
- Move bls CI code to a separate action and improve ARM build (#314)
- Persistent kvstore abci (#313)
- Improve statesync.backfill (#316)
- Small improvement in test four add four minus one genesis validators (#318)

### Testing

- Fix validator conn executor test backport
- Update mockery mocks
- Fix test test_abci_cli

### Backport

- Add basic metrics to the indexer package. (#7250) (#7252)
- Add basic metrics to the indexer package. (#7250) (#7252)

### Build

- Bump google.golang.org/grpc from 1.41.0 to 1.42.0 (#7218)
- Bump github.com/lib/pq from 1.10.3 to 1.10.4 (#7260)
- Bump github.com/tendermint/tm-db from 0.6.4 to 0.6.6 (#7285)
- Bump github.com/adlio/schema from 1.1.14 to 1.1.15 (#7406)
- Bump github.com/adlio/schema from 1.1.15 to 1.2.2 (#7422)
- Bump github.com/spf13/viper from 1.9.0 to 1.10.0 (#7435)
- Bump github.com/adlio/schema from 1.2.2 to 1.2.3 (#7436)
- Bump google.golang.org/grpc from 1.42.0 to 1.43.0 (#7458)
- Bump github.com/spf13/cobra from 1.2.1 to 1.3.0 (#7457)
- Bump github.com/rs/zerolog from 1.26.0 to 1.26.1 (#7467)
- Bump github.com/spf13/viper from 1.10.0 to 1.10.1 (#7468)
- Bump github.com/rs/cors from 1.8.0 to 1.8.2
- Bump github.com/rs/cors from 1.8.0 to 1.8.2 (#7485)
- Bump github.com/BurntSushi/toml from 0.4.1 to 1.0.0 (#7560)
- Make sure to test packages with external tests (backport #7608) (#7635)
- Bump github.com/prometheus/client_golang (#7637)
- Bump github.com/prometheus/client_golang (#249)
- Bump github.com/BurntSushi/toml from 0.4.1 to 1.0.0
- Bump github.com/vektra/mockery/v2 from 2.9.4 to 2.10.0 (#7684)
- Bump google.golang.org/grpc from 1.43.0 to 1.44.0 (#7693)
- Bump github.com/golangci/golangci-lint (#7696)
- Bump google.golang.org/grpc from 1.41.0 to 1.42.0 (#7218)
- Bump github.com/lib/pq from 1.10.3 to 1.10.4
- Bump github.com/tendermint/tm-db from 0.6.4 to 0.6.6 (#7285)
- Bump github.com/spf13/viper from 1.9.0 to 1.10.0 (#7435)
- Bump github.com/adlio/schema from 1.2.2 to 1.2.3 (#7436)
- Bump github.com/spf13/cobra from 1.2.1 to 1.3.0 (#7457)
- Bump github.com/rs/zerolog from 1.26.0 to 1.26.1 (#7467)
- Downgrade tm-db from v0.6.7 to v0.6.6
- Use Go 1.18 to fix issue building curve25519-voi
- Provide base branch to make as variable (#321)
- Implement full release workflow in the release script (#332)

### Ci

- Move test execution to makefile (#7372) (#7374)
- Cleanup build/test targets (backport #7393) (#7395)
- Skip docker image builds during PRs (#7397) (#7398)
- Move test execution to makefile (#7372) (#7374)
- Update mergify for tenderdash 0.8
- Cleanup build/test targets (backport #7393) (#7395)
- Skip docker image builds during PRs (#7397) (#7398)
- Fixes for arm builds

### Cmd

- Cosmetic changes for errors and print statements (#7377) (#7408)
- Add integration test for rollback functionality (backport #7315) (#7369)
- Cosmetic changes for errors and print statements (#7377) (#7408)
- Add integration test for rollback functionality (backport #7315) (#7369)

### Config

- Add a Deprecation annotation to P2PConfig.Seeds. (#7496) (#7497)
- Add a Deprecation annotation to P2PConfig.Seeds. (#7496) (#7497)

### Consensus

- Add some more checks to vote counting (#7253) (#7262)
- Calculate prevote message delay metric (backport #7551) (#7618)
- Check proposal non-nil in prevote message delay metric (#7625) (#7632)
- Add some more checks to vote counting (#7253) (#7262)

### E2e

- Stabilize validator update form (#7340) (#7351)
- Clarify apphash reporting (#7348) (#7352)
- Generate keys for more stable load (#7344) (#7353)
- App hash test cleanup (0.35 backport) (#7350)
- Limit legacyp2p and statesyncp2p (#7361)
- Use more simple strings for generated transactions (#7513) (#7514)
- Constrain test parallelism and reporting (backport #7516) (#7517)
- Make tx test more stable (backport #7523) (#7527)
- Stabilize validator update form (#7340) (#7351)
- Clarify apphash reporting (#7348) (#7352)
- Generate keys for more stable load (#7344) (#7353)
- App hash test cleanup (0.35 backport) (#7350)

### Evidence

- Remove source of non-determinism from test (#7266) (#7268)
- Remove source of non-determinism from test (#7266) (#7268)

### Internal/libs/protoio

- Optimize MarshalDelimited by plain byteslice allocations+sync.Pool (#7325) (#7426)
- Optimize MarshalDelimited by plain byteslice allocations+sync.Pool (#7325) (#7426)

### Internal/proxy

- Add initial set of abci metrics backport (#7342)
- Add initial set of abci metrics backport (#7342)

### Lint

- Remove lll check (#7346) (#7357)
- Remove lll check (#7346) (#7357)

### P2p

- Reduce peer score for dial failures (backport #7265) (#7271)
- Always advertise self, to enable mutual address discovery (#7620)
- Reduce peer score for dial failures (backport #7265) (#7271)

### Pubsub

- Report a non-nil error when shutting down. (#7310)
- Report a non-nil error when shutting down. (#7310)

### Rpc

- Backport experimental buffer size control parameters from #7230 (tm v0.35.x) (#7276)
- Implement header and header_by_hash queries (backport #7270) (#7367)
- Check error code for broadcast_tx_commit (#7683) (#7688)
- Backport experimental buffer size control parameters from #7230 (tm v0.35.x) (#7276)
- Implement header and header_by_hash queries (backport #7270) (#7367)

### Types

- Fix path handling in node key tests (#7493) (#7502)
- Fix path handling in node key tests (#7493) (#7502)

## [0.8.0-dev.1] - 2022-03-24

### Backport

- Backport of [Tendermint 0.35.0](https://github.com/tendermint/tendermint/releases/tag/v0.35.0)

### Bug Fixes

- Panic on precommits does not have any +2/3 votes
- Improved error handling  in DashCoreSignerClient
- Abci/example, cmd and test packages were fixed after the upstream backport
- Some fixes to be able to compile the add
- Some fixes made by PR feedback
- Use types.DefaultDashVotingPower rather than internal dashDefaultVotingPower
- Detect and fix data-race in MockPV (#262)
- Race condition when logging (#271)
- Decrease memory used by debug logs (#280)
- Tendermint stops when validator node id lookup fails (#279)
- Backport e2e tests (#248)

### Miscellaneous Tasks

- Eliminate compile errors after backport of tendermint 0.35 (#238)
- Update unit tests after backport fo tendermint v0.35 (#245)
- Backport Tenderdash 0.7 to 0.8 (#246)
- Fix e2e tests and protxhash population (#273)
- Improve logging for debug purposes
- Stabilize consensus algorithm (#284)

### Refactor

- Apply peer review feedback
- Change node's proTxHash on slice from pointer of slice (#263)
- Some minor changes in validate-conn-executor and routerDashDialer (#277)
- Populate proTxHash in address-book (#274)
- Replace several functions with an identical body (processStateCh,processDataCh,processVoteCh,processVoteSetBitsCh) on one function processMsgCh (#296)

### Testing

- Ensure commit stateid in wal is OK
- KeepInvalidTxsInCache test is invalid

### Build

- Bump github.com/lib/pq from 1.10.3 to 1.10.4
- Run e2e tests in parallel
- Bump technote-space/get-diff-action from 5.0.1 to 5.0.2
- Bump github.com/adlio/schema from 1.1.14 to 1.1.15
- Bump github.com/adlio/schema from 1.1.15 to 1.2.3
- Bump docker/build-push-action from 2.7.0 to 2.9.0
- Bump github.com/gorilla/websocket from 1.4.2 to 1.5.0
- Bump github.com/rs/zerolog from 1.26.0 to 1.26.1
- Bump actions/github-script from 5 to 6
- Bump docker/login-action from 1.10.0 to 1.12.0
- Bump docker/login-action from 1.12.0 to 1.13.0
- Bump docker/login-action from 1.13.0 to 1.14.1
- Bump golangci/golangci-lint-action from 2.5.2 to 3.1.0

## [0.7.0] - 2022-01-24

### Miscellaneous Tasks

- Create only 1 proof block by default
- Release script and initial changelog (#250)
- [**breaking**] Bump ABCI version and update release.sh to change TMVersionDefault automatically (#253)

### Build

- Bump github.com/prometheus/client_golang (#249)
- Bump github.com/BurntSushi/toml from 0.4.1 to 1.0.0

## [0.7.0-dev.6] - 2022-01-07

### Bug Fixes

- Change CI testnet config from ci.toml on dashcore.toml
- Update the title of pipeline task
- Ensure seed at least once connects to another seed (#200)
- Panic on precommits does not have any +2/3 votes
- Improved error handling  in DashCoreSignerClient
- Don't disconnect already disconnected validators

### Documentation

- Add description about how to keep validators public keys at full node
- Add information how to sue preset for network generation
- Change a type of code block

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

### Refactor

- Minor formatting improvements
- Apply peer review feedback

### Testing

- Regenerate  remote_client mock
- Get rid of workarounds for issues fixed in 0.6.1
- Clean up databases in tests (#6304)
- Improve cleanup for data and disk use (#6311)
- Close db in randConsensusNetWithPeers, just as it is in randConsensusNet
- Ensure commit stateid in wal is OK

### Buf

- Modify buf.yml, add buf generate (#5653)

### Build

- Fix proto-lint step in Makefile
- Github workflows: fix dependabot and code coverage (#191)
- Bump github.com/adlio/schema from 1.1.13 to 1.1.14
- Bump github.com/lib/pq from 1.10.3 to 1.10.4
- Run e2e tests in parallel
- Bump technote-space/get-diff-action from 5.0.1 to 5.0.2
- Bump github.com/adlio/schema from 1.1.14 to 1.1.15
- Bump github.com/adlio/schema from 1.1.15 to 1.2.3
- Bump github.com/rs/cors from 1.8.0 to 1.8.2

### E2e

- Add option to dump and analyze core dumps

## [0.6.1-dev.1] - 2021-10-26

### Bug Fixes

- Accessing validator state safetly
- Safe state access in TestValidProposalChainLocks
- Safe state access in TestReactorInvalidBlockChainLock
- Safe state access in TestReactorInvalidBlockChainLock
- Seeds should not hang when disconnected from all nodes

### Buf

- Modify buf.yml, add buf generate (#5653)

### Build

- Bump rtCamp/action-slack-notify from 2.1.1 to 2.2.0
- Fix proto-lint step in Makefile

### Light

- Fix panic when empty commit is received from server

## [0.6.0] - 2021-10-14

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
- Fix MD after the lint
- To avoid potential race conditions the validator-set-update is needed to copy rather than using the pointer to the field at PersistentKVStoreApplication, 'cause it leads to a race condition
- Update a comment block

### Documentation

- State ID
- State-id.md typos and grammar
- Remove invalid info about initial state id
- Add some code comments
- ADR: Inter Validator Set Messaging
- Adr-d001: apllied feedback, added additional info
- Adr-d001 clarified abci protocol changes
- Adr-d001 describe 3 scenarios and minor restructure
- Adr-d001: clarify terms based on peer  review
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
- Install abci-cli when running make tests_integrations (#6834)

### Abci

- Change client to use multi-reader mutexes (backport #6306) (#6873)

### Add

- Update e2e doc

### Build

- E2e docker app can be run with dlv debugger
- Improve e2e docker container debugging
- Update all deps to most recent version
- Replace github.com/go-kit/kit/log with github.com/go-kit/log

### Cleanup

- Remove not needed binary test/app/grpc_client

### Config

- Add example on external_address (backport #6621) (#6624)

### E2e

- Disable app tests for light client (#6672)
- Avoid starting nodes from the future (#6835) (#6838)
- Cleanup node start function (#6842) (#6848)

### Internal/consensus

- Update error log (#6863) (#6867)

### Light

- Fix early erroring (#6905)

### Rpc

- Add chunked rpc interface (backport #6445) (#6717)

### Statesync

- Improve stateprovider handling in the syncer (backport) (#6881)

## [0.6.0-dev.2] - 2021-09-10

### Features

- Info field with arbitrary data to ResultBroadcastTx

### Abci

- Change client to use multi-reader mutexes (backport #6306) (#6873)

### E2e

- Cleanup node start function (#6842) (#6848)

### Internal/consensus

- Update error log (#6863) (#6867)

### Light

- Fix early erroring (#6905)

### Statesync

- Improve stateprovider handling in the syncer (backport) (#6881)

## [0.6.0-dev.1] - 2021-08-19

### Features

- [**breaking**] Proposed app version (#148)

### Miscellaneous Tasks

- Bump tenderdash version to 0.6.0-dev.1

### Testing

- Install abci-cli when running make tests_integrations (#6834)

### Changelog

- Prepare for v0.34.12 (#6831)

### Changelog_pending

- Add missing entry (#6830)

### E2e

- Avoid starting nodes from the future (#6835) (#6838)

### Rpc

- Log update (backport #6825) (#6826)

### Version

- Bump for 0.34.12 (#6832)

## [0.5.11-dev.4] - 2021-07-31

### State/privval

- Vote timestamp fix (backport #6748) (#6783)

## [0.5.10] - 2021-07-26

### Build

- Bump golangci/golangci-lint-action from 2.3.0 to 2.5.2

## [0.5.7] - 2021-07-16

### Bug Fixes

- Maverick compile issues (#104)
- Private validator key still automatically creating (#120)
- Getting pro tx hash from full node
- Incorrectly assume amd64 arch during docker build
- Image isn't pushed after build

### Documentation

- Rename tenderdash and update target repo
- Update github issue and pr templates (#131)

### Features

- Improve initialisation (#117)
- Add arm64 arch for builds

### Miscellaneous Tasks

- Target production dockerhub org
- Use official docker action

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

### Config

- Add example on external_address (backport #6621) (#6624)

### E2e

- Disable app tests for light client (#6672)

### Light

- Correctly handle contexts (backport -> v0.34.x) (#6685)
- Add case to catch cancelled contexts within the detector (backport #6701) (#6720)

### Release

- Prepare changelog for v0.34.11 (#6597)

### Rpc

- Add chunked rpc interface (backport #6445) (#6717)

### Statesync

- Increase chunk priority and robustness (#6582)

## [0.4.1] - 2021-06-09

### .github

- Split the issue template into two seperate templates (#2073)
- Add markdown link checker (#4513)
- Move checklist from PR description into an auto-comment (#4745)
- Fix whitespace for autocomment (#4747)
- Fix whitespace for auto-comment (#4750)
- Rename crashers output (fuzz-nightly-test) (#5993)
- Archive crashers and fix set-crashers-count step (#5992)
- Fix fuzz-nightly job (#5965)
- Use job ID (not step ID) inside if condition (#6060)
- Remove erik as reviewer from dependapot (#6076)
- Make core team codeowners (#6384)

### .github/codeowners

- Add alexanderbez (#5913)

### .github/issue_template

- Update `/dump_consensus_state` request. (#5060)

### .github/workflows

- Try different e2e nightly test set (#6036)
- Separate e2e workflows for 0.34.x and master (#6041)
- Fix whitespace in e2e config file (#6043)
- Cleanup yaml for e2e nightlies (#6049)

### .golangci

- Disable new linters (#4024)

### .goreleaser

- Don't build linux/arm
- Build for windows
- Remove arm64 build instructions and bump changelog again (#6131)

### ADR

- Fix malleability problems in Secp256k1 signatures
- Add missing numbers as blank templates (#5154)

### ADR-016

- Add versions to Block and State (#2644)
- Add protocol Version to NodeInfo (#2654)
- Update ABCI Info method for versions (#2662)

### ADR-037

- DeliverBlock (#3420)

### ADR-053

- Update with implementation plan after prototype (#4427)
- Strengthen and simplify the state sync ABCI interface (#4610)

### ADR-057

- RPC (#4857)

### BROKEN

- Attempt to replace go-wire.JSON with json.Unmarshall in rpc

### Backport

- #6494 (#6506)

### Bech32

- Wrap error messages

### Bug Fixes

- Fix spelling of comment (#4566)
- Jsonrpc url parsing and dial function (#6264) (#6288)

### CHANGELOG

- Update release date
- Update release date
- Update release/v0.32.8 details (#4162)
- Update to reflect 0.33.5 (#4915)
- Add 0.32.12 changelog entry (#4918)
- Update for 0.34.0-rc4 (#5400)
- Prepare 0.34.1-rc1 (#5832)

### CHANGELOG_PENDING

- Fix the upcoming release number (#5103)

### CODEOWNERS

- Specify more precise codeowners (#5333)
- Remove erikgrinaker (#6057)

### CONTRIBUTING

- Include instructions for installing protobuf
- Update minor release process (#4909)

### CRandHex

- Fix up doc to mention length of digits

### Client

- DumpConsensusState, not DialSeeds. Cleanup

### Connect2Switches

- Panic on err

### Docs

- Update description of seeds and persistent peers

### Documentation

- Move FROM to golang:1.4 because 1.4.2 broke
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
- Add doc on state sync configuration (#5304)
- Move subscription to tendermint-core (#5323)
- Add missing metrics (#5325)
- Add more description to initial_height (#5350)
- Make rfc section disppear (#5353)
- Document max entries for `/blockchain` RPC (#5356)
- Fix incorrect time_iota_ms configuration (#5385)
- Specify 0.34 (#5823)
- Package-lock.json fix (#5948)
- Bump package-lock.json of v0.34.x (#5952)
- Change v0.33 version (#5950)
- Release Linux/ARM64 image (#5925)
- Dont login when in PR (#5961)
- Fix typo in state sync example (#5989)
- Fix sample code #6186

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
- Parse TENDERMINT_BUILD_OPTIONS (#4738)
- Always pull image in proto-gen-docker. (#5953)

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
- Specify supported versions (#4660)
- Update chat link with Discord instead of Riot (#5071)
- Clean up README (#5391)

### ResponseEndBlock

- Ensure Address matches PubKey if provided

### Security

- Use bytes.Equal for key comparison
- Compile time assert to, and document sort.Interface
- Remove RipeMd160.
- Implement PeerTransport
- Refactor Remote signers (#3370)
- Cross-check new header with all witnesses (#4373)
- Bump google.golang.org/grpc from 1.28.1 to 1.29.0
- Bump vuepress-theme-cosmos from 1.0.165 to 1.0.166 in /docs (#4920)
- [Security] Bump websocket-extensions from 0.1.3 to 0.1.4 in /docs (#4976)
- Bump github.com/prometheus/client_golang from 1.6.0 to 1.7.0 (#5027)
- [Security] Bump lodash from 4.17.15 to 4.17.19 in /docs
- [Security] Bump prismjs from 1.20.0 to 1.21.0 in /docs
- Bump vuepress-theme-cosmos from 1.0.169 to 1.0.172 in /docs (#5286)
- Bump google.golang.org/grpc from 1.31.0 to 1.31.1 (#5290)
- Bump github.com/golang/protobuf from 1.4.2 to 1.4.3 (#5506)
- Bump github.com/spf13/cobra from 1.0.0 to 1.1.0 (#5505)
- Bump github.com/prometheus/client_golang from 1.7.1 to 1.8.0 (#5515)
- Bump github.com/spf13/cobra from 1.1.0 to 1.1.1 (#5526)
- Bump google.golang.org/grpc from 1.33.1 to 1.33.2 (#5635)
- Bump github.com/stretchr/testify from 1.6.1 to 1.7.0 (#5897)
- Bump google.golang.org/grpc from 1.34.0 to 1.35.0 (#5902)
- Bump vuepress-theme-cosmos from 1.0.179 to 1.0.180 in /docs (#5915)
- Add security mailing list (#5916)
- Bump github.com/tendermint/tm-db from 0.6.3 to 0.6.4 (#6073)
- Bump watchpack from 2.1.0 to 2.1.1 in /docs (#6063)
- Update 0.34.3 changelog with details on security vuln (bp #6108) (#6110)

### Testing

- Broadcast_tx with tmsp; p2p
- Add throughput benchmark using mintnet and netmon
- Install mintnet, netmon
- Use MACH_PREFIX
- Cleanup
- Dont run cloud test on push to master
- README.md
- Refactor bash; test fastsync (failing)
- Name client conts so we dont need to rm them because circle
- Test dummy using rpc query
- Add xxd dep to dockerfile
- More verbosity
- Add killall to dockerfile. cleanup
- Codecov
- Use glide with mintnet/netmon
- Install glide for network test
- App persistence
- Tmsp query result is json
- Increase proposal timeout
- Cleanup and fix scripts
- Crank it to eleventy
- More cleanup on p2p
- RandConsensusNet takes more args
- Crank circle timeouts
- Automate building consensus/test_data
- Circle artifacts
- Dont start cs until all peers connected
- Shorten timeouts
- Remove codecov patch threshold
- Kill and restart all nodes
- Use PROXY_APP=persistent_dummy
- Use fail-test failure indices
- More unique container names
- Set log_level=info
- Always rebuild grpc_client
- Split up test/net/test.sh
- Unexport internal function.
- Update docker to 1.7.4
- Dont use log files on circle
- Shellcheck
- Forward CIRCLECI var through docker
- Only use syslog on circle
- More logging
- Wait for tendermint proc
- Add extra kill after fail index triggered
- Wait for ports to be freed
- Use --pex on restart
- Install abci apps first
- Fix docker and apps
- Dial_seeds
- Docker exec doesnt work on circle
- Bump sleep to 5 for bound ports release
- Better client naming
- Use unix socket for rpc
- Shellcheck
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
- Run remaining E2E testnets on run-multiple.sh failure (#5557)
- Tag E2E Docker resources and autoremove them (#5558)
- Add evidence e2e tests (#5488)
- Fix handling of start height in generated E2E testnets (#5563)
- Disable E2E misbehaviors due to bugs (#5569)
- Fix various E2E test issues (#5576)
- Fix secp failures (#5649)
- Switched node keys back to edwards (#4)
- Fix TestByzantinePrevoteEquivocation flake (#5710)
- Fix integration tests and rename binary
- Improve WaitGroup handling in Byzantine tests (#5861)
- Disable abci/grpc and blockchain/v2 due to flake (#5854)
- Don't use foo-bar.net in TestHTTPClientMakeHTTPDialer (#5997) (#6047)
- Enable pprof server to help debugging failures (#6003)
- Increase sign/propose tolerances (#6033)
- Increase validator tolerances (#6037)
- Move fuzz tests into this repo (#5918)
- Fix `make test` (#5966)

### UPGRADING

- Polish upgrading instructions for 0.34 (#5398)

### UPGRADING.md

- Write about the LastResultsHash change (#5000)

### Update

- JTMSP -> jABCI

### Vagrantfile

- Update Go version

### ValidatorSet#GetByAddress

- Return -1 if no validator was found

### WAL

- Better errors and new fail point (#3246)

### WIP

- Begin parallel refactoring with go-wire Write methods and MConnection
- Fix rpc/core
- More empty struct examples
- Add implementation of mock/fake http-server
- Rename package name from fakeserver to mockcoreserver
- Change the method names of call structure, Fix adding headers
- Add mock of JRPCServer implementation on top of HTTServer mock

### [Docs]

- Minor doc touchups (#4171)

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
- Refactor tagging events using list of lists (#3643)
- Refactor ABCI CheckTx and DeliverTx signatures (#3735)
- Refactor CheckTx to notify of recheck (#3744)
- Minor cleanups in the socket client (#3758)
- Fix documentation regarding CheckTx type update (#3789)
- Remove TotalTxs and NumTxs from Header (#3783)
- Fix broken spec link (#4366)
- Fix protobuf lint issues
- Regenerate proto files
- Remove protoreplace script
- Remove python examples
- Proto files follow same path  (#5039)
- Fix abci evidence types (#5174)
- Add ResponseInitChain.app_hash, check and record it (#5227)
- Update evidence (#5324)
- Fix socket client error for state sync responses (#5395)
- Fix ReCheckTx for Socket Client (bp #6124) (#6125)

### Abci-cli

- Print OK if code is 0
- Prefix flag variables with flag

### Abci/client

- Fix DATA RACE in gRPC client (#3798)

### Abci/example/kvstore

- Decrease val power by 1 upon equivocation (#5056)

### Abci/examples

- Switch from hex to base64 pubkey in kvstore (#3641)

### Abci/grpc

- Return async responses in order (#5520) (#5531)
- Fix ordering of sync/async callback combinations (#5556)
- Fix invalid mutex handling in StopForError() (#5849)

### Abci/kvstore

- Return `LastBlockHeight` and `LastBlockAppHash` in `Info` (#4233)

### Abci/server

- Recover from app panics in socket server (#3809)
- Print panic & stack trace to STDERR if logger is not set

### Abci/types

- Update comment (#3612)
- Add comment for TotalVotingPower (#5081)

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
- Peer Behaviour (#3539)
- PeerBehaviour updates (#3558)
- [43] blockchain riri-org (#3753)
- ADR-052: Tendermint Mode (#4302)
- ADR-051: Double Signing Risk Reduction (#4262)
- Light client implementation (#4397)
- Crypto encoding for proto (#4481)
- Add API stability ADR (#5341)

### Adr#50

- Improve trusted peering (#4072)

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

### Adr-047

- Evidence handling (#4429)

### Adr-053

- Update after state sync merge (#4768)

### All

- No more anonymous imports
- Fix vet issues with build tags, formatting
- Gofmt (#1743)
- Name reactors when they are initialized (#4608)

### Ansible

- Update tendermint and basecoin versions
- Added option to provide accounts for genesis generation, terraform: added option to secure DigitalOcean servers, devops: added DNS name creation to tendermint terraform

### Appveyor

- Use make

### Arm

- Add install script, fix Makefile (#2824)

### Autofile

- Ensure file is open in Sync
- Resolve relative paths (#4390)

### Batch

- Progress

### Behaviour

- Return correct reason in MessageOutOfOrder (#3772)
- Add simple doc.go (#5055)

### Binary

- Prevent runaway alloc

### Bit_array

- Simplify subtraction

### Block

- Fix max commit sig size (#5567)

### Block/state

- Add CallTx type
- Gas price for block and tx

### Blockchain

- Use ApplyBlock
- Thread safe store.Height()
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
- Update the maxHeight when a peer is removed (#3350)
- Comment out logger in test code that causes a race condition (#3500)
- Dismiss request channel delay (#3459)
- Reorg reactor (#3561)
- Add v2 reactor (#4361)
- Enable v2 to be set (#4597)
- Proto migration  (#4969)
- Test vectors for proto encoding (#5073)
- Fix fast sync halt with initial height > 1 (#5249)
- Verify +2/3 (#5278)

### Blockchain/pool

- Some comments and small changes

### Blockchain/reactor

- RespondWithNoResponseMessage for missing height

### Blockchain/store

- Comment about panics

### Blockchain/v0

- Stop tickers on poolRoutine exit (#5860)

### Blockchain/v1

- Add noBlockResponse handling  (#5401)
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
- Make the removal of an already removed peer a noop (#5553)
- Remove peers from the processor  (#5607)
- Fix missing mutex unlock (#5862)

### Blockchain[v1]

- Increased timeout times for peer tests (#4871)

### Blockpool

- Fix removePeer bug

### Blockstore

- Allow initial SaveBlock() at any height
- Fix race conditions when loading data (#5382)

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
- Bump google.golang.org/grpc from 1.29.0 to 1.29.1 (#4735)
- Manually bump github.com/prometheus/client_golang from 1.5.1 to 1.6.0 (#4758)
- Bump github.com/golang/protobuf from 1.4.0 to 1.4.1 (#4794)
- Bump vuepress-theme-cosmos from 1.0.163 to 1.0.164 in /docs (#4815)
- Bump github.com/spf13/viper from 1.6.3 to 1.7.0 (#4814)
- Bump github.com/golang/protobuf from 1.4.1 to 1.4.2 (#4849)
- Bump vuepress-theme-cosmos from 1.0.164 to 1.0.165 in /docs
- Bump github.com/stretchr/testify from 1.5.1 to 1.6.0
- Bump github.com/stretchr/testify from 1.6.0 to 1.6.1
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
- Bump github.com/tendermint/tm-db from 0.6.1 to 0.6.2 (#5296)
- Bump google.golang.org/grpc from 1.31.1 to 1.32.0 (#5346)
- Bump github.com/minio/highwayhash from 1.0.0 to 1.0.1 (#5370)
- Bump vuepress-theme-cosmos from 1.0.172 to 1.0.173 in /docs (#5390)
- Bump actions/cache from v2.1.1 to v2.1.2 (#5487)
- Bump golangci/golangci-lint-action from v2.2.0 to v2.2.1 (#5486)
- Bump technote-space/get-diff-action from v3 to v4 (#5485)
- Bump google.golang.org/grpc from 1.32.0 to 1.33.1 (#5544)
- Bump github.com/tendermint/tm-db from 0.6.2 to 0.6.3
- Refactor BLS library/bindings integration  (#9)
- BLS scripts - Improve build.sh, Fix install.sh (#10)
- Dashify some files (#11)
- Fix docker image and docker.yml workflow (#12)
- Fix coverage.yml, bump go version, install BLS, drop an invalid character (#19)
- Fix test.yml, bump go version, install BLS, fix job names (#18)
- Bump gaurav-nelson/github-action-markdown-link-check (#22)
- Bump codecov/codecov-action from v1.0.13 to v1.0.15 (#23)
- Bump golangci/golangci-lint-action from v2.2.1 to v2.3.0 (#24)
- Bump rtCamp/action-slack-notify from e9db0ef to 2.1.1 (#25)
- Bump google.golang.org/grpc from 1.33.2 to 1.34.0 (#26)
- Bump vuepress-theme-cosmos from 1.0.173 to 1.0.177 in /docs (#27)
- Bump actions/cache from v2.1.3 to v2.1.4 (#6055)
- Bump google.golang.org/grpc from 1.36.1 to 1.37.0 (bp #6330) (#6335)

### Certifiers

- Test uses WaitForHeight

### Changelog

- Add prehistory
- Add genesis amount->power
- More review fixes/release/v0.31.0 (#3427)
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
- Add missing date to v0.33.5 release, fix indentation (#5454) (#5455)
- Prepare changelog for RC5 (#5494)
- Squash changelog from 0.34 RCs into one (#5687)
- Update changelog for v0.34.1 (#5872)
- Prepare 0.34.2 release (#5894)
- Update for 0.34.3 (#5926)
- Update for v0.34.4 (#6096)
- Improve with suggestions from @melekes (#6097)
- Update for 0.34.5 (#6129)
- Bump to v0.34.6
- Fix changelog pending version numbering (#6149)
- Update for 0.34.8 (#6181)
- Prepare changelog for 0.34.9 release (#6333)
- Update for 0.34.10 (#6357)

### Ci

- Setup abci in dependency step
- Move over abci-cli tests
- Reduce log output in test_cover (#2105)
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
- Fix net run (#5343)
- Docker remove circleci and add github action (#5551)
- Add goreleaser (#5527)
- Tests (#5577)
- Remove `add-path` (#5674)
- Build for 32 bit, libs: fix overflow (#5700)
- Install BLS library in lint.yml and bump its go version (#15)
- E2e fixes - docker image, e2e.yml BLS library, default KeyType (#21)

### Ci/e2e

- Avoid running job when no go files are touched (#5471)

### Circle

- Docker 1.10.0
- Add metalinter to test
- Fix config.yml
- Add GOCACHE=off and -v to tests
- Save p2p logs as artifacts (#2566)

### Circleci

- Add a job to automatically update docs (#3005)
- Update go version (#3051)
- Removed complexity from docs deployment job  (#3396)
- Run P2P IPv4 and IPv6 tests in parallel (#4459)
- Fix reproducible builds test (#4497)
- Remove Gitian reproducible_builds job (#5462)

### Cleanup

- Replace common.Exit with log.Crit or log.Fatal

### Cli

- Testnet cmd inits files for testnet
- ResetAll doesnt depend on cobra
- Support --root and --home
- More descriptive naming
- Viper.Set(HomeFlag, rootDir)
- Clean up error handling
- Use cobra's new ExactArgs() feature
- WriteDemoConfig -> WriteConfigVals
- Add option to not clear address book with unsafe reset (#3606)
- Add `--cs.create_empty_blocks_interval` flag (#4205)
- Add `--db_backend` and `--db_dir` flags to tendermint node cmd (#4235)
- Add optional `--genesis_hash` flag to check genesis hash upon startup (#4238)
- Debug sub-command (#4227)
- Add command to generate shell completion scripts (#4665)
- Light home dir should default to where the full node default is (#5392)

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
- Show useful error when tm not initialised (#4512)
- Fix debug kill and change debug dump archive filename format (#4517)
- Hyphen-case cli  v0.34.1 (#5786)

### Cmd/abci-cli

- Use a single connection per session
- Implement batch

### Cmd/debug

- Execute p.Signal only when p is not nil (#4271)

### Cmd/lite

- Switch to new lite2 package (#4300)

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
- CMap: slight optimization in Keys() and Values(). (#3567)

### Common/BitArray

- Reduce fragility with methods

### Common/IsDirEmpty

- Do not mask non-existance errors

### Common/rand

- Remove exponential distribution functions (#1979)

### Config

- Hardcode default genesis.json
- Block size, consensus timeouts, recheck tx
- Cswal_light, mempool_broadcast, mempool_reap
- Toggle authenticated encryption
- Disable_data_hash (for testing)
- All urls use tcp:// or unix:// prefix
- Filter_peers defaults to false
- Reduce timeouts during test
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
- Make possible to set absolute paths for TLS cert and key (#3765)
- Move max_msg_bytes into mempool section (#3869)
- Add rocksdb as a db backend option (#4239)
- Allow fastsync.version = v2 (#4639)
- Trust period consistency (#5297)
- Rename prof_laddr to pprof_laddr and move it to rpc (#5315)
- Set time_iota_ms to timeout_commit in test genesis (#5386)
- Add state sync discovery_time setting (#5399)
- Set statesync.rpc_servers when generating config file (#5433) (#5438)
- Fix mispellings (#5914)

### Consensus

- Broadcast evidence tx on ErrVoteConflictingSignature
- Check both vote orderings for dupeout txs
- Fix negative timeout; log levels
- Msg saving and replay
- Replay console
- Use replay log to avoid sign regression
- Don't wait for wal if conS not running
- Dont allow peer round states to decrease
- Cswal doesnt write any consensus msgs in light mode
- Fix more races in tests
- Fix race from OnStop accessing cs.Height
- T.Fatal -> panic
- Hvs.Reset(height, valSet)
- Increase mempool_test timeout
- Don't print shared vars in cs.String()
- Add note about replay test
- No sign err in replay; fix a race
- Hvs.StringIndented needed a lock. addresses #284
- Test reactor
- Fix panic on POLRound=-1
- Ensure dir for cswal on reactor tests
- Lock before loading commit
- Track index of privVal
- Test validator set change
- Wal.Flush() and cleanup replay tests
- TimeoutTicker, skip TimeoutCommit on HasAll
- Mv timeoutRoutine into TimeoutTicker
- No internal vars in reactor.String()
- Sync wal.writeHeight
- Remove crankTimeoutPropose from tests
- Be more explicit when we need to write height after handshake
- Let time.Timer handle non-positive durations
- Check HasAll when TwoThirdsMajority
- Nice error msg if ApplyBlock fails
- Handshake replay test using wal
- More handshake replay tests
- Some more informative logging
- Timeout on replayLastBlock
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
- Check block parts don't exceed maximum block bytes (#5436)
- Open target WAL as read/write during autorepair (#5536) (#5547)
- Groom Logs (#5917)
- Remove privValidator from log call (#6128)
- More log grooming (bp #6140) (#6143)

### Consensus/WAL

- Benchmark WALDecode across data sizes

### Consensus/replay

- Remove timeout

### Consensus/types

- Fix BenchmarkRoundStateDeepCopy panics (#4244)

### Consensus/wal

- #HEIGHT -> #ENDHEIGHT

### Console

- Fix output, closes #93
- Fix tests

### Contributing

- Use full version from site
- Add steps for adding and removing rc branches (#5223)

### Core

- Apply megacheck vet tool (unused, gosimple, staticcheck)

### Counter

- Fix tx buffer overflow

### Cov

- Ignore autogen file (#5033)

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
- Fix infinite recursion in Secp256k1 string formatting (#5707) (#5709)

### Crypto/amino

- Address anton's comment on PubkeyAminoRoute (#2592)
- Add function to modify key codec (#4112)

### Crypto/ed25519

- Update the godocs (#2002)
- Remove privkey.Generate method (#2022)

### Crypto/hkdfchachapoly

- Add testing seal to the test vector

### Crypto/merkle

- Remove byter in favor of plain byte slices (#2595)
- Remove simple prefix (#4989)

### Crypto/random

- Use chacha20, add forward secrecy (#2562)

### Crypto/secp256k1

- Add godocs, remove indirection in privkeys (#2017)
- Fix signature malleability, adopt more efficient en… (#2239)

### Crypto|p2p|state|types

- Rename Pub/PrivKey's TypeIdentifier() and Type()

### Cs

- Prettify logging of ignored votes (#3086)
- Reset triggered timeout precommit (#3310)
- Sync WAL more frequently (#3300)
- Update wal comments (#3334)
- Comment out log.Error to avoid TestReactorValidatorSetChanges timing out (#3401)
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

### Cs/wal

- Refuse to encode msg that is bigger than maxMsgSizeBytes (#3303)

### Cswal

- Write #HEIGHT:1 for empty wal

### Daemon

- Refactor out of cmd into own package

### Db

- Add Close() to db interface. closes #31
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
- Add support for badgerdb (#5233)

### Dep

- Pin all deps to version or commit
- Revert updates
- Update tm-db to 0.4.0 (#4289)
- Bump gokit dep (#4424)
- Maunally bump dep (#4436)
- Bump protobuf, cobra, btcutil & std lib deps (#4676)

### Deps

- Update gogo/protobuf from 1.1.1 to 1.2.1 and golang/protobuf from 1.1.0 to 1.3.0 (#3357)
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

### Dist

- Dont mkdir in container
- Dont mkdir in container

### Distribution

- Lock binary dependencies to specific commits (#2550)

### Dummy

- Valset changes and tests
- Verify pubkey is go-crypto encoded in DeliverTx. closes #51
- Include app.key tag

### E2e

- Use ed25519 for secretConn (remote signer) (#5678)
- Releases nightly (#5906)
- Add benchmarking functionality (bp #6210) (#6216)
- Integrate light clients (bp #6196)
- Tx load to use broadcast sync instead of commit (backport #6347) (#6352)
- Relax timeouts (#6356)

### Ebuchman

- Added some demos on how to parse unknown types

### Ed25519

- Use golang/x/crypto fork (#2558)

### Encoding

- Remove codecs (#4996)

### Errcheck

- PR comment fixes

### Events

- Integrate event switch into services via Eventable interface

### Evidence

- More funcs in store.go
- Store tests and fixes
- Pool test
- Reactor test
- Reactor test
- Dont send evidence to unsynced peers
- Check peerstate exists; dont send old evidence
- Give each peer a go-routine
- Enforce ordering in DuplicateVoteEvidence (#4151)
- Introduce time.Duration to evidence params (#4254)
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
- Use bytes instead of quantity to limit size (#5449)(#5476)
- Don't gossip consensus evidence too soon (#5528)
- Don't send committed evidence and ignore inbound evidence that is already committed (#5574)
- Structs can independently form abci evidence (#5610)
- Omit bytes field (#5745)
- Buffer evidence from consensus (#5890)
- Terminate broadcastEvidenceRoutine when peer is stopped (#6068)
- Fix bug with hashes (backport #6375) (#6381)

### Example

- Fix func suffix

### Example/dummy

- Remove iavl dep - just use raw db

### Example/kvstore

- Return ABCI query height (#4509)

### Fmt

- Run 'make fmt'

### Format

- Add format cmd & goimport repo (#4586)

### Genesis

- Add support for arbitrary initial height (#5191)

### Github

- Update PR template to indicate changing pending changelog. (#2059)
- Edit templates for use in issues and pull requests (#4483)
- Add nightly E2E testnet action (#5480)
- Rename e2e jobs (#5502)
- Only notify nightly E2E failures once (#5559)

### Gitian

- Update reproducible builds to build with Go 1.12.8 (#3902)

### Gitignore

- Add .vendor-new (#3566)

### Glide

- Update go-common
- Update lock and add util scripts
- Update go-common
- Update go-wire
- Use versions where applicable
- Update for autofile fix
- More external deps locked to versions
- Update grpc version

### Go.mod

- Upgrade iavl and deps (#5657)

### Goreleaser

- Downcase archive and binary names (#6029)
- Reintroduce arm64 build instructions

### Grpcdb

- Better readability for docs and constructor names
- Close Iterator/ReverseIterator after use (#3424)

### Hd

- Optimize ReverseBytes + add tests
- Comments and some cleanup

### Header

- Check block protocol (#5340)

### Http

- Http-utils added after extraction

### Https

- //github.com/tendermint/tendermint/pull/1128#discussion_r162799294

### Indexer

- Allow indexing an event at runtime (#4466)
- Remove index filtering (#5006)
- Remove info log (#6194)

### Ints

- Stricter numbers (#4939)

### Json

- Add Amino-compatible encoder/decoder (#4955)

### Json2wal

- Increase reader's buffer size (#3147)

### Jsonrpc

- Change log to debug (#5131)

### Keys

- Transactions.go -> types.go
- Change to []bytes  (#4950)

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
- Remove useless code in group (#3504)
- Remove commented and unneeded code (#3757)
- Minor cleanup (#3794)
- Remove db from tendermint in favor of tendermint/tm-cmn (#3811)
- Remove bech32
- Remove kv (#4874)
- Wrap mutexes for build flag with godeadlock (#5126)

### Libs/autofile

- Bring back loops (#2261)

### Libs/autofile/group_test

- Remove unnecessary logging (#2100)

### Libs/bits

- Inline defer and change order of mutexes (#5187)

### Libs/cmn

- Remove Tempfile, Tempdir, switch to ioutil variants (#2114)

### Libs/cmn/writefileatomic

- Handle file already exists gracefully (#2113)

### Libs/common

- Refactor tempfile code into its own file
- Remove deprecated PanicXXX functions (#3595)
- Remove heap.go (#3780)
- Remove unused functions (#3784)
- Refactor libs/common 01 (#4230)
- Refactor libs/common 2 (#4231)
- Refactor libs common 3 (#4232)
- Refactor libs/common 4 (#4237)
- Refactor libs/common 5 (#4240)

### Libs/common/rand

- Update godocs

### Libs/db

- Add cleveldb.Stats() (#3379)
- Close batch (#3397)
- Close batch (#3397)
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
- [JSON format] include timestamp (bp #6174) (#6179)

### Libs/os

- EnsureDir now returns IO errors and checks file type (#5852)
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
- Cross-check the very first header (#5429)
- Run detector for sequentially validating light client (#5538) (#5601)
- Make fraction parts uint64, ensuring that it is always positive (#5655)
- Fix panic with RPC calls to commit and validator when height is nil (#6040)
- Remove witnesses in order of decreasing index (#6065)
- Handle too high errors correctly (backport #6346) (#6351)

### Light/evidence

- Handle FLA backport (#6331)

### Light/provider/http

- Fix Validators (#6024)

### Light/rpc

- Fix ABCIQuery (#5375)

### Lint

- Remove dot import (go-common)
- S/common.Fmt/fmt.Sprintf
- S/+=1/++, remove else clauses
- Couple more fixes
- Apply deadcode/unused
- Golint issue fixes (#4258)
- Add review dog (#4652)
- Enable nolintlinter, disable on tests
- Various fixes
- Errcheck  (#5091)
- Add markdown linter (#5254)
- Add errchecks (#5316)
- Enable errcheck (#5336)
- Run gofmt and goimports  (#13)

### Linter

- Couple fixes
- Add metalinter to Makefile & apply some fixes
- Last fixes & add to circle
- Address deadcode, implement incremental lint testing
- Sort through each kind and address small fixes
- Enable in CI & make deterministic
- (1/2) enable errcheck (#5064)
- Fix some bls-related linter issues (#14)

### Linters

- Enable scopelint (#3963)
- Modify code to pass maligned and interfacer (#3959)
- Enable stylecheck (#4153)

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
- Remove unused variable

### Lite

- MemStoreProvider GetHeightBinarySearch method + fix ValKeys.signHeaders
- < len(v) in for loop check, as per @melekes' recommendation
- TestCacheGetsBestHeight with GetByHeight and GetByHeightBinarySearch
- Comment out iavl code - TODO #1183
- Add synchronization in lite verify (#2396)
- Follow up from #3989 (#4209)
- Modified bisection to loop (#4400)
- Add helper functions for initiating the light client (#4486)
- Fix HTTP provider error handling

### Lite/proxy

- Validation* tests and hardening for nil dereferences
- Consolidate some common test headers into a variable

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

- Fix $LOG variable (#3423)

### Log

- Move some Info to Debug
- Tm -> TM

### Logging

- Print string instead of callback (#6178)
- Shorten precommit log message (#6270) (#6274)

### Logs

- Cleanup (#6198)

### Make

- Dont use -v on go test
- Update protoc_abci use of awk
- Add back tools cmd (#4281)
- Remove sentry setup cmds (#4383)

### Makefile

- Remove megacheck
- Fix protoc_libs
- Add `make check_dep` and remove `make ensure_deps` (#2055)
- Lint flags
- Fix build-docker-localnode target (#3122)
- Minor cleanup (#3994)
- Place phony markers after targets (#4408)
- Add options for other DBs (#5357)
- Remove call to tools (#6104)

### Markdownlint

- Ignore .github directory (#5351)

### Maverick

- Reduce some duplication (#6052)

### Mempool

- Add GetState()
- Remove bad txs from cacheMap
- Don't remove committed txs from cache
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
- Introduce KeepInvalidTxsInCache config option (#5813)
- Disable MaxBatchBytes (#5800)

### Mempool/reactor

- Fix reactor broadcast test (#5362)

### Mempool/rpc

- Log grooming (bp #6201) (#6203)

### Mergify

- Remove unnecessary conditions (#4501)
- Use strict merges (#4502)
- Use PR title and body for squash merge commit (#4669)

### Merkle

- Go-common -> tmlibs
- Remove go-wire dep by copying EncodeByteSlice
- Remove unused funcs. unexport simplemap. improv docs
- Use amino for byteslice encoding
- Return hashes for empty merkle trees (#5193)

### Metalinter

- Add linter to Makefile like tendermint

### Metrics

- Add additional metrics to p2p and consensus (#2425)
- Only increase last_signed_height if commitSig for block (#4283)
- Switch from gauge to histogram (#5326)

### Mocks

- Update with 2.2.1 (#5294)

### Mod

- Go mod tidy

### Nano

- Update comments

### Networks

- Update readmes

### Networks/remote

- Turn on GO111MODULE and use git clone instead of go get (#4203)

### Node

- ConfigFromViper
- NewNode takes DBProvider and GenDocProvider
- Clean makeNodeInfo
- Remove dup code from rebase
- Remove commented out trustMetric
- Respond always to OS interrupts (#2479)
- Refactor privValidator ext client code & tests (#2895)
- Refactor node.NewNode (#3456)
- Fix a bug where `nil` is recorded as node's address (#3740)
- Run whole func in goroutine, not just logger.Error fn (#3743)
- Allow registration of custom reactors while creating node (#3771)
- Use GRPCMaxOpenConnections when creating the gRPC server (#4349)
- Don't attempt fast sync when InitChain sets self as only validator (#5211)
- Fix genesis state propagation to state sync (#5302)

### Note

- Add nondeterministic note to events (#6220) (#6225)

### Os

- Simplify EnsureDir() (#5871)

### P2p

- Push handshake containing chainId for early disconnect. Closes #12
- Fix switch_test to account for handshake
- Broadcast spawns goroutine to Send on each peer and times out after 10 seconds. Closes #7
- Fix switch test for Broadcast returning success channel
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
- Fix MConnection inbound traffic statistics and rate limiting (#5868) (#5870)
- Fix "Unknown Channel" bug on CustomReactors (#6297)
- Fix using custom channels (#6339)

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
- Add Bufferpool (#3664)
- Simplify secret connection handshake malleability fix with merlin (#4185)
- Add a test for MakeSecretConnection (#4829)
- Migrate to Protobuf (#4990)
- Check for channel id overflow before processing receive msg (backport #6522) (#6528)

### P2p/connetion

- Remove panics, test error cases

### P2p/pex

- Simplify ensurePeers
- Wait to connect to all peers in reactor test
- Minor cleanup and comments
- Some addrbook fixes
- Allow configured seed nodes to not be resolvable over DNS (#2129)
- Fix mismatch between dialseeds and checkseeds. (#2151)
- Consult seeds in crawlPeersRoutine (#3647)
- Fix DATA RACE
- Migrate to Protobuf (#4973)

### P2p/secret_connection

- Switch salsa usage to hkdf + chacha

### P2p/test

- Wait for listener to get ready (#4881)
- Fix Switch test race condition (#4893)

### P2p/trust

- Split into multiple files and improve function order
- Lock on Copy()
- Remove extra channels
- Fix nil pointer error on TrustMetric Copy() (#1819)

### P2p/trustmetric

- Non-deterministic test

### Pex

- Dial seeds when address book needs more addresses (#3603)
- Various follow-ups (#3605)
- Use highwayhash for pex bucket

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
- Increase timeout to mitigate non-deterministic test failure (#3580)
- Remove misplaced debug statement (#4103)
- Add `SignerDialerEndpointRetryWaitInterval` option (#4115)
- Return error on getpubkey (#4534)
- Remove deprecated `OldFilePV`
- Retry GetPubKey/SignVote/SignProposal N times before
- Migrate to protobuf (#4985)
- If remote signer errors, don't retry (#5140)
- Add chainID to requests (#5239)
- Allow passing options to NewSignerDialerEndpoint (#5434) (#5437)
- Fix ping message encoding (#5442)
- Make response values non nullable (#5583)
- Increase read/write timeout to 5s and calculate ping interva… (#5666)
- Reset pingTimer to avoid sending unnecessary pings (#5642) (#5668)

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
- Bump gogoproto (1.3.2) (#5886)
- Docker deployment (#5931)

### Proto/tendermint/abci

- Fix Request oneof numbers (#5116)

### Protoc

- "//nolint: gas" directive after pb generation (#164)

### Proxy

- Typed app conns
- NewAppConns takes a NewTMSPClient func
- Wrap NewTMSPClient in ClientCreator
- Nil -> nilapp
- Remove Handshaker from proxy pkg (#2437)
- Improve ABCI app connection handling (#5078)

### Pubsub

- Comments
- Fixes after Ethan's review (#3212)

### Reactors

- Omit incoming message bytes from reactor logs (#5743)

### Readme

- Js-tmsp -> js-abci
- Update install instruction (#100)
- Re-organize & update docs links
- Fix link to original paper (#4391)
- Add discord to readme (#4533)
- Add badge for git tests (#4732)
- Add source graph badge (#4980)

### Relase_notes

- Add release notes for v0.34.0

### Release

- Minor release 0.33.1 (#4401)

### Remotedb

- A client package implementing the db.DB interface

### Removal

- Remove build folder (#4565)

### Repeat_timer

- Drain channel in Stop; done -> wg

### Replay

- Larger read buffer
- More tests
- Ensure cs.height and wal.height match

### Rfc

- Add end-to-end testing RFC (#5337)

### Rpc

- Add status and net info
- Return tx hash, creates contract, contract addr in broadcast (required some helper functions). Closes #30
- Give each call a dedicated Response struct, add basic test
- Separate out golang API into rpc/core
- Generalized rpc using reflection on funcs and params
- Fixes for better type handlings, explicit error field in response, more tests
- Cleanup, more tests, working http and jsonrpc
- Fix tests to count mempool; copy responses to avoid data races
- Return (*Response, error) for all functions
- GetStorage and Call methods. Tests.
- Decrement mempool count after block mined
- GetStorage and Call methods. Tests.
- Decrement mempool count after block mined
- Auto generated client methods using rpc-gen
- Myriad little fixes
- Cleanup, use client for tests, rpc-gen fixes
- Websockets
- Tests cleanup, use client lib for JSONRPC testing too
- Test CallCode and Call
- Fix memcount error in tests
- Use gorilla websockets
- First successful websocket event subscription
- Websocket events testing
- Use NewBlock event in rpc tests
- Cleanup tests and test contract calls
- Genesis route
- Remove unecessary response wrappers
- Add app_hash to /status
- TMResult and TMEventData
- Test cleanup
- Unsafe_set_config
- Num_unconfirmed_txs (avoid sending txs back)
- Start/stop cpu profiler
- Unsafe_write_heap_profile
- Broadcast tests. closes #219
- Unsafe_flush_mempool. closes #190
- Use interfaces for pipe
- Remove restriction on DialSeeds
- /commit
- Fix SeenCommit condition
- Dial_seeds msg. addresses #403
- Better arg validation for /tx
- /tx allows height+hash
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
- Fix content-type header
- Index block events to support block event queries (bp #6226) (#6261)

### Rpc/client

- Use compile time assertions instead of methods
- Include NetworkClient interface into Client interface (#3473)
- Add basic authentication (#4291)
- Split out client packages (#4628)
- Take context as first param (#5347)

### Rpc/client/http

- Log error (#5182)

### Rpc/core

- Ints are strings in responses, closes #1896
- Do not lock ConsensusState mutex
- Return an error if `page=0` (#4947)

### Rpc/core/types

- UintX -> int

### Rpc/jsonrpc

- Unmarshal RPCRequest correctly (bp #6191) (#6193)

### Rpc/jsonrpc/server

- Merge WriteRPCResponseHTTP and WriteRPCResponseAr (#5141)
- Ws server optimizations (#5312)
- Return an error in WriteRPCResponseHTTP(Error) (bp #6204) (#6230)

### Rpc/lib

- No Result wrapper
- Test tcp and unix
- Set logger on ws conn
- Remove dead files, closes #710
- Write a test for TLS server (#3703)
- Fix RPC client, which was previously resolving https protocol to http (#4131)

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

### Rpc/swagger

- Add numtxs to blockmeta (#4139)

### Rpc/test

- /tx
- Restore txindexer after setting null
- Fix test race in TestAppCalls (#4894)
- Wait for mempool CheckTx callback (#4908)
- Wait for subscription in TestTxEventsSentWithBroadcastTxAsync (#4907)

### Rpc/tests

- Panic dont t.Fatal. use random txs for broadcast

### Rpc/wsevents

- Small cleanup

### Rtd

- Build fixes

### Scripts

- Quickest/easiest fresh install
- Remove install scripts (#4242)

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

- Forgot a file
- Version bump 0.7.4
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

- ExecTx bug fixes for create contract
- Fix debug logs
- Fix CreateAddress to use Address not Word
- Fixes for creating a contract and msging it in the same block
- Fix GetStorage on blockcache with unknown account
- FireEvents flag on ExecTx and fixes for GetAccount
- ApplyBlock
- AppHashIsStale -> IntermediateState
- Remove StateIntermediate
- ABCIResponses, s.Save() in ApplyBlock
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
- Fix block event indexing reserved key check (#6314) (#6315)

### State/store

- Remove extra `if` statement (#3774)

### Statesync

- Use Protobuf instead of Amino for p2p traffic (#4943)
- Fix valset off-by-one causing consensus failures (#5311)
- Broadcast snapshot request to all peers on startup (#5320)
- Fix the validator set heights (again) (#5330)
- Check all necessary heights when adding snapshot to pool (#5516) (#5518)
- Improve e2e test outcomes (backport #6378) (#6380)

### Store

- Register block amino, not just crypto (#3894)
- Proto migration (#4974)

### Swagger

- Update swagger port (#4498)
- Remove duplicate blockID 
- Define version (#4952)
- Update (#5257)

### Template

- Add labels to pr template

### Throttle_timer

- Fix race, use mtx instead of atomic

### Tm-bench

- Improve code shape
- Update dependencies, add total metrics
- Add deprecation warning (#3992)

### Tm-monitor

- Update health after we added / removed node (#2694)
- Update build-docker Makefile target (#3790)
- Add Context to RPC handlers (#3792)

### Tmbench

- Fix iterating through the blocks, update readme
- Make tx size configurable
- Update dependencies to use tendermint's master
- Make sendloop act in one second segments (#110)
- Make it more resilient to WSConn breaking (#111)

### Tmhash

- Add Sum function

### Tmsp

- ResponseInfo and ResponseEndBlock

### Tmtime

- Canonical, some comments (#2312)

### Toml

- Make sections standout (#4993)

### Tool

- Add Mergify (#4490)

### Tooling

- Remove tools/Makefile (bp #6102) (#6106)

### Tools

- Remove redundant grep -v vendors/ (#1996)
- Clean up Makefile and remove LICENSE file (#2042)
- Refactor tm-bench (#2570)
- Remove need to install buf (#4605)
- Update gogoproto get cmd (#5007)

### Tools.mk

- Use tags instead of revisions where possible
- Install protoc

### Tools/build

- Delete stale tools (#4558)

### Tools/tm-bench

- Don't count the first block if its empty
- Remove testing flags from help (#1949)
- Don't count the first block if its empty (#1948)
- Bounds check for txSize and improving test cases (#2410)
- Remove tm-bench in favor of tm-load-test (#4169)

### Tools/tm-signer-harness

- Fix listener leak in newTestHarnessListener() (#5850)

### Tools/tmbench

- Fix the end time being used for statistics calculation
- Improve accuracy with large tx sizes.
- Move statistics to a seperate file

### Txindexer

- Refactor Tx Search Aggregation (#3851)

### Types

- PrivVal.LastSignature. closes #247
- Pretty print validators
- Update LastBlockInfo and ConfigInfo
- Copy vote set bit array
- Copy commit bit array
- Benchmark WriteSignBytes
- Canonical_json.go
- SignatureEd25519 -> Signature
- Use mtx on PartSet.String()
- ValSet LastProposer->Proposer and Proposer()->GetProposer()
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

### Types/heartbeat

- Test all Heartbeat functions

### Types/params

- Introduce EvidenceParams

### Types/priv_validator

- Fixes for latest p2p and cmn

### Types/test

- Remove slow test cases in TestValSetUpdatePriorityOrderTests (#4903)

### Types/time

- Add note about stripping monotonic part

### Types/validator_set_test

- Move funcs around

### Upgrading

- Add note on rpc/client subpackages (#4636)
- State store change (#5364)
- Update 0.34 instructions with updates since RC4 (#5686)

### Upnp

- Keep a link

### Ux

- Use docker to format proto files (#5384)

### Version

- Bump 0.7.3
- Add and bump abci version
- Types
- Bump version numbers (#5173)

### Vm

- Check errors early to avoid infinite loop
- Fix Pad functions, state: add debug log for create new account
- Fix endianess by flipping on subslic
- Flip sha3 result
- Fix errors not being returned
- Eventable and flip fix on CALL address
- Catch stack underflow on Peek()

### Wal

- Gr.Close()

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

<!-- generated by git-cliff -->
