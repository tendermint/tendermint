package types

import (
	"bytes"
	"io/ioutil"
	"testing"

	. "github.com/tendermint/go-common"
)

const (
	testPartSize = 65536 // 64KB ...  4096 // 4KB
)

func TestBasicPartSet(t *testing.T) {

	// Construct random data of size partSize * 100
	data := RandBytes(testPartSize * 100)

	partSet := NewPartSetFromData(data, testPartSize)
	if len(partSet.Hash()) == 0 {
		t.Error("Expected to get hash")
	}
	if partSet.Total() != 100 {
		t.Errorf("Expected to get 100 parts, but got %v", partSet.Total())
	}
	if !partSet.IsComplete() {
		t.Errorf("PartSet should be complete")
	}

	// Test adding parts to a new partSet.
	partSet2 := NewPartSetFromHeader(partSet.Header())

	for i := 0; i < partSet.Total(); i++ {
		part := partSet.GetPart(i)
		//t.Logf("\n%v", part)
		added, err := partSet2.AddPart(part, true)
		if !added || err != nil {
			t.Errorf("Failed to add part %v, error: %v", i, err)
		}
	}

	if !bytes.Equal(partSet.Hash(), partSet2.Hash()) {
		t.Error("Expected to get same hash")
	}
	if partSet2.Total() != 100 {
		t.Errorf("Expected to get 100 parts, but got %v", partSet2.Total())
	}
	if !partSet2.IsComplete() {
		t.Errorf("Reconstructed PartSet should be complete")
	}

	// Reconstruct data, assert that they are equal.
	data2Reader := partSet2.GetReader()
	data2, err := ioutil.ReadAll(data2Reader)
	if err != nil {
		t.Errorf("Error reading data2Reader: %v", err)
	}
	if !bytes.Equal(data, data2) {
		t.Errorf("Got wrong data.")
	}

}

func TestWrongProof(t *testing.T) {

	// Construct random data of size partSize * 100
	data := RandBytes(testPartSize * 100)
	partSet := NewPartSetFromData(data, testPartSize)

	// Test adding a part with wrong data.
	partSet2 := NewPartSetFromHeader(partSet.Header())

	// Test adding a part with wrong trail.
	part := partSet.GetPart(0)
	part.Proof.Aunts[0][0] += byte(0x01)
	added, err := partSet2.AddPart(part, true)
	if added || err == nil {
		t.Errorf("Expected to fail adding a part with bad trail.")
	}

	// Test adding a part with wrong bytes.
	part = partSet.GetPart(1)
	part.Bytes[0] += byte(0x01)
	added, err = partSet2.AddPart(part, true)
	if added || err == nil {
		t.Errorf("Expected to fail adding a part with bad bytes.")
	}

}
