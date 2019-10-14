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
}

// TrustedHeaders returns a channel onto which new trusted headers are posted.
func (c *Client) TrustedHeaders() <-chan *types.SignedHeader {
	return c.trustedHeaders
}

// Err returns a channel onto which errors are posted.
func (c *Client) Err() <-chan error {
	return c.err
}

// Stop stops the client.
func (c *Client) Stop() {
	close(c.quit)
}

func (c *Client) autoUpdate() {
	ticker := time.NewTicker(c.updatePeriod)
	for {
		select {
		case <-ticker.C:
			h, err := c.base.VerifyNextHeader()
			if err != nil {
				c.err <- err
				continue
			}
			c.trustedHeaders <- h
		case <-c.quit:
			return
		}
	}
}
