package types

import (
	"bytes"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

var (
	valEd25519   = []string{ABCIPubKeyTypeEd25519}
	valSecp256k1 = []string{ABCIPubKeyTypeSecp256k1}
	valSr25519   = []string{ABCIPubKeyTypeSr25519}
)

func TestConsensusParamsValidation(t *testing.T) {
	testCases := []struct {
		name   string
		params ConsensusParams
		valid  bool
	}{
		// test block params
		{
			name: "block params valid",
			params: makeParams(makeParamsArgs{
				blockBytes:   1,
				evidenceAge:  2,
				precision:    1,
				messageDelay: 1}),
			valid: true,
		},
		{
			name: "block params invalid MaxBytes",
			params: makeParams(makeParamsArgs{
				blockBytes:   0,
				evidenceAge:  2,
				precision:    1,
				messageDelay: 1}),
			valid: false,
		},
		{
			name: "block params large MaxBytes",
			params: makeParams(makeParamsArgs{
				blockBytes:   47 * 1024 * 1024,
				evidenceAge:  2,
				precision:    1,
				messageDelay: 1}),
			valid: true,
		},
		{
			name: "block params small MaxBytes",
			params: makeParams(makeParamsArgs{
				blockBytes:   10,
				evidenceAge:  2,
				precision:    1,
				messageDelay: 1}),
			valid: true,
		},
		{
			name: "block params 100MB MaxBytes",
			params: makeParams(makeParamsArgs{
				blockBytes:   100 * 1024 * 1024,
				evidenceAge:  2,
				precision:    1,
				messageDelay: 1}),
			valid: true,
		},
		{
			name: "block params MaxBytes too large",
			params: makeParams(makeParamsArgs{
				blockBytes:   101 * 1024 * 1024,
				evidenceAge:  2,
				precision:    1,
				messageDelay: 1}),
			valid: false,
		},
		{
			name: "block params 1GB MaxBytes",
			params: makeParams(makeParamsArgs{
				blockBytes:   1024 * 1024 * 1024,
				evidenceAge:  2,
				precision:    1,
				messageDelay: 1}),
			valid: false,
		},
		// test evidence params
		{
			name: "evidence MaxAge and MaxBytes 0",
			params: makeParams(makeParamsArgs{
				blockBytes:       1,
				evidenceAge:      0,
				maxEvidenceBytes: 0,
				precision:        1,
				messageDelay:     1}),
			valid: false,
		},
		{
			name: "evidence MaxBytes greater than Block.MaxBytes",
			params: makeParams(makeParamsArgs{
				blockBytes:       1,
				evidenceAge:      2,
				maxEvidenceBytes: 2,
				precision:        1,
				messageDelay:     1}),
			valid: false,
		},
		{
			name: "evidence size below Block.MaxBytes",
			params: makeParams(makeParamsArgs{
				blockBytes:       1000,
				evidenceAge:      2,
				maxEvidenceBytes: 1,
				precision:        1,
				messageDelay:     1}),
			valid: true,
		},
		{
			name: "evidence MaxAgeDuration < 0",
			params: makeParams(makeParamsArgs{
				blockBytes:       1,
				evidenceAge:      -1,
				maxEvidenceBytes: 0,
				precision:        1,
				messageDelay:     1}),
			valid: false,
		},
		{
			name: "no pubkey types",
			params: makeParams(makeParamsArgs{
				evidenceAge:  2,
				pubkeyTypes:  []string{},
				precision:    1,
				messageDelay: 1}),
			valid: false,
		},
		{
			name: "invalid pubkey types",
			params: makeParams(makeParamsArgs{
				evidenceAge:  2,
				pubkeyTypes:  []string{"potatoes make good pubkeys"},
				precision:    1,
				messageDelay: 1}),
			valid: false,
		},
		{
			name: "negative MessageDelay",
			params: makeParams(makeParamsArgs{
				evidenceAge:  2,
				precision:    1,
				messageDelay: -1}),
			valid: false,
		},
		{
			name: "negative Precision",
			params: makeParams(makeParamsArgs{
				evidenceAge:  2,
				precision:    -1,
				messageDelay: 1}),
			valid: false,
		},
	}
	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.valid {
				assert.NoErrorf(t, tc.params.ValidateConsensusParams(), "expected no error for valid params (#%d)", i)
			} else {
				assert.Errorf(t, tc.params.ValidateConsensusParams(), "expected error for non valid params (#%d)", i)
			}
		})
	}
}

