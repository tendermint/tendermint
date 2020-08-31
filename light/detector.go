package light

import (
	"bytes"
	"errors"
	"time"

	"github.com/tendermint/tendermint/light/provider"
	"github.com/tendermint/tendermint/types"
)

func (c *Client) detectDivergence(verifiedHeaders []*types.SignedHeader) error {
	c.providerMutex.Lock()
	defer c.providerMutex.Unlock()

	// 1. Make sure AT LEAST ONE witness returns the same header.
	var (
		headerMatched      bool
		lastErrConfHeaders error
		lastVerifiedHeader = verifiedHeaders[len(verifiedHeaders) - 1]
	)

	// TODO: move exponential backoff of request to the provider. This shouldn't be part of the client itself
	for attempt := uint16(1); attempt <= c.maxRetryAttempts; attempt++ {
		if len(c.witnesses) == 0 {
			return errNoWitnesses{}
		}

		// launch one goroutine per witness
		errc := make(chan error, len(c.witnesses))
		for i, witness := range c.witnesses {
			go c.compareNewHeaderWithWitness(errc, h, witness, i)
		}

		witnessesToRemove := make([]int, 0)

		// handle errors as they come
		for i := 0; i < cap(errc); i++ {
			err := <-errc

			switch e := err.(type) {
			case nil: // at least one header matched
				headerMatched = true
			case ErrConflictingHeaders: // fork detected
				c.logger.Info("FORK DETECTED", "witness", e.Witness, "err", err)
				witnessTrace, primaryHeader, err = c.verifyDivergentHeaderAgainstTrace(verifiedHeaders, e.H2, e.Witness)
				if err != nil {
					c.logger.Info("Witness sent us invalid divergent header", "witness", e.Witness)
					continue
				}


				lastErrConfHeaders = e

			case errBadWitness:
				c.logger.Info("Bad witness", "witness", c.witnesses[e.WitnessIndex], "err", err)
				// if witness sent us invalid header, remove it
				// TODO: Move error handling of provider to the provider package
				if e.Code == invalidHeader {
					c.logger.Info("Witness sent us invalid header / vals -> removing it", "witness", c.witnesses[e.WitnessIndex])
					witnessesToRemove = append(witnessesToRemove, e.WitnessIndex)
				}
			}
		}

		for _, idx := range witnessesToRemove {
			c.removeWitness(idx)
		}

		if headerMatched {
			return nil
		}

		// 2. Otherwise, sleep
		time.Sleep(backoffTimeout(attempt))
	}

	return errors.New("awaiting response from all witnesses exceeded dropout time")
}

func (c *Client) compareNewHeaderWithWitness(errc chan error, h *types.SignedHeader,
	witness provider.Provider, witnessIndex int) {

	altH, _, err := c.signedHeaderAndValSetFromWitness(h.Height, witness)
	if err != nil {
		err.WitnessIndex = witnessIndex
		errc <- err
		return
	}

	if !bytes.Equal(h.Hash(), altH.Hash()) {
		errc <- ErrConflictingHeaders{H1: h, Primary: c.primary, H2: altH, Witness: witness}
	}

	errc <- nil
}

// sendConflictingHeadersEvidence sends evidence to all witnesses and primary
// on best effort basis.
//
// Evidence needs to be submitted to all full nodes since there's no way to
// determine which full node is correct (honest).
func (c *Client) sendConflictingHeadersEvidence(ev *types.ConflictingHeadersEvidence) {
	err := c.primary.ReportEvidence(ev)
	if err != nil {
		c.logger.Error("Failed to report evidence to primary", "ev", ev, "primary", c.primary)
	}

	for _, w := range c.witnesses {
		err := w.ReportEvidence(ev)
		if err != nil {
			c.logger.Error("Failed to report evidence to witness", "ev", ev, "witness", w)
		}
	}
}

// returns the trace of the witness between the common header and the witnesses divergent header
// and the lunatic header
func (c *Client) verifyDivergentHeaderAgainstTrace(
	trace []*types.SignedHeader,
	divergentHeader *types.SignedHeader,
	source provider.Provider) ([]*types.SignedHeader, *types.SignedHeader, error) {

}

func (c *Client) checkForLunaticAttack(
	commongHeader, verifiedHeader, divergentHeader *types.SignedHeader) (*types.LunaticValidatorEvidence, error) {

}
