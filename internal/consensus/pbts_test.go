package consensus

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tmtimemocks "github.com/tendermint/tendermint/libs/time/mocks"
	"github.com/tendermint/tendermint/types"
)

func TestProposerWaitTime(t *testing.T) {
	genesisTime, err := time.Parse(time.RFC3339, "2019-03-13T23:00:00Z")
	require.NoError(t, err)
	testCases := []struct {
		name         string
		blockTime    time.Time
		localTime    time.Time
		expectedWait time.Duration
	}{
		{
			name:         "block time greater than local time",
			blockTime:    genesisTime.Add(5 * time.Nanosecond),
			localTime:    genesisTime.Add(1 * time.Nanosecond),
			expectedWait: 4 * time.Nanosecond,
		},
		{
			name:         "local time greater than block time",
			blockTime:    genesisTime.Add(1 * time.Nanosecond),
			localTime:    genesisTime.Add(5 * time.Nanosecond),
			expectedWait: 0,
		},
		{
			name:         "both times equal",
			blockTime:    genesisTime.Add(5 * time.Nanosecond),
			localTime:    genesisTime.Add(5 * time.Nanosecond),
			expectedWait: 0,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			b := types.Block{
				Header: types.Header{
					Time: testCase.blockTime,
				},
			}

			mockSource := new(tmtimemocks.Source)
			mockSource.On("Now").Return(testCase.localTime)

			ti := proposerWaitTime(mockSource, b.Header)
			assert.Equal(t, testCase.expectedWait, ti)
		})
	}
}

func TestProposalTimeout(t *testing.T) {
	genesisTime, err := time.Parse(time.RFC3339, "2019-03-13T23:00:00Z")
	require.NoError(t, err)
	testCases := []struct {
		name              string
		localTime         time.Time
		previousBlockTime time.Time
		precision         time.Duration
		msgDelay          time.Duration
		expectedDuration  time.Duration
	}{
		{
			name:              "MsgDelay + Precision has not quite elapsed",
			localTime:         genesisTime.Add(525 * time.Millisecond),
			previousBlockTime: genesisTime.Add(6 * time.Millisecond),
			precision:         time.Millisecond * 20,
			msgDelay:          time.Millisecond * 500,
			expectedDuration:  1 * time.Millisecond,
		},
		{
			name:              "MsgDelay + Precision equals current time",
			localTime:         genesisTime.Add(525 * time.Millisecond),
			previousBlockTime: genesisTime.Add(5 * time.Millisecond),
			precision:         time.Millisecond * 20,
			msgDelay:          time.Millisecond * 500,
			expectedDuration:  0,
		},
		{
			name:              "MsgDelay + Precision has elapsed",
			localTime:         genesisTime.Add(725 * time.Millisecond),
			previousBlockTime: genesisTime.Add(5 * time.Millisecond),
			precision:         time.Millisecond * 20,
			msgDelay:          time.Millisecond * 500,
			expectedDuration:  0,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			b := types.Block{
				Header: types.Header{
					Time: testCase.previousBlockTime,
				},
			}

			mockSource := new(tmtimemocks.Source)
			mockSource.On("Now").Return(testCase.localTime)

			tp := types.TimestampParams{
				Precision: testCase.precision,
				MsgDelay:  testCase.msgDelay,
			}

			ti := proposalStepWaitingTime(mockSource, b.Header, tp)
			assert.Equal(t, testCase.expectedDuration, ti)
		})
	}
}