type makeParamsArgs struct {
	blockBytes          int64
	blockGas            int64
	recheck             bool
	evidenceAge         int64
	maxEvidenceBytes    int64
	pubkeyTypes         []string
	precision           time.Duration
	messageDelay        time.Duration
	bypassCommitTimeout bool

	propose                *time.Duration
	proposeDelta           *time.Duration
	vote                   *time.Duration
	voteDelta              *time.Duration
	commit                 *time.Duration
	evidenceMaxAgeDuration *time.Duration

	appVersion uint64

	abciExtensionHeight int64
}

func makeParams(args makeParamsArgs) ConsensusParams {
	if args.pubkeyTypes == nil {
		args.pubkeyTypes = valEd25519
	}
	if args.propose == nil {
		args.propose = durationPtr(1)
	}
	if args.proposeDelta == nil {
		args.proposeDelta = durationPtr(1)
	}
	if args.vote == nil {
		args.vote = durationPtr(1)
	}
	if args.voteDelta == nil {
		args.voteDelta = durationPtr(1)
	}
	if args.commit == nil {
		args.commit = durationPtr(1)
	}
	if args.evidenceMaxAgeDuration == nil {
		args.evidenceMaxAgeDuration = durationPtr(time.Duration(args.evidenceAge))
	}

	return ConsensusParams{
		Block: BlockParams{
			MaxBytes: args.blockBytes,
			MaxGas:   args.blockGas,
		},
		Evidence: EvidenceParams{
			MaxAgeNumBlocks: args.evidenceAge,
			MaxAgeDuration:  *args.evidenceMaxAgeDuration,
			MaxBytes:        args.maxEvidenceBytes,
		},
		Validator: ValidatorParams{
			PubKeyTypes: args.pubkeyTypes,
		},
		Synchrony: SynchronyParams{
			Precision:    args.precision,
			MessageDelay: args.messageDelay,
		},
		Timeout: TimeoutParams{
			Propose:             *args.propose,
			ProposeDelta:        *args.proposeDelta,
			Vote:                *args.vote,
			VoteDelta:           *args.voteDelta,
			Commit:              *args.commit,
			BypassCommitTimeout: args.bypassCommitTimeout,
		},
		Version: VersionParams{
			AppVersion: args.appVersion,
		},
		ABCI: ABCIParams{
			VoteExtensionsEnableHeight: args.abciExtensionHeight,
			RecheckTx:                  args.recheck,
		},
	}
}

