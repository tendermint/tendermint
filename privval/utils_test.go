package privval

import (
	"fmt"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestIsConnTimeoutForNonTimeoutErrors(t *testing.T) {
	assert.False(t, IsConnTimeout(fmt.Errorf("max retries exceeded: %w", ErrDialRetryMax)))
	assert.False(t, IsConnTimeout(errors.New("completely irrelevant error")))
}
