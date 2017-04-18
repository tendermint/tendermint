package autofile

import (
	"errors"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"testing"

	. "github.com/tendermint/go-common"
)

// NOTE: Returned group has ticker stopped
func createTestGroup(t *testing.T, headSizeLimit int64) *Group {
	testID := RandStr(12)
	testDir := "_test_" + testID
	err := EnsureDir(testDir, 0700)
	if err != nil {
		t.Fatal("Error creating dir", err)
	}
	headPath := testDir + "/myfile"
	g, err := OpenGroup(headPath)
	if err != nil {
		t.Fatal("Error opening Group", err)
	}
	g.SetHeadSizeLimit(headSizeLimit)
	g.stopTicker()

	if g == nil {
		t.Fatal("Failed to create Group")
	}
	return g
}

func destroyTestGroup(t *testing.T, g *Group) {
	err := os.RemoveAll(g.Dir)
	if err != nil {
		t.Fatal("Error removing test Group directory", err)
	}
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

func TestCheckHeadSizeLimit(t *testing.T) {
	g := createTestGroup(t, 1000*1000)

	// At first, there are no files.
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 0, 0, 0)

	// Write 1000 bytes 999 times.
	for i := 0; i < 999; i++ {
		err := g.WriteLine(RandStr(999))
		if err != nil {
			t.Fatal("Error appending to head", err)
		}
	}
	g.Flush()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 0, 999000, 999000)

	// Even calling checkHeadSizeLimit manually won't rotate it.
	g.checkHeadSizeLimit()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 0, 999000, 999000)

	// Write 1000 more bytes.
	err := g.WriteLine(RandStr(999))
	if err != nil {
		t.Fatal("Error appending to head", err)
	}
	g.Flush()

	// Calling checkHeadSizeLimit this time rolls it.
	g.checkHeadSizeLimit()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 1, 1000000, 0)

	// Write 1000 more bytes.
	err = g.WriteLine(RandStr(999))
	if err != nil {
		t.Fatal("Error appending to head", err)
	}
	g.Flush()

	// Calling checkHeadSizeLimit does nothing.
	g.checkHeadSizeLimit()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 1, 1001000, 1000)

	// Write 1000 bytes 999 times.
	for i := 0; i < 999; i++ {
		err := g.WriteLine(RandStr(999))
		if err != nil {
			t.Fatal("Error appending to head", err)
		}
	}
	g.Flush()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 1, 2000000, 1000000)

	// Calling checkHeadSizeLimit rolls it again.
	g.checkHeadSizeLimit()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 2, 2000000, 0)

	// Write 1000 more bytes.
	_, err = g.Head.Write([]byte(RandStr(999) + "\n"))
	if err != nil {
		t.Fatal("Error appending to head", err)
	}
	g.Flush()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 2, 2001000, 1000)

	// Calling checkHeadSizeLimit does nothing.
	g.checkHeadSizeLimit()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 2, 2001000, 1000)

	// Cleanup
	destroyTestGroup(t, g)
}

func TestSearch(t *testing.T) {
	g := createTestGroup(t, 10*1000)

	// Create some files in the group that have several INFO lines in them.
	// Try to put the INFO lines in various spots.
	for i := 0; i < 100; i++ {
		// The random junk at the end ensures that this INFO linen
		// is equally likely to show up at the end.
		_, err := g.Head.Write([]byte(Fmt("INFO %v %v\n", i, RandStr(123))))
		if err != nil {
			t.Error("Failed to write to head")
		}
		g.checkHeadSizeLimit()
		for j := 0; j < 10; j++ {
			_, err := g.Head.Write([]byte(RandStr(123) + "\n"))
			if err != nil {
				t.Error("Failed to write to head")
			}
			g.checkHeadSizeLimit()
		}
	}

	// Create a search func that searches for line
	makeSearchFunc := func(target int) SearchFunc {
		return func(line string) (int, error) {
			parts := strings.Split(line, " ")
			if len(parts) != 3 {
				return -1, errors.New("Line did not have 3 parts")
			}
			i, err := strconv.Atoi(parts[1])
			if err != nil {
				return -1, errors.New("Failed to parse INFO: " + err.Error())
			}
			if target < i {
				return 1, nil
			} else if target == i {
				return 0, nil
			} else {
				return -1, nil
			}
		}
	}

	// Now search for each number
	for i := 0; i < 100; i++ {
		t.Log("Testing for i", i)
		gr, match, err := g.Search("INFO", makeSearchFunc(i))
		if err != nil {
			t.Fatal("Failed to search for line:", err)
		}
		if !match {
			t.Error("Expected Search to return exact match")
		}
		line, err := gr.ReadLine()
		if err != nil {
			t.Fatal("Failed to read line after search", err)
		}
		if !strings.HasPrefix(line, Fmt("INFO %v ", i)) {
			t.Fatal("Failed to get correct line")
		}
		// Make sure we can continue to read from there.
		cur := i + 1
		for {
			line, err := gr.ReadLine()
			if err == io.EOF {
				if cur == 99+1 {
					// OK!
					break
				} else {
					t.Fatal("Got EOF after the wrong INFO #")
				}
			} else if err != nil {
				t.Fatal("Error reading line", err)
			}
			if !strings.HasPrefix(line, "INFO ") {
				continue
			}
			if !strings.HasPrefix(line, Fmt("INFO %v ", cur)) {
				t.Fatalf("Unexpected INFO #. Expected %v got:\n%v", cur, line)
			}
			cur += 1
		}
		gr.Close()
	}

	// Now search for something that is too small.
	// We should get the first available line.
	{
		gr, match, err := g.Search("INFO", makeSearchFunc(-999))
		if err != nil {
			t.Fatal("Failed to search for line:", err)
		}
		if match {
			t.Error("Expected Search to not return exact match")
		}
		line, err := gr.ReadLine()
		if err != nil {
			t.Fatal("Failed to read line after search", err)
		}
		if !strings.HasPrefix(line, "INFO 0 ") {
			t.Error("Failed to fetch correct line, which is the earliest INFO")
		}
		err = gr.Close()
		if err != nil {
			t.Error("Failed to close GroupReader", err)
		}
	}

	// Now search for something that is too large.
	// We should get an EOF error.
	{
		gr, _, err := g.Search("INFO", makeSearchFunc(999))
		if err != io.EOF {
			t.Error("Expected to get an EOF error")
		}
		if gr != nil {
			t.Error("Expected to get nil GroupReader")
		}
	}

	// Cleanup
	destroyTestGroup(t, g)
}

