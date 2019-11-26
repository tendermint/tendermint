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
	err            chan error
}

// NewAutoClient creates a new client and starts a polling goroutine.
func NewAutoClient(base *Client, updatePeriod time.Duration) *AutoClient {
	c := &AutoClient{
		base:           base,
		updatePeriod:   updatePeriod,
		quit:           make(chan struct{}),
		trustedHeaders: make(chan *types.SignedHeader),
		err:            make(chan error),
	}
	go c.autoUpdate()
	return c
}

// TrustedHeaders returns a channel onto which new trusted headers are posted.
func (c *AutoClient) TrustedHeaders() <-chan *types.SignedHeader {
	return c.trustedHeaders
}

// Err returns a channel onto which errors are posted.
func (c *AutoClient) Err() <-chan error {
	return c.err
}

// Stop stops the client.
func (c *AutoClient) Stop() {
	close(c.quit)
}

func (c *AutoClient) autoUpdate() {
	lastTrustedHeight, err := c.base.LastTrustedHeight()
	if err != nil {
		c.err <- err
		return
	}

	ticker := time.NewTicker(c.updatePeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := c.base.VerifyHeaderAtHeight(lastTrustedHeight+1, time.Now())
			if err != nil {
				c.err <- err
				continue
			}
			h, err := c.base.TrustedHeader(lastTrustedHeight+1, time.Now())
			if err != nil {
				c.err <- err
				continue
			}
			c.trustedHeaders <- h
			lastTrustedHeight = h.Height
		case <-c.quit:
			return
		}
	}
}
