package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/bits"
)

func TestCopy(t *testing.T) {
	t.Run("VerifyShallowCopy", func(t *testing.T) {
		prsOne := PeerRoundState{}
		prsOne.Prevotes = bits.NewBitArray(12)
		prsTwo := prsOne

		prsOne.Prevotes.SetIndex(1, true)

		require.Equal(t, prsOne.Prevotes, prsTwo.Prevotes)
	})
	t.Run("DeepCopy", func(t *testing.T) {
		prsOne := PeerRoundState{}
		prsOne.Prevotes = bits.NewBitArray(12)
		prsTwo := prsOne.Copy()

		prsOne.Prevotes.SetIndex(1, true)

		require.NotEqual(t, prsOne.Prevotes, prsTwo.Prevotes)
	})
}
