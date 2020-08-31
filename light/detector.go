package light

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/light/provider"
	"github.com/tendermint/tendermint/types"
)

// The detector component of the light client detect and handles attacks on the light client.
// More info here:
// tendermint/docs/architecture/adr-047-handling-evidence-from-light-client.md

// detectDivergence is a second wall of defense for the light client and is used
// only in the case of skipping verification which employs the trust level mechanism.
//
// It takes the target verified header and compares it with the headers of a set of
// witness providers that the light client is connected to. If a conflicting header
// is returned it verifies and examines the conflicting header against the verified
// trace that was produced from the primary. If successful it produces two sets of evidence
// and sends them to the opposite provider before halting.
//
// If there are no conflictinge headers, the light client deems the verified target header
// trusted and saves it to the trusted store.
func (c *Client) detectDivergence(primaryTrace []*types.LightBlock, now time.Time) error {
	var (
		headerMatched      bool
		lastErrConfHeaders error
		lastVerifiedHeader = primaryTrace[len(primaryTrace)-1].SignedHeader
		witnessesToRemove  = make([]int, 0)
	)

	c.providerMutex.Lock()
	defer c.providerMutex.Unlock()

	if len(c.witnesses) == 0 {
		return errNoWitnesses{}
	}

	// launch one goroutine per witness to retrieve the light block of the target height
	// and compare it with the header from the primary
	errc := make(chan error, len(c.witnesses))
	for i, witness := range c.witnesses {
		go c.compareNewHeaderWithWitness(errc, lastVerifiedHeader, witness, i)
	}

	// handle errors from the header comparisons as they come in
	for i := 0; i < cap(errc); i++ {
		err := <-errc

		switch e := err.(type) {
		case nil: // at least one header matched
			headerMatched = true
		case errConflictingHeaders:
			// We have conflicting headers. This could possibly imply an attack on the light client.
			// First we need to verify the witness's header using the same skipping verification and then we
			// need to find the point that the headers diverge and examine this for any evidence of an attack.
			//
			// We combine these actions together, verifying the witnesses headers and outputting the trace
			// which captures the bifurcation point and if successful provides the information to create
			witnessTrace, primaryBlock, err := c.examineConflictingHeaderAgainstTrace(primaryTrace, e.Block.SignedHeader, e.Witness, now)
			if err != nil {
				c.logger.Info("Error validating witness's divergent header", "witness", e.Witness, "err", err)
				witnessesToRemove = append(witnessesToRemove, e.Index)
				continue
			}
			// We are suspecting that the primary is faulty, hence we hold the witness as the source of truth
			// and generate evidence against the primary that we can send to the witness
			ev := createEvidence(witnessTrace[0], witnessTrace[len(witnessTrace)-1], primaryBlock)
			c.sendEvidence(ev, e.Witness)
			lastErrConfHeaders = e

			// This may not be valid because the witness itself is at fault. So now we reverse it, examining the
			// trace provided by the witness and holding the primary as the source of truth. Note: primary may not
			// respond but this is okay as we will halt anyway.
			primaryTrace, witnessBlock, err := c.examineConflictingHeaderAgainstTrace(witnessTrace, primaryBlock.SignedHeader, c.primary, now)
			if err != nil {
				c.logger.Info("Error validating primary's divergent header", "primary", c.primary, "err", err)
				continue
			}

			// We now use the primary trace to create evidence against the witness and send it to the primary
			ev = createEvidence(primaryTrace[0], primaryTrace[len(primaryTrace)-1], witnessBlock)
			c.sendEvidence(ev, c.primary)

		case errBadWitness:
			c.logger.Info("Witness returned an error during header comparison", "witness", c.witnesses[e.Index], "err", err)
			// if witness sent us an invalid header, then remove it. If it didn't respond or couldn't find the block, then we
			// ignore it and move on to the next witness
			if _, ok := e.Reason.(provider.ErrBadLightBlock); ok {
				c.logger.Info("Witness sent us invalid header / vals -> removing it", "witness", c.witnesses[e.Index])
				witnessesToRemove = append(witnessesToRemove, e.Index)
			}
		}
	}

	for _, idx := range witnessesToRemove {
		c.removeWitness(idx)
	}

	// 1. If we saw conflicting headers we still pass the error back to the user
	if lastErrConfHeaders != nil {
		return lastErrConfHeaders
	}

	// 2. Else if we had at least one witness that returned the same header then we
	// conclude that we can trust the header
	if headerMatched {
		return nil
	}

	// 3. All witnesses have either not responded, don't have the block or sent invalid blocks.
	return errors.New("All witnesses have either not responded, don't have the block or sent invalid blocks." +
		" You should look to change your witnesses or review the light client's logs for more information.")

}