func TestRotateFile(t *testing.T) {
	g := createTestGroup(t, 0)
	g.WriteLine("Line 1")
	g.WriteLine("Line 2")
	g.WriteLine("Line 3")
	g.Flush()
	g.RotateFile()
	g.WriteLine("Line 4")
	g.WriteLine("Line 5")
	g.WriteLine("Line 6")
	g.Flush()

	// Read g.Head.Path+"000"
	body1, err := ioutil.ReadFile(g.Head.Path + ".000")
	if err != nil {
		t.Error("Failed to read first rolled file")
	}
	if string(body1) != "Line 1\nLine 2\nLine 3\n" {
		t.Errorf("Got unexpected contents: [%v]", string(body1))
	}

	// Read g.Head.Path
	body2, err := ioutil.ReadFile(g.Head.Path)
	if err != nil {
		t.Error("Failed to read first rolled file")
	}
	if string(body2) != "Line 4\nLine 5\nLine 6\n" {
		t.Errorf("Got unexpected contents: [%v]", string(body2))
	}

	// Cleanup
	destroyTestGroup(t, g)
}

func TestFindLast1(t *testing.T) {
	g := createTestGroup(t, 0)

	g.WriteLine("Line 1")
	g.WriteLine("Line 2")
	g.WriteLine("# a")
	g.WriteLine("Line 3")
	g.Flush()
	g.RotateFile()
	g.WriteLine("Line 4")
	g.WriteLine("Line 5")
	g.WriteLine("Line 6")
	g.WriteLine("# b")
	g.Flush()

	match, found, err := g.FindLast("#")
	if err != nil {
		t.Error("Unexpected error", err)
	}
	if !found {
		t.Error("Expected found=True")
	}
	if match != "# b" {
		t.Errorf("Unexpected match: [%v]", match)
	}

	// Cleanup
	destroyTestGroup(t, g)
}

func TestFindLast2(t *testing.T) {
	g := createTestGroup(t, 0)

	g.WriteLine("Line 1")
	g.WriteLine("Line 2")
	g.WriteLine("Line 3")
	g.Flush()
	g.RotateFile()
	g.WriteLine("# a")
	g.WriteLine("Line 4")
	g.WriteLine("Line 5")
	g.WriteLine("# b")
	g.WriteLine("Line 6")
	g.Flush()

	match, found, err := g.FindLast("#")
	if err != nil {
		t.Error("Unexpected error", err)
	}
	if !found {
		t.Error("Expected found=True")
	}
	if match != "# b" {
		t.Errorf("Unexpected match: [%v]", match)
	}

	// Cleanup
	destroyTestGroup(t, g)
}

func TestFindLast3(t *testing.T) {
	g := createTestGroup(t, 0)

	g.WriteLine("Line 1")
	g.WriteLine("# a")
	g.WriteLine("Line 2")
	g.WriteLine("# b")
	g.WriteLine("Line 3")
	g.Flush()
	g.RotateFile()
	g.WriteLine("Line 4")
	g.WriteLine("Line 5")
	g.WriteLine("Line 6")
	g.Flush()

	match, found, err := g.FindLast("#")
	if err != nil {
		t.Error("Unexpected error", err)
	}
	if !found {
		t.Error("Expected found=True")
	}
	if match != "# b" {
		t.Errorf("Unexpected match: [%v]", match)
	}

	// Cleanup
	destroyTestGroup(t, g)
}

func TestFindLast4(t *testing.T) {
	g := createTestGroup(t, 0)

	g.WriteLine("Line 1")
	g.WriteLine("Line 2")
	g.WriteLine("Line 3")
	g.Flush()
	g.RotateFile()
	g.WriteLine("Line 4")
	g.WriteLine("Line 5")
	g.WriteLine("Line 6")
	g.Flush()

	match, found, err := g.FindLast("#")
	if err != nil {
		t.Error("Unexpected error", err)
	}
	if found {
		t.Error("Expected found=False")
	}
	if match != "" {
		t.Errorf("Unexpected match: [%v]", match)
	}

	// Cleanup
	destroyTestGroup(t, g)
}
