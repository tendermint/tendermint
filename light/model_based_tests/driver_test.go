package model_based_tests

import (
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/light"
	"github.com/tendermint/tendermint/types"
)

const jsonDir = "./json"

func TestVerify(t *testing.T) {
	filenames := jsonFilenames(t)

	for _, filename := range filenames {
		t.Logf("-> %s", filename)

		jsonBlob, err := ioutil.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}

		var tc testCase
		err = tmjson.Unmarshal(jsonBlob, &tc)
		if err != nil {
			t.Fatal(err)
		}

		var (
			// chainID             = tc.Initial.SignedHeader.Header.ChainID
			trustedSignedHeader = tc.Initial.SignedHeader
			trustedNextVals     = tc.Initial.NextValidatorSet
			trustingPeriod      = time.Duration(tc.Initial.TrustingPeriod) * time.Microsecond
			// now                 = tc.Initial.Now
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
			if err != nil {
				t.Logf("TRUSTED HEADER %v\nNEW HEADER %v", trustedSignedHeader, newSignedHeader)

				switch input.Verdict {
				case "SUCCESS":
					t.Fatalf("unexpected error: %v", err)
				case "NOT_ENOUGH_TRUST":
					if _, ok := err.(light.ErrNewValSetCantBeTrusted); !ok {
						t.Fatalf("expected ErrNewValSetCantBeTrusted, but got %v", err)
					}
				case "INVALID":
					if _, ok := err.(light.ErrInvalidHeader); !ok {
						t.Fatalf("expected ErrInvalidHeader, but got %v", err)
					}
				default:
					t.Fatalf("unexpected verdict: %q", input.Verdict)
				}
			} else {
				trustedSignedHeader = *newSignedHeader
				trustedNextVals = *newVals
			}
		}
	}

}

// func TestBisection(t *testing.T) {
// 	tests, err := getTestPaths("./json/bisection/")
// 	if err != nil {
// 		fmt.Println(err)
// 	}

// 	for _, test := range tests {

// 		// we skip this one for now because the current version (v0.33.6)
// 		// does not panic on receiving conflicting commits from witnesses
// 		skippedTest := "json/bisection/multi_peer/conflicting_valid_commits_from_one_of_the_witnesses.json"
// 		if test == skippedTest {
// 			fmt.Printf("\ntest case skipped: %v", skippedTest)
// 			continue
// 		}

// 		data := generator.ReadFile(test)

// 		cdc := amino.NewCodec()
// 		cryptoAmino.RegisterAmino(cdc)

// 		cdc.RegisterInterface((*provider.Provider)(nil), nil)
// 		cdc.RegisterConcrete(generator.MockProvider{}, "com.tendermint/MockProvider", nil)

// 		var testBisection generator.TestBisection
// 		e := cdc.UnmarshalJSON(data, &testBisection)
// 		if e != nil {
// 			fmt.Printf("error: %v", e)
// 		}

// 		fmt.Println(testBisection.Description)

// 		trustedStore := dbs.New(dbm.NewMemDB(), testBisection.Primary.ChainID())
// 		witnesses := testBisection.Witnesses
// 		trustOptions := lite.TrustOptions{
// 			Period: testBisection.TrustOptions.Period,
// 			Height: testBisection.TrustOptions.Height,
// 			Hash:   testBisection.TrustOptions.Hash,
// 		}
// 		trustLevel := testBisection.TrustOptions.TrustLevel
// 		expectedOutput := testBisection.ExpectedOutput

// 		client, e := lite.NewClient(
// 			testBisection.Primary.ChainID(),
// 			trustOptions,
// 			testBisection.Primary,
// 			witnesses,
// 			trustedStore,
// 			lite.SkippingVerification(trustLevel))
// 		if e != nil {
// 			fmt.Println(e)
// 		}

// 		height := testBisection.HeightToVerify
// 		_, e = client.VerifyHeaderAtHeight(height, testBisection.Now)
// 		// ---
// 		fmt.Println(e)
// 		// ---
// 		err := e != nil
// 		expectsError := expectedOutput == "error"
// 		if (err && !expectsError) || (!err && expectsError) {
// 			t.Errorf("\n Failing test: %s \n Error: %v \n Expected error: %v", testBisection.Description, e, testBisection.ExpectedOutput)

// 		}
// 	}
// }

func jsonFilenames(t *testing.T) []string {
	names := make([]string, 0)

	files, err := ioutil.ReadDir(jsonDir)
	if err != nil {
		t.Fatal(err)
	}

	base := filepath.Base(jsonDir)

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".json" {
			names = append(names, filepath.Join(base, file.Name()))
		}
	}

	return names
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
	LightBlock types.LightBlock `json:"block"`
	Now        time.Time        `json:"now"`
	Verdict    string           `json:"verdict"`
}
