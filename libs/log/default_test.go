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
		size      int64
		expectErr bool
	}{
		"invalid format": {
			format:    "foo",
			level:     log.LogLevelInfo,
			size:      0,
			expectErr: true,
		},
		"invalid level": {
			format:    log.LogFormatJSON,
			level:     "foo",
			size:      0,
			expectErr: true,
		},
		"valid format and level": {
			format:    log.LogFormatJSON,
			level:     log.LogLevelInfo,
			size:      0,
			expectErr: false,
		},
		"valid log element max length": {
			format:    log.LogFormatJSON,
			level:     log.LogLevelInfo,
			size:      log.MaxLogElementLength,
			expectErr: false,
		},
		"invalid log element length": {
			format:    log.LogFormatJSON,
			level:     log.LogLevelInfo,
			size:      -1,
			expectErr: true,
		},
		"invalid log element length, exceed limit": {
			format:    log.LogFormatJSON,
			level:     log.LogLevelInfo,
			size:      log.MaxLogElementLength + 1,
			expectErr: true,
		},
	}

	for name, tc := range testCases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			_, err := log.NewDefaultLogger(tc.format, tc.level, false, int(tc.size))
			if tc.expectErr {
				require.Error(t, err)
				require.Panics(t, func() {
					_ = log.MustNewDefaultLogger(tc.format, tc.level, false, int(tc.size))
				})
			} else {
				require.NoError(t, err)
			}
		})
	}
}
