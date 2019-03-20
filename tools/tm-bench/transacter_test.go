package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

// This test tests that the output of generate tx and update tx is consistent
func TestGenerateTxUpdateTxConsistentency(t *testing.T) {
	cases := []struct {
		connIndex        int
		startingTxNumber int
		txSize           int
		hostname         string
		numTxsToTest     int
	}{
		{0, 0, 40, "localhost:26657", 1000},
		{70, 300, 10000, "localhost:26657", 1000},
		{0, 50, 100000, "localhost:26657", 1000},
	}

	for tcIndex, tc := range cases {
		hostnameHash := sha256.Sum256([]byte(tc.hostname))
		// Tx generated from update tx. This is defined outside of the loop, since we have
		// to a have something initially to update
		updatedTx := generateTx(tc.connIndex, tc.startingTxNumber, tc.txSize, hostnameHash)
		updatedHex := make([]byte, len(updatedTx)*2)
		hex.Encode(updatedHex, updatedTx)
		for i := 0; i < tc.numTxsToTest; i++ {
			expectedTx := generateTx(tc.connIndex, tc.startingTxNumber+i, tc.txSize, hostnameHash)
			expectedHex := make([]byte, len(expectedTx)*2)
			hex.Encode(expectedHex, expectedTx)

			updateTx(updatedTx, updatedHex, tc.startingTxNumber+i)

			// after first 32 bytes is 8 bytes of time, then purely random bytes
			require.Equal(t, expectedTx[:32], updatedTx[:32],
				"First 32 bytes of the txs differed. tc #%d, i #%d", tcIndex, i)
			require.Equal(t, expectedHex[:64], updatedHex[:64],
				"First 64 bytes of the hex differed. tc #%d, i #%d", tcIndex, i)
			// Test the lengths of the txs are as expected
			require.Equal(t, tc.txSize, len(expectedTx),
				"Length of expected Tx differed. tc #%d, i #%d", tcIndex, i)
			require.Equal(t, tc.txSize, len(updatedTx),
				"Length of expected Tx differed. tc #%d, i #%d", tcIndex, i)
			require.Equal(t, tc.txSize*2, len(expectedHex),
				"Length of expected hex differed. tc #%d, i #%d", tcIndex, i)
			require.Equal(t, tc.txSize*2, len(updatedHex),
				"Length of updated hex differed. tc #%d, i #%d", tcIndex, i)
		}
	}
}

func BenchmarkIterationOfSendLoop(b *testing.B) {
	var (
		connIndex = 0
		txSize    = 25000
	)

	now := time.Now()
	// something too far away to matter
	endTime := now.Add(time.Hour)
	txNumber := 0
	hostnameHash := sha256.Sum256([]byte{0})
	tx := generateTx(connIndex, txNumber, txSize, hostnameHash)
	txHex := make([]byte, len(tx)*2)
	hex.Encode(txHex, tx)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		updateTx(tx, txHex, txNumber)
		paramsJSON, err := json.Marshal(map[string]interface{}{"tx": txHex})
		if err != nil {
			fmt.Printf("failed to encode params: %v\n", err)
			os.Exit(1)
		}
		_ = json.RawMessage(paramsJSON)
		_ = now.Add(sendTimeout)

		if err != nil {
			err = errors.Wrap(err,
				fmt.Sprintf("txs send failed on connection #%d", connIndex))
			logger.Error(err.Error())
			return
		}

		// Cache the now operations
		if i%5 == 0 {
			now = time.Now()
			if now.After(endTime) {
				break
			}
		}

		txNumber++
	}
}
