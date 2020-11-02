package mbt

import (
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/light"
	"github.com/tendermint/tendermint/types"
)

const jsonDir = "./json"

func TestVerify(t *testing.T) {
	filenames := jsonFilenames(t)

	for _, filename := range filenames {
		filename := filename
		t.Run(filename, func(t *testing.T) {

			jsonBlob, err := ioutil.ReadFile(filename)
			if err != nil {
				t.Fatal(err)
			}

			var tc testCase
			err = tmjson.Unmarshal(jsonBlob, &tc)
			if err != nil {
				t.Fatal(err)
			}

			t.Log(tc.Description)

			var (
				trustedSignedHeader = tc.Initial.SignedHeader
				trustedNextVals     = tc.Initial.NextValidatorSet
				trustingPeriod      = time.Duration(tc.Initial.TrustingPeriod) * time.Nanosecond
			)

			for _, input := range tc.Input {
				var (
					newSignedHeader = input.LightBlock.SignedHeader
					newVals         = input.LightBlock.ValidatorSet
				)

				err = light.Verify(
					&trustedSignedHeader,
					&trustedNextVals,
					newSignedHeader,
					newVals,
					trustingPeriod,
					input.Now,
					1*time.Second,
					light.DefaultTrustLevel,
				)

				t.Logf("%d -> %d", trustedSignedHeader.Height, newSignedHeader.Height)

				switch input.Verdict {
				case "SUCCESS":
					require.NoError(t, err)
				case "NOT_ENOUGH_TRUST":
					require.IsType(t, light.ErrNewValSetCantBeTrusted{}, err)
				case "INVALID":
					switch err.(type) {
					case light.ErrOldHeaderExpired:
					case light.ErrInvalidHeader:
					default:
						t.Fatalf("expected either ErrInvalidHeader or ErrOldHeaderExpired, but got %v", err)
					}
				default:
					t.Fatalf("unexpected verdict: %q", input.Verdict)
				}

				if err == nil { // advance
					trustedSignedHeader = *newSignedHeader
					trustedNextVals = *input.LightBlock.NextValidatorSet
				}
			}
		})
	}
}

// jsonFilenames returns a list of files in jsonDir directory
func jsonFilenames(t *testing.T) []string {
	matches, err := filepath.Glob(filepath.Join(jsonDir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	return matches
}

type testCase struct {
	Description string      `json:"description"`
	Initial     initialData `json:"initial"`
	Input       []inputData `json:"input"`
}

type initialData struct {
	SignedHeader     types.SignedHeader `json:"signed_header"`
	NextValidatorSet types.ValidatorSet `json:"next_validator_set"`
	TrustingPeriod   uint64             `json:"trusting_period"`
	Now              time.Time          `json:"now"`
}

type inputData struct {
	LightBlock lightBlockWithNextValidatorSet `json:"block"`
	Now        time.Time                      `json:"now"`
	Verdict    string                         `json:"verdict"`
}

// In tendermint-rs, NextValidatorSet is used to verify new blocks (opposite to
// Go tendermint).
type lightBlockWithNextValidatorSet struct {
	*types.SignedHeader `json:"signed_header"`
	ValidatorSet        *types.ValidatorSet `json:"validator_set"`
	NextValidatorSet    *types.ValidatorSet `json:"next_validator_set"`
}
