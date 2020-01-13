package lite

import (
	"time"

	"github.com/tendermint/tendermint/types"
)

// AutoClient can auto update itself by fetching headers every N seconds.
type AutoClient struct {
	base         *Client
	updatePeriod time.Duration
	quit         chan struct{}

	trustedHeaders chan *types.SignedHeader
	errs           chan error
}

// NewAutoClient creates a new client and starts a polling goroutine.
func NewAutoClient(base *Client, updatePeriod time.Duration) *AutoClient {
	c := &AutoClient{
		base:           base,
		updatePeriod:   updatePeriod,
		quit:           make(chan struct{}),
		trustedHeaders: make(chan *types.SignedHeader),
		errs:           make(chan error),
	}
	go c.autoUpdate()
	return c
}

// TrustedHeaders returns a channel onto which new trusted headers are posted.
func (c *AutoClient) TrustedHeaders() <-chan *types.SignedHeader {
	return c.trustedHeaders
}

// Err returns a channel onto which errors are posted.
func (c *AutoClient) Errs() <-chan error {
	return c.errs
}

// Stop stops the client.
func (c *AutoClient) Stop() {
	close(c.quit)
}

func (c *AutoClient) autoUpdate() {
	ticker := time.NewTicker(c.updatePeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			lastTrustedHeight, err := c.base.LastTrustedHeight()
			if err != nil {
				c.errs <- err
				continue
			}

			if lastTrustedHeight == -1 {
				// no headers yet => wait
				continue
			}

			h, err := c.base.VerifyHeaderAtHeight(lastTrustedHeight+1, time.Now())
			if err != nil {
				// no header yet or verification error => try again after updatePeriod
				c.errs <- err
				continue
			}
			c.trustedHeaders <- h
		case <-c.quit:
			return
		}
	}
}
