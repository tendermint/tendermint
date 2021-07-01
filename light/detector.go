package light

import (
	"bytes"
	"context"
	"time"

	"github.com/tendermint/tendermint/light/provider"
	"github.com/tendermint/tendermint/types"
)

// compareNewHeaderWithWitness takes the verified header from the primary and compares it with a
// header from a specified witness. The function can return one of three errors:
//
// 1: errConflictingHeaders -> there may have been an attack on this light client
// 2: errBadWitness -> the witness has either not responded, doesn't have the header or has given us an invalid one
//    Note: In the case of an invalid header we remove the witness
// 3: nil -> the hashes of the two headers match
func (c *Client) compareNewHeaderWithWitness(ctx context.Context, errc chan error, h *types.SignedHeader,
	witness provider.Provider, witnessIndex int) {

	lightBlock, err := witness.LightBlock(ctx, h.Height)
	switch err {
	// no error means we move on to checking the hash of the two headers
	case nil:
		break

	// the witness hasn't been helpful in comparing headers, we mark the response and continue
	// comparing with the rest of the witnesses
	case provider.ErrNoResponse, provider.ErrLightBlockNotFound:
		errc <- err
		return

	// the witness' head of the blockchain is lower than the height of the primary. This could be one of
	// two things:
	//    1) The witness is lagging behind
	//    2) The primary may be performing a lunatic attack with a height and time in the future
	case provider.ErrHeightTooHigh:
		// The light client now asks for the latest header that the witness has
		var isTargetHeight bool
		isTargetHeight, lightBlock, err = c.getTargetBlockOrLatest(ctx, h.Height, witness)
		if err != nil {
			errc <- err
			return
		}

		// if the witness caught up and has returned a block of the target height then we can
		// break from this switch case and continue to verify the hashes
		if isTargetHeight {
			break
		}

		// witness' last header is below the primary's header. We check the times to see if the blocks
		// have conflicting times
		if !lightBlock.Time.Before(h.Time) {
			errc <- errConflictingHeaders{Block: lightBlock, WitnessIndex: witnessIndex}
			return
		}

		// the witness is behind. We wait for a period WAITING = 2 * DRIFT + LAG.
		// This should give the witness ample time if it is a participating member
		// of consensus to produce a block that has a time that is after the primary's
		// block time. If not the witness is too far behind and the light client removes it
		time.Sleep(2*c.maxClockDrift + c.maxBlockLag)
		isTargetHeight, lightBlock, err = c.getTargetBlockOrLatest(ctx, h.Height, witness)
		if err != nil {
			errc <- errBadWitness{Reason: err, WitnessIndex: witnessIndex}
			return
		}
		if isTargetHeight {
			break
		}

		// the witness still doesn't have a block at the height of the primary.
		// Check if there is a conflicting time
		if !lightBlock.Time.Before(h.Time) {
			errc <- errConflictingHeaders{Block: lightBlock, WitnessIndex: witnessIndex}
			return
		}

		// Following this request response procedure, the witness has been unable to produce a block
		// that can somehow conflict with the primary's block. We thus conclude that the witness
		// is too far behind and thus we return a no response error.
		//
		// NOTE: If the clock drift / lag has been miscalibrated it is feasible that the light client has
		// drifted too far ahead for any witness to be able provide a comparable block and thus may allow
		// for a malicious primary to attack it
		errc <- provider.ErrNoResponse
		return

	default:
		// all other errors (i.e. invalid block, closed connection or unreliable provider) we mark the
		// witness as bad and remove it
		errc <- errBadWitness{Reason: err, WitnessIndex: witnessIndex}
		return
	}

	if !bytes.Equal(h.Hash(), lightBlock.Hash()) {
		errc <- errConflictingHeaders{Block: lightBlock, WitnessIndex: witnessIndex}
	}

	c.logger.Debug("Matching header received by witness", "height", h.Height, "witness", witnessIndex)
	errc <- nil
}

//// sendEvidence sends evidence to a provider on a best effort basis.
//func (c *Client) sendEvidence(ctx context.Context, ev *types.LightClientAttackEvidence, receiver provider.Provider) {
//	err := receiver.ReportEvidence(ctx, ev)
//	if err != nil {
//		c.logger.Error("Failed to report evidence to provider", "ev", ev, "provider", receiver)
//	}
//}

// getTargetBlockOrLatest gets the latest height, if it is greater than the target height then it queries
// the target height else it returns the latest. returns true if it successfully managed to acquire the target
// height.
func (c *Client) getTargetBlockOrLatest(
	ctx context.Context,
	height int64,
	witness provider.Provider,
) (bool, *types.LightBlock, error) {
	lightBlock, err := witness.LightBlock(ctx, 0)
	if err != nil {
		return false, nil, err
	}

	if lightBlock.Height == height {
		// the witness has caught up to the height of the provider's signed header. We
		// can resume with checking the hashes.
		return true, lightBlock, nil
	}

	if lightBlock.Height > height {
		// the witness has caught up. We recursively call the function again. However in order
		// to avoud a wild goose chase where the witness sends us one header below and one header
		// above the height we set a timeout to the context
		lightBlock, err := witness.LightBlock(ctx, height)
		return true, lightBlock, err
	}

	return false, lightBlock, nil
}