func TestConsensusParamsHash(t *testing.T) {
	type consensusParamsTestcase struct {
		name   string
		params ConsensusParams
		hash   []byte
	}

	// Take first case as a base case, tweak one parameter for subsequent cases
	// to make sure that these all get different hashes.
	testcases := []*consensusParamsTestcase{
		{
			name: "#1 Base case",
			params: makeParams(makeParamsArgs{
				blockBytes:             4,
				blockGas:               6,
				recheck:                true,
				evidenceAge:            5,
				maxEvidenceBytes:       4,
				pubkeyTypes:            []string{"type"},
				precision:              time.Second * 2,
				messageDelay:           time.Second * 3,
				bypassCommitTimeout:    true,
				propose:                durationPtr(time.Second * 4),
				proposeDelta:           durationPtr(time.Second * 5),
				vote:                   durationPtr(time.Second * 6),
				voteDelta:              durationPtr(time.Second * 7),
				commit:                 durationPtr(time.Second * 8),
				evidenceMaxAgeDuration: durationPtr(time.Second * 9),
				appVersion:             1,
				abciExtensionHeight:    2,
			}),
		},
		{
			name: "#2 Change blockBytes",
			params: makeParams(makeParamsArgs{
				blockBytes:             5,
				blockGas:               6,
				recheck:                true,
				evidenceAge:            5,
				maxEvidenceBytes:       4,
				pubkeyTypes:            []string{"type"},
				precision:              time.Second * 2,
				messageDelay:           time.Second * 3,
				bypassCommitTimeout:    true,
				propose:                durationPtr(time.Second * 4),
				proposeDelta:           durationPtr(time.Second * 5),
				vote:                   durationPtr(time.Second * 6),
				voteDelta:              durationPtr(time.Second * 7),
				commit:                 durationPtr(time.Second * 8),
				evidenceMaxAgeDuration: durationPtr(time.Second * 9),
				appVersion:             1,
				abciExtensionHeight:    2,
			}),
		},
		{
			name: "#3 Change blockGas",
			params: makeParams(makeParamsArgs{
				blockBytes:             4,
				blockGas:               7,
				recheck:                true,
				evidenceAge:            5,
				maxEvidenceBytes:       4,
				pubkeyTypes:            []string{"type"},
				precision:              time.Second * 2,
				messageDelay:           time.Second * 3,
				bypassCommitTimeout:    true,
				propose:                durationPtr(time.Second * 4),
				proposeDelta:           durationPtr(time.Second * 5),
				vote:                   durationPtr(time.Second * 6),
				voteDelta:              durationPtr(time.Second * 7),
				commit:                 durationPtr(time.Second * 8),
				evidenceMaxAgeDuration: durationPtr(time.Second * 9),
				appVersion:             1,
				abciExtensionHeight:    2,
			}),
		},
		{
			name: "#4 Change recheck",
			params: makeParams(makeParamsArgs{
				blockBytes:             4,
				blockGas:               6,
				recheck:                false,
				evidenceAge:            5,
				maxEvidenceBytes:       4,
				pubkeyTypes:            []string{"type"},
				precision:              time.Second * 2,
				messageDelay:           time.Second * 3,
				bypassCommitTimeout:    true,
				propose:                durationPtr(time.Second * 4),
				proposeDelta:           durationPtr(time.Second * 5),
				vote:                   durationPtr(time.Second * 6),
				voteDelta:              durationPtr(time.Second * 7),
				commit:                 durationPtr(time.Second * 8),
				evidenceMaxAgeDuration: durationPtr(time.Second * 9),
				appVersion:             1,
				abciExtensionHeight:    2,
			}),
		},
		{
			name: "#5 Change evidenceAge",
			params: makeParams(makeParamsArgs{
				blockBytes:             4,
				blockGas:               6,
				recheck:                true,
				evidenceAge:            6,
				maxEvidenceBytes:       4,
				pubkeyTypes:            []string{"type"},
				precision:              time.Second * 2,
				messageDelay:           time.Second * 3,
				bypassCommitTimeout:    true,
				propose:                durationPtr(time.Second * 4),
				proposeDelta:           durationPtr(time.Second * 5),
				vote:                   durationPtr(time.Second * 6),
				voteDelta:              durationPtr(time.Second * 7),
				commit:                 durationPtr(time.Second * 8),
				evidenceMaxAgeDuration: durationPtr(time.Second * 9),
				appVersion:             1,
				abciExtensionHeight:    2,
			}),
		},
		{
			name: "#6 Change maxEvidenceBytes",
			params: makeParams(makeParamsArgs{
				blockBytes:             4,
				blockGas:               6,
				recheck:                true,
				evidenceAge:            5,
				maxEvidenceBytes:       5,
				pubkeyTypes:            []string{"type"},
				precision:              time.Second * 2,
				messageDelay:           time.Second * 3,
				bypassCommitTimeout:    true,
				propose:                durationPtr(time.Second * 4),
				proposeDelta:           durationPtr(time.Second * 5),
				vote:                   durationPtr(time.Second * 6),
				voteDelta:              durationPtr(time.Second * 7),
				commit:                 durationPtr(time.Second * 8),
				evidenceMaxAgeDuration: durationPtr(time.Second * 9),
				appVersion:             1,
				abciExtensionHeight:    2,
			}),
		},
		{
			name: "#7 Change pubkeyTypes",
			params: makeParams(makeParamsArgs{
				blockBytes:             4,
				blockGas:               6,
				recheck:                true,
				evidenceAge:            5,
				maxEvidenceBytes:       4,
				pubkeyTypes:            []string{"type2"},
				precision:              time.Second * 2,
				messageDelay:           time.Second * 3,
				bypassCommitTimeout:    true,
				propose:                durationPtr(time.Second * 4),
				proposeDelta:           durationPtr(time.Second * 5),
				vote:                   durationPtr(time.Second * 6),
				voteDelta:              durationPtr(time.Second * 7),
				commit:                 durationPtr(time.Second * 8),
				evidenceMaxAgeDuration: durationPtr(time.Second * 9),
				appVersion:             1,
				abciExtensionHeight:    2,
			}),
		},
		{
			name: "#8 Change precision",
			params: makeParams(makeParamsArgs{
				blockBytes:             4,
				blockGas:               6,
				recheck:                true,
				evidenceAge:            5,
				maxEvidenceBytes:       4,
				pubkeyTypes:            []string{"type"},
				precision:              time.Second * 3,
				messageDelay:           time.Second * 3,
				bypassCommitTimeout:    true,
				propose:                durationPtr(time.Second * 4),
				proposeDelta:           durationPtr(time.Second * 5),
				vote:                   durationPtr(time.Second * 6),
				voteDelta:              durationPtr(time.Second * 7),
				commit:                 durationPtr(time.Second * 8),
				evidenceMaxAgeDuration: durationPtr(time.Second * 9),
				appVersion:             1,
				abciExtensionHeight:    2,
			}),
		},
		{
			name: "#9 Change messageDelay",
			params: makeParams(makeParamsArgs{
				blockBytes:             4,
				blockGas:               6,
				recheck:                true,
				evidenceAge:            5,
				maxEvidenceBytes:       4,
				pubkeyTypes:            []string{"type"},
				precision:              time.Second * 2,
				messageDelay:           time.Second * 4,
				bypassCommitTimeout:    true,
				propose:                durationPtr(time.Second * 4),
				proposeDelta:           durationPtr(time.Second * 5),
				vote:                   durationPtr(time.Second * 6),
				voteDelta:              durationPtr(time.Second * 7),
				commit:                 durationPtr(time.Second * 8),
				evidenceMaxAgeDuration: durationPtr(time.Second * 9),
				appVersion:             1,
				abciExtensionHeight:    2,
			}),
		},
		{
			name: "#10 Change bypassCommitTimeout",
			params: makeParams(makeParamsArgs{
				blockBytes:             4,
				blockGas:               6,
				recheck:                true,
				evidenceAge:            5,
				maxEvidenceBytes:       4,
				pubkeyTypes:            []string{"type"},
				precision:              time.Second * 2,
				messageDelay:           time.Second * 3,
				bypassCommitTimeout:    false,
				propose:                durationPtr(time.Second * 4),
				proposeDelta:           durationPtr(time.Second * 5),
				vote:                   durationPtr(time.Second * 6),
				voteDelta:              durationPtr(time.Second * 7),
				commit:                 durationPtr(time.Second * 8),
				evidenceMaxAgeDuration: durationPtr(time.Second * 9),
				appVersion:             1,
				abciExtensionHeight:    2,
			}),
		},
		{
			name: "#11 Change propose",
			params: makeParams(makeParamsArgs{
				blockBytes:             4,
				blockGas:               6,
				recheck:                true,
				evidenceAge:            5,
				maxEvidenceBytes:       4,
				pubkeyTypes:            []string{"type"},
				precision:              time.Second * 2,
				messageDelay:           time.Second * 3,
				bypassCommitTimeout:    true,
				propose:                durationPtr(time.Second * 5),
				proposeDelta:           durationPtr(time.Second * 5),
				vote:                   durationPtr(time.Second * 6),
				voteDelta:              durationPtr(time.Second * 7),
				commit:                 durationPtr(time.Second * 8),
				evidenceMaxAgeDuration: durationPtr(time.Second * 9),
				appVersion:             1,
				abciExtensionHeight:    2,
			}),
		},
		{
			name: "#12 Change proposeDelta",
			params: makeParams(makeParamsArgs{
				blockBytes:             4,
				blockGas:               6,
				recheck:                true,
				evidenceAge:            5,
				maxEvidenceBytes:       4,
				pubkeyTypes:            []string{"type"},
				precision:              time.Second * 2,
				messageDelay:           time.Second * 3,
				bypassCommitTimeout:    true,
				propose:                durationPtr(time.Second * 4),
				proposeDelta:           durationPtr(time.Second * 6),
				vote:                   durationPtr(time.Second * 6),
				voteDelta:              durationPtr(time.Second * 7),
				commit:                 durationPtr(time.Second * 8),
				evidenceMaxAgeDuration: durationPtr(time.Second * 9),
				appVersion:             1,
				abciExtensionHeight:    2,
			}),
		},
		{
			name: "#13 Change vote",
			params: makeParams(makeParamsArgs{
				blockBytes:             4,
				blockGas:               6,
				recheck:                true,
				evidenceAge:            5,
				maxEvidenceBytes:       4,
				pubkeyTypes:            []string{"type"},
				precision:              time.Second * 2,
				messageDelay:           time.Second * 3,
				bypassCommitTimeout:    true,
				propose:                durationPtr(time.Second * 4),
				proposeDelta:           durationPtr(time.Second * 5),
				vote:                   durationPtr(time.Second * 7),
				voteDelta:              durationPtr(time.Second * 7),
				commit:                 durationPtr(time.Second * 8),
				evidenceMaxAgeDuration: durationPtr(time.Second * 9),
				appVersion:             1,
				abciExtensionHeight:    2,
			}),
		},
		{
			name: "#14 Change voteDelta",
			params: makeParams(makeParamsArgs{
				blockBytes:             4,
				blockGas:               6,
				recheck:                true,
				evidenceAge:            5,
				maxEvidenceBytes:       4,
				pubkeyTypes:            []string{"type"},
				precision:              time.Second * 2,
				messageDelay:           time.Second * 3,
				bypassCommitTimeout:    true,
				propose:                durationPtr(time.Second * 4),
				proposeDelta:           durationPtr(time.Second * 5),
				vote:                   durationPtr(time.Second * 6),
				voteDelta:              durationPtr(time.Second * 8),
				commit:                 durationPtr(time.Second * 8),
				evidenceMaxAgeDuration: durationPtr(time.Second * 9),
				appVersion:             1,
				abciExtensionHeight:    2,
			}),
		},
		{
			name: "#15 Change commit",
			params: makeParams(makeParamsArgs{
				blockBytes:             4,
				blockGas:               6,
				recheck:                true,
				evidenceAge:            5,
				maxEvidenceBytes:       4,
				pubkeyTypes:            []string{"type"},
				precision:              time.Second * 2,
				messageDelay:           time.Second * 3,
				bypassCommitTimeout:    true,
				propose:                durationPtr(time.Second * 4),
				proposeDelta:           durationPtr(time.Second * 5),
				vote:                   durationPtr(time.Second * 6),
				voteDelta:              durationPtr(time.Second * 7),
				commit:                 durationPtr(time.Second * 9),
				evidenceMaxAgeDuration: durationPtr(time.Second * 9),
				appVersion:             1,
				abciExtensionHeight:    2,
			}),
		},
		{
			name: "#16 Change evidenceMaxAgeDuration",
			params: makeParams(makeParamsArgs{
				blockBytes:             4,
				blockGas:               6,
				recheck:                true,
				evidenceAge:            5,
				maxEvidenceBytes:       4,
				pubkeyTypes:            []string{"type"},
				precision:              time.Second * 2,
				messageDelay:           time.Second * 3,
				bypassCommitTimeout:    true,
				propose:                durationPtr(time.Second * 4),
				proposeDelta:           durationPtr(time.Second * 5),
				vote:                   durationPtr(time.Second * 6),
				voteDelta:              durationPtr(time.Second * 7),
				commit:                 durationPtr(time.Second * 8),
				evidenceMaxAgeDuration: durationPtr(time.Second * 10),
				appVersion:             1,
				abciExtensionHeight:    2,
			}),
		},
		{
			name: "#17 Change appVersion",
			params: makeParams(makeParamsArgs{
				blockBytes:             4,
				blockGas:               6,
				recheck:                true,
				evidenceAge:            5,
				maxEvidenceBytes:       4,
				pubkeyTypes:            []string{"type"},
				precision:              time.Second * 2,
				messageDelay:           time.Second * 3,
				bypassCommitTimeout:    true,
				propose:                durationPtr(time.Second * 4),
				proposeDelta:           durationPtr(time.Second * 5),
				vote:                   durationPtr(time.Second * 6),
				voteDelta:              durationPtr(time.Second * 7),
				commit:                 durationPtr(time.Second * 8),
				evidenceMaxAgeDuration: durationPtr(time.Second * 9),
				appVersion:             2,
				abciExtensionHeight:    2,
			}),
		},
		{
			name: "#18 Change abciExtensionHeight",
			params: makeParams(makeParamsArgs{
				blockBytes:             4,
				blockGas:               6,
				recheck:                true,
				evidenceAge:            5,
				maxEvidenceBytes:       4,
				pubkeyTypes:            []string{"type"},
				precision:              time.Second * 2,
				messageDelay:           time.Second * 3,
				bypassCommitTimeout:    true,
				propose:                durationPtr(time.Second * 4),
				proposeDelta:           durationPtr(time.Second * 5),
				vote:                   durationPtr(time.Second * 6),
				voteDelta:              durationPtr(time.Second * 7),
				commit:                 durationPtr(time.Second * 8),
				evidenceMaxAgeDuration: durationPtr(time.Second * 9),
				appVersion:             1,
				abciExtensionHeight:    3,
			}),
		},
	}

	for _, tc := range testcases {
		tc.hash = tc.params.HashConsensusParams()
		if tc.hash == nil {
			t.Errorf("%s: HashConsensusParams() returned nil", tc.name)
		}
	}

	// make sure there are no duplicates...
	// sort, then check in order for matches
	sort.Slice(testcases, func(i, j int) bool {
		return bytes.Compare(testcases[i].hash, testcases[j].hash) < 0
	})
	for i := 0; i < len(testcases)-1; i++ {
		caseA := testcases[i]
		caseB := testcases[i+1]

		assert.NotEqual(t, caseA.hash, caseB.hash, fmt.Sprintf("case %s hash should not be equal case %s hash", caseA.name, caseB.name))
	}
}

