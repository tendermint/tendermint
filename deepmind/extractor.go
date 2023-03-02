package deepmind

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/figment-networks/extractor-cosmos"
	"github.com/tendermint/tendermint/types"
)

var (
	enabled            bool
	currentBlockHeight int64
	inBlock            bool
)

func Enable() {
	enabled = true
}

func IsEnabled() bool {
	return enabled
}

func Disable() {
	enabled = false
}

func Initialize(config *extractor.Config) {
	extractor.SetWriterFromConfig(config)
	enabled = true
}

func Shutdown(ctx context.Context) {
	defer func() {
		enabled = false
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.Tick(time.Second):
			if !inBlock {
				return
			}
		}
	}
}

func Abort(err error) {
	panic(err)
}

func BeginBlock(height int64) error {
	if !enabled {
		return nil
	}

	if height == currentBlockHeight {
		panic("cannot initialize the same height more than once")
	}

	currentBlockHeight = height
	inBlock = true

	err := extractor.SetHeight(height)
	if err != nil {
		return err
	}
	return extractor.WriteLine(extractor.MsgBegin, "%d", height)
}

func FinalizeBlock(height int64) error {
	if !enabled {
		return nil
	}

	if height != currentBlockHeight {
		panic("finalize block on invalid height")
	}

	inBlock = false

	return extractor.WriteLine(extractor.MsgEnd, "%d", height)
}

func AddBlockData(data types.EventDataNewBlock) error {
	if !enabled {
		return nil
	}

	buff, err := encodeBlock(data)
	if err != nil {
		return err
	}

	return extractor.WriteLine(extractor.MsgBlock, "%s", base64.StdEncoding.EncodeToString(buff))
}

func AddBlockHeaderData(data types.EventDataNewBlockHeader) error {
	if !enabled {
		return nil
	}
	// Skipped for now
	return nil
}

func AddEvidenceData(data types.EventDataNewEvidence) error {
	if !enabled {
		return nil
	}
	// Skipped for now
	return nil
}

func AddTransactionData(tx types.EventDataTx) error {
	if !enabled {
		return nil
	}

	data, err := encodeTx(&tx.TxResult)
	if err != nil {
		return err
	}

	return extractor.WriteLine(extractor.MsgTx, "%s", base64.StdEncoding.EncodeToString(data))
}

func AddValidatorSetUpdatesData(updates types.EventDataValidatorSetUpdates) error {
	if !enabled {
		return nil
	}

	data, err := encodeValidatorSetUpdates(&updates)
	if err != nil {
		return err
	}

	return extractor.WriteLine(extractor.MsgValidatorSetUpdate, "%s", base64.StdEncoding.EncodeToString(data))
}
