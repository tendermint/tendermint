package cmd

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/log"
)

func init() {
}

func TestParse(t *testing.T) {
	testCases := []struct {
		args       []string
		conditions []string
	}{
		{
			args: []string{
				"parse",
				"--type", "tendermint.abci.Request",
				"--format", formatHex,
				"--input", "4862221220d029205e1750bcadb32f1fece2276054a70f15aeefc55dc1348744769de66caa"},
			conditions: []string{"\"listSnapshots\": {"},
		},
		{
			args: []string{
				"parse",
				"--format", formatHex,
				"--input", "fa0a3aba050a20e4e3f15d7327ee3744dc48af49d5b90e15f606c254980d93ce810dc15522560c12ac03" +
					"0a04080b10011213646173682d6465766e65742d6d656b686f6e6718f803220b089d8999940610ae8a8e3d2a480a20" +
					"d1366b94331fe00e4b0ef14c1fddf2c82fb027b40c1ee9fd0e968b8070b1b0c51224080112202285c8ea033205b319" +
					"0b0de78006595281a6625211475174269e32c14126d98f3220b9ab874257cb4faf81dd57463feb8cb8ec3969284ad2" +
					"869a6c4750f1a4966cb03a20e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855422078" +
					"ef73832b71a20175bbec20673405a829177ad30eb2fb0d6170b79e526c73a54a2078ef73832b71a20175bbec206734" +
					"05a829177ad30eb2fb0d6170b79e526c73a55220048091bc7ddc283f77bfbf91d73c44da58c3df8a9cbc867405d8b7" +
					"f3daada22f5a20d029205e1750bcadb32f1fece2276054a70f15aeefc55dc1348744769de66caa6220e3b0c44298fc" +
					"1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b8556a20e3b0c44298fc1c149afbf4c8996fb92427ae41" +
					"e4649b934ca495991b7852b855a006df2daa0620bc3fd5994e3fdd56dc6168b5e946d9f2c3c088e10e116d97c9482f" +
					"e3963b87981ae6011a20000000e152ef0de997920b1718a91a19fc7da4ca364ddcf35cd805ca36730f48226093637c" +
					"e2edb53bbcef8eb806268a03bdc36a39a4da54b851f897b8d38a6a79e148ea0d85de1684018781a7d3c640f69d19bc" +
					"850bbc91765f94c968697617e9ed99bd2d9a643872ed707cb6f184a067b07639394e28c9aca37db23a1b3ec254e22a" +
					"600662d934751ce171188e6c55f42defc4d5c5c221b809fc2de1366b5e1c5b24f32ceeb0b077edfebed6dac0a7323f" +
					"9aa60209dddc989b0676a227ee6cd490ead09920b07c5f45039c353f0ebd231a8858b503928004a91bd70062005254" +
					"d16ef2041200",
			},
			conditions: []string{
				"beginBlock",
				"\"nextValidatorsHash\": \"eO9zgytxogF1u+wgZzQFqCkXetMOsvsNYXC3nlJsc6U=\"",
			},
		},
		{
			args: []string{
				"--format", formatBase64,
				"--type", "tendermint.abci.Response",
				"--input", `mAc6yQMIAyLEA29tZHRaWE56WVdkbGVCMURhR0ZwYmt4dlkyc2dkbVZ5YVdacFkyRjBhVzl1SUda
aGFXeGxaR1JrWVhSaG8yWm9aV2xuYUhRYUpHRUxYV2xpYkc5amEwaGhjMmg0UURJMU1UYzNNemd3
WkRjeU1qZGpPRGN5TldGbU5ESmxNVGxrTVdGak5EZGhZV1ZpTWpaa09UTTJZalF3TXpRMU1EQXdN
REF4TlRJM1pUQm1OalE1TnpWcGMybG5ibUYwZFhKbGVNQTVZVFptTm1VeE1XVXlNREF3TURBd01U
TmtabUZpTkRaak1tRXpNV0UyWkdSbFpHUmlZbU5qTnpRM09UTXpNekJsT0RJMU1UbGlNVFppTkdR
eU1qWXdZV1ZpTURVNU1XVmlNakkzTkRBeFpqZGpNVEl4T1RVMk5XWXdNekZsWkRnME16UTBOalZq
TmpreFpqTTRZMkU1TVRaaFptSTVaRGxtWXpWaVpqSXdaR000T0RNeE1qZGhZbUkzTVdSbU5URTFN
ekkzWldRek1HSXhaVEkyWTJJMFpUTmxOalJtTjJGbU5XWTBPREpoT0RCaFl6QXlPRE00TmpSa05q
WTJaREk9
`,
			},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			var err error

			parseCmd := &ParseCmd{}
			cmd := parseCmd.Command()
			cmd.SetArgs(tc.args)

			outBuf := &bytes.Buffer{}
			cmd.SetOut(outBuf)

			errBuf := &bytes.Buffer{}
			cmd.SetErr(errBuf)
			logger, err = log.NewLogger(log.LogLevelDebug, errBuf)
			require.NoError(t, err)

			err = cmd.Execute()
			assert.NoError(t, err)
			assert.Equal(t, 0, errBuf.Len(), errBuf.String())

			s := outBuf.String()

			for _, condition := range tc.conditions {
				assert.Contains(t, s, condition)
			}

			t.Log(s)
		})
	}
}