func TestConsensusParamsUpdate(t *testing.T) {
	testCases := []struct {
		initialParams ConsensusParams
		updates       *tmproto.ConsensusParams
		updatedParams ConsensusParams
	}{
		// empty updates
		{
			initialParams: makeParams(makeParamsArgs{blockBytes: 1, blockGas: 2, evidenceAge: 3}),
			updates:       &tmproto.ConsensusParams{},
			updatedParams: makeParams(makeParamsArgs{blockBytes: 1, blockGas: 2, evidenceAge: 3}),
		},
		{
			// update synchrony params
			initialParams: makeParams(makeParamsArgs{evidenceAge: 3, precision: time.Second, messageDelay: 3 * time.Second}),
			updates: &tmproto.ConsensusParams{
				Synchrony: &tmproto.SynchronyParams{
					Precision:    durationPtr(time.Second * 2),
					MessageDelay: durationPtr(time.Second * 4),
				},
			},
			updatedParams: makeParams(makeParamsArgs{evidenceAge: 3, precision: 2 * time.Second, messageDelay: 4 * time.Second}),
		},
		{
			// update timeout params
			initialParams: makeParams(makeParamsArgs{
				abciExtensionHeight: 1,
			}),
			updates: &tmproto.ConsensusParams{
				Abci: &tmproto.ABCIParams{
					VoteExtensionsEnableHeight: 10,
				},
			},
			updatedParams: makeParams(makeParamsArgs{
				abciExtensionHeight: 10,
			}),
		},
		{
			// update timeout params
			initialParams: makeParams(makeParamsArgs{
				propose:             durationPtr(3 * time.Second),
				proposeDelta:        durationPtr(500 * time.Millisecond),
				vote:                durationPtr(time.Second),
				voteDelta:           durationPtr(500 * time.Millisecond),
				commit:              durationPtr(time.Second),
				bypassCommitTimeout: false,
			}),
			updates: &tmproto.ConsensusParams{
				Timeout: &tmproto.TimeoutParams{
					Propose:             durationPtr(2 * time.Second),
					ProposeDelta:        durationPtr(400 * time.Millisecond),
					Vote:                durationPtr(5 * time.Second),
					VoteDelta:           durationPtr(400 * time.Millisecond),
					Commit:              durationPtr(time.Minute),
					BypassCommitTimeout: true,
				},
			},
			updatedParams: makeParams(makeParamsArgs{
				propose:             durationPtr(2 * time.Second),
				proposeDelta:        durationPtr(400 * time.Millisecond),
				vote:                durationPtr(5 * time.Second),
				voteDelta:           durationPtr(400 * time.Millisecond),
				commit:              durationPtr(time.Minute),
				bypassCommitTimeout: true,
			}),
		},
		// fine updates
		{
			initialParams: makeParams(makeParamsArgs{blockBytes: 1, blockGas: 2, evidenceAge: 3}),
			updates: &tmproto.ConsensusParams{
				Block: &tmproto.BlockParams{
					MaxBytes: 100,
					MaxGas:   200,
				},
				Evidence: &tmproto.EvidenceParams{
					MaxAgeNumBlocks: 300,
					MaxAgeDuration:  time.Duration(300),
					MaxBytes:        50,
				},
				Validator: &tmproto.ValidatorParams{
					PubKeyTypes: valSecp256k1,
				},
			},
			updatedParams: makeParams(makeParamsArgs{
				blockBytes: 100, blockGas: 200,
				evidenceAge:      300,
				maxEvidenceBytes: 50,
				pubkeyTypes:      valSecp256k1}),
		},
		{
			initialParams: makeParams(makeParamsArgs{blockBytes: 1, blockGas: 2, evidenceAge: 3}),
			updates: &tmproto.ConsensusParams{
				Block: &tmproto.BlockParams{
					MaxBytes: 100,
					MaxGas:   200,
				},
				Evidence: &tmproto.EvidenceParams{
					MaxAgeNumBlocks: 300,
					MaxAgeDuration:  time.Duration(300),
					MaxBytes:        50,
				},
				Validator: &tmproto.ValidatorParams{
					PubKeyTypes: valSr25519,
				},
			},
			updatedParams: makeParams(makeParamsArgs{
				blockBytes:       100,
				blockGas:         200,
				evidenceAge:      300,
				maxEvidenceBytes: 50,
				pubkeyTypes:      valSr25519}),
		},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.updatedParams, tc.initialParams.UpdateConsensusParams(tc.updates))
	}
}

