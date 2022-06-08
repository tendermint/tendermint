package log_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/log"
)

func TestNewDefaultLogger(t *testing.T) {
	testCases := map[string]struct {
		format    string
		level     string
		expectErr bool
	}{
		"invalid format": {
			format:    "foo",
			level:     log.LogLevelInfo,
			expectErr: true,
		},
		"invalid level": {
			format:    log.LogFormatJSON,
			level:     "foo",
			expectErr: true,
		},
		"valid format and level": {
			format:    log.LogFormatJSON,
			level:     log.LogLevelInfo,
			expectErr: false,
		},
	}

	for name, tc := range testCases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			_, err := log.NewDefaultLogger(tc.format, tc.level)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