// compareNewHeaderWithWitness takes the verified header from the primary and compares it with a
// header from a specified witness. The function can return one of three errors:
//
// 1: errConflictingHeaders -> there may have been an attack on this light client
// 2: errBadWitness -> the witness has either not responded, doesn't have the header or has given us an invalid one
//    Note: In the case of an invalid header we remove the witness
// 3: nil -> the hashes of the two headers match
func (c *Client) compareNewHeaderWithWitness(errc chan error, h *types.SignedHeader,
	witness provider.Provider, witnessIndex int) {

	lightBlock, err := witness.LightBlock(h.Height)
	if err != nil {
		errc <- errBadWitness{Reason: err, Index: witnessIndex}
		return
	}

	if !bytes.Equal(h.Hash(), lightBlock.Hash()) {
		errc <- errConflictingHeaders{Block: lightBlock, Witness: witness, Index: witnessIndex}
	}

	errc <- nil
}

// sendEvidence sends evidence to a provider on a best effort basis.
func (c *Client) sendEvidence(ev *types.LightClientAttackEvidence, receiver provider.Provider) {
	err := receiver.ReportEvidence(ev)
	if err != nil {
		c.logger.Error("Failed to report evidence to provider", "ev", ev, "provider", receiver)
	}
}

// examineConflictingHeaderAgainstTrace takes a trace from one provider and a divergent header that
// it has received from another and preforms verifySkipping at the heights of each of the intermediate
// headers in the trace until it reaches the divergentHeader. 1 of 2 things can happen.
//
// 1. The light client verifies a header that is different to the intermediate header in the trace. This
//    is the bifurcation point and the light client can create evidence from it
// 2. The source stops responding, doesn't have the block or sends an invalid header in which case we
//    return the error and remove the witness
func (c *Client) examineConflictingHeaderAgainstTrace(
	trace []*types.LightBlock,
	divergentHeader *types.SignedHeader,
	source provider.Provider, now time.Time) ([]*types.LightBlock, *types.LightBlock, error) {

	var previouslyVerifiedBlock *types.LightBlock

	for idx, traceBlock := range trace {
		// The first block in the trace MUST be the same to the light block that the source produces
		// else we cannot continue with verification.
		sourceBlock, err := source.LightBlock(traceBlock.Height)
		if err != nil {
			return nil, nil, err
		}

		if idx == 0 {
			if shash, thash := sourceBlock.Hash(), traceBlock.Hash(); !bytes.Equal(shash, thash) {
				return nil, nil, fmt.Errorf("trusted block is different to the source's first block (%X = %X)",
					thash, shash)
			}
			previouslyVerifiedBlock = sourceBlock
			continue
		}

		// we check that the source provider can verify a block at the same height of the
		// intermediate height
		trace, err := c.verifySkipping(source, previouslyVerifiedBlock, sourceBlock, now)
		if err != nil {
			return nil, nil, fmt.Errorf("verifySkipping of conflicting header failed: %w", err)
		}
		// check if the headers verified by the source has diverged from the trace
		if shash, thash := sourceBlock.Hash(), traceBlock.Hash(); !bytes.Equal(shash, thash) {
			// Bifurcation point found!
			return trace, traceBlock, nil
		}

		// headers are still the same. update the previouslyVerifiedBlock
		previouslyVerifiedBlock = sourceBlock
	}

	// We have reached the end of the trace without observing a divergence. The last header  is thus different
	// from the divergent header that the source originally sent us, then we return an error.
	return nil, nil, fmt.Errorf("source provided different header to the original and is the same as the targer"+
		"header of the trace (%X != %X = %X)",
		previouslyVerifiedBlock.Hash(), divergentHeader.Hash(), trace[len(trace)-1].Hash())

}

// isInvalidHeader takes a trusted header and matches it againt a conflicting header
// to determine whether the conflicting header was the product of a valid state transition
// or not. If it is then all the deterministic fields of the header should be the same.
// If not, it is an invalid header and constitutes a lunatic attack.
func isInvalidHeader(trusted, conflicting *types.Header) bool {
	return bytes.Equal(trusted.ValidatorsHash, conflicting.ValidatorsHash) &&
		bytes.Equal(trusted.NextValidatorsHash, conflicting.NextValidatorsHash) &&
		bytes.Equal(trusted.ConsensusHash, conflicting.ConsensusHash) &&
		bytes.Equal(trusted.AppHash, conflicting.AppHash) &&
		bytes.Equal(trusted.LastResultsHash, conflicting.LastResultsHash)
}

// createEvidence forms LightClientAttackEvidence and determines, based on the trusted and conflicting
// header, the type of the attack.
func createEvidence(common, trusted, conflicting *types.LightBlock) *types.LightClientAttackEvidence {
	var attackType types.AttackType

	switch {
	case isInvalidHeader(trusted.Header, conflicting.Header):
		attackType = types.Lunatic
	case trusted.Commit.Round == conflicting.Commit.Round:
		attackType = types.Equivocation
	default:
		attackType = types.Amnesia
	}

	return &types.LightClientAttackEvidence{
		ConflictingBlock: conflicting,
		CommonHeight:     common.Height,
		Type:             attackType,
	}
}