func TestConsensusParamsUpdate_AppVersion(t *testing.T) {
	params := makeParams(makeParamsArgs{blockBytes: 1, blockGas: 2, evidenceAge: 3})

	assert.EqualValues(t, 0, params.Version.AppVersion)

	updated := params.UpdateConsensusParams(
		&tmproto.ConsensusParams{Version: &tmproto.VersionParams{AppVersion: 1}})

	assert.EqualValues(t, 1, updated.Version.AppVersion)
}

func TestConsensusParamsUpdate_VoteExtensionsEnableHeight(t *testing.T) {
	t.Run("set to height but initial height already run", func(*testing.T) {
		initialParams := makeParams(makeParamsArgs{
			abciExtensionHeight: 1,
		})
		update := &tmproto.ConsensusParams{
			Abci: &tmproto.ABCIParams{
				VoteExtensionsEnableHeight: 10,
			},
		}
		require.Error(t, initialParams.ValidateUpdate(update, 1))
		require.Error(t, initialParams.ValidateUpdate(update, 5))
	})
	t.Run("reset to 0", func(t *testing.T) {
		initialParams := makeParams(makeParamsArgs{
			abciExtensionHeight: 1,
		})
		update := &tmproto.ConsensusParams{
			Abci: &tmproto.ABCIParams{
				VoteExtensionsEnableHeight: 0,
			},
		}
		require.Error(t, initialParams.ValidateUpdate(update, 1))
	})
	t.Run("set to height before current height run", func(*testing.T) {
		initialParams := makeParams(makeParamsArgs{
			abciExtensionHeight: 100,
		})
		update := &tmproto.ConsensusParams{
			Abci: &tmproto.ABCIParams{
				VoteExtensionsEnableHeight: 10,
			},
		}
		require.Error(t, initialParams.ValidateUpdate(update, 11))
		require.Error(t, initialParams.ValidateUpdate(update, 99))
	})
	t.Run("set to height after current height run", func(*testing.T) {
		initialParams := makeParams(makeParamsArgs{
			abciExtensionHeight: 300,
		})
		update := &tmproto.ConsensusParams{
			Abci: &tmproto.ABCIParams{
				VoteExtensionsEnableHeight: 99,
			},
		}
		require.NoError(t, initialParams.ValidateUpdate(update, 11))
		require.NoError(t, initialParams.ValidateUpdate(update, 98))
	})
	t.Run("no error when unchanged", func(*testing.T) {
		initialParams := makeParams(makeParamsArgs{
			abciExtensionHeight: 100,
		})
		update := &tmproto.ConsensusParams{
			Abci: &tmproto.ABCIParams{
				VoteExtensionsEnableHeight: 100,
			},
		}
		require.NoError(t, initialParams.ValidateUpdate(update, 500))
	})
	t.Run("updated from 0 to 0", func(t *testing.T) {
		initialParams := makeParams(makeParamsArgs{
			abciExtensionHeight: 0,
		})
		update := &tmproto.ConsensusParams{
			Abci: &tmproto.ABCIParams{
				VoteExtensionsEnableHeight: 0,
			},
		}
		require.NoError(t, initialParams.ValidateUpdate(update, 100))
	})
}

