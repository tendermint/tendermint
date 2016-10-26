package autofile

import (
	"testing"

	. "github.com/tendermint/go-common"
)

func createTestGroup(t *testing.T, headPath string) *Group {
	autofile, err := OpenAutoFile(headPath)
	if err != nil {
		t.Fatal("Error opening AutoFile", headPath, err)
	}
	g, err := OpenGroup(autofile)
	if err != nil {
		t.Fatal("Error opening Group", err)
	}
	return g
}

func assertGroupInfo(t *testing.T, gInfo GroupInfo, minIndex, maxIndex int, totalSize, headSize int64) {
	if gInfo.MinIndex != minIndex {
		t.Errorf("GroupInfo MinIndex expected %v, got %v", minIndex, gInfo.MinIndex)
	}
	if gInfo.MaxIndex != maxIndex {
		t.Errorf("GroupInfo MaxIndex expected %v, got %v", maxIndex, gInfo.MaxIndex)
	}
	if gInfo.TotalSize != totalSize {
		t.Errorf("GroupInfo TotalSize expected %v, got %v", totalSize, gInfo.TotalSize)
	}
	if gInfo.HeadSize != headSize {
		t.Errorf("GroupInfo HeadSize expected %v, got %v", headSize, gInfo.HeadSize)
	}
}

func TestCreateGroup(t *testing.T) {
	testID := RandStr(12)
	testDir := "_test_" + testID
	err := EnsureDir(testDir, 0700)
	if err != nil {
		t.Fatal("Error creating dir", err)
	}

	g := createTestGroup(t, testDir+"/myfile")
	if g == nil {
		t.Error("Failed to create Group")
	}
	g.SetHeadSizeLimit(1000 * 1000)
	g.stopTicker()

	// At first, there are no files.
	assertGroupInfo(t, g.ReadGroupInfo(), -1, -1, 0, 0)

	// Write 1000 bytes 999 times.
	for i := 0; i < 999; i++ {
		_, err := g.Head.Write([]byte(RandStr(999) + "\n"))
		if err != nil {
			t.Fatal("Error appending to head", err)
		}
	}
	assertGroupInfo(t, g.ReadGroupInfo(), -1, -1, 999000, 999000)

	// Even calling checkHeadSizeLimit manually won't rotate it.
	g.checkHeadSizeLimit()
	assertGroupInfo(t, g.ReadGroupInfo(), -1, -1, 999000, 999000)

	// Write 1000 more bytes.
	_, err = g.Head.Write([]byte(RandStr(999) + "\n"))
	if err != nil {
		t.Fatal("Error appending to head", err)
	}

	// Calling checkHeadSizeLimit this time rolls it.
	g.checkHeadSizeLimit()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 0, 1000000, 0)

	// Write 1000 more bytes.
	_, err = g.Head.Write([]byte(RandStr(999) + "\n"))
	if err != nil {
		t.Fatal("Error appending to head", err)
	}

	// Calling checkHeadSizeLimit does nothing.
	g.checkHeadSizeLimit()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 0, 1001000, 1000)

	// Write 1000 bytes 999 times.
	for i := 0; i < 999; i++ {
		_, err := g.Head.Write([]byte(RandStr(999) + "\n"))
		if err != nil {
			t.Fatal("Error appending to head", err)
		}
	}
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 0, 2000000, 1000000)

	// Calling checkHeadSizeLimit rolls it again.
	g.checkHeadSizeLimit()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 1, 2000000, 0)

	// Write 1000 more bytes.
	_, err = g.Head.Write([]byte(RandStr(999) + "\n"))
	if err != nil {
		t.Fatal("Error appending to head", err)
	}
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 1, 2001000, 1000)

	// Calling checkHeadSizeLimit does nothing.
	g.checkHeadSizeLimit()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 1, 2001000, 1000)
}