func TestProto(t *testing.T) {
	params := []ConsensusParams{
		makeParams(makeParamsArgs{blockBytes: 4, blockGas: 2, evidenceAge: 3, maxEvidenceBytes: 1}),
		makeParams(makeParamsArgs{blockBytes: 1, blockGas: 4, evidenceAge: 3, maxEvidenceBytes: 1}),
		makeParams(makeParamsArgs{blockBytes: 1, blockGas: 2, evidenceAge: 4, maxEvidenceBytes: 1}),
		makeParams(makeParamsArgs{blockBytes: 2, blockGas: 5, evidenceAge: 7, maxEvidenceBytes: 1}),
		makeParams(makeParamsArgs{blockBytes: 1, blockGas: 7, evidenceAge: 6, maxEvidenceBytes: 1}),
		makeParams(makeParamsArgs{blockBytes: 9, blockGas: 5, evidenceAge: 4, maxEvidenceBytes: 1}),
		makeParams(makeParamsArgs{blockBytes: 7, blockGas: 8, evidenceAge: 9, maxEvidenceBytes: 1}),
		makeParams(makeParamsArgs{blockBytes: 4, blockGas: 6, evidenceAge: 5, maxEvidenceBytes: 1}),
		makeParams(makeParamsArgs{precision: time.Second, messageDelay: time.Minute}),
		makeParams(makeParamsArgs{precision: time.Nanosecond, messageDelay: time.Millisecond}),
		makeParams(makeParamsArgs{abciExtensionHeight: 100}),
		makeParams(makeParamsArgs{abciExtensionHeight: 100}),
		makeParams(makeParamsArgs{
			propose:             durationPtr(2 * time.Second),
			proposeDelta:        durationPtr(400 * time.Millisecond),
			vote:                durationPtr(5 * time.Second),
			voteDelta:           durationPtr(400 * time.Millisecond),
			commit:              durationPtr(time.Minute),
			bypassCommitTimeout: true,
		}),
	}

	for i := range params {
		pbParams := params[i].ToProto()

		oriParams := ConsensusParamsFromProto(pbParams)

		assert.Equal(t, params[i], oriParams)

	}
}

func durationPtr(t time.Duration) *time.Duration {
	return &t
}
