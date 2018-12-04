package autofile

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cmn "github.com/tendermint/tendermint/libs/common"
)

func createTestGroupWithHeadSizeLimit(t *testing.T, headSizeLimit int64) *Group {
	testID := cmn.RandStr(12)
	testDir := "_test_" + testID
	err := cmn.EnsureDir(testDir, 0700)
	require.NoError(t, err, "Error creating dir")

	headPath := testDir + "/myfile"
	g, err := OpenGroup(headPath, GroupHeadSizeLimit(headSizeLimit))
	require.NoError(t, err, "Error opening Group")
	require.NotEqual(t, nil, g, "Failed to create Group")

	return g
}

func destroyTestGroup(t *testing.T, g *Group) {
	g.Close()

	err := os.RemoveAll(g.Dir)
	require.NoError(t, err, "Error removing test Group directory")
}

func assertGroupInfo(t *testing.T, gInfo GroupInfo, minIndex, maxIndex int, totalSize, headSize int64) {
	assert.Equal(t, minIndex, gInfo.MinIndex)
	assert.Equal(t, maxIndex, gInfo.MaxIndex)
	assert.Equal(t, totalSize, gInfo.TotalSize)
	assert.Equal(t, headSize, gInfo.HeadSize)
}

func TestCheckHeadSizeLimit(t *testing.T) {
	g := createTestGroupWithHeadSizeLimit(t, 1000*1000)

	// At first, there are no files.
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 0, 0, 0)

	// Write 1000 bytes 999 times.
	for i := 0; i < 999; i++ {
		err := g.WriteLine(cmn.RandStr(999))
		require.NoError(t, err, "Error appending to head")
	}
	g.Flush()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 0, 999000, 999000)

	// Even calling checkHeadSizeLimit manually won't rotate it.
	g.checkHeadSizeLimit()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 0, 999000, 999000)

	// Write 1000 more bytes.
	err := g.WriteLine(cmn.RandStr(999))
	require.NoError(t, err, "Error appending to head")
	g.Flush()

	// Calling checkHeadSizeLimit this time rolls it.
	g.checkHeadSizeLimit()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 1, 1000000, 0)

	// Write 1000 more bytes.
	err = g.WriteLine(cmn.RandStr(999))
	require.NoError(t, err, "Error appending to head")
	g.Flush()

	// Calling checkHeadSizeLimit does nothing.
	g.checkHeadSizeLimit()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 1, 1001000, 1000)

	// Write 1000 bytes 999 times.
	for i := 0; i < 999; i++ {
		err = g.WriteLine(cmn.RandStr(999))
		require.NoError(t, err, "Error appending to head")
	}
	g.Flush()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 1, 2000000, 1000000)

	// Calling checkHeadSizeLimit rolls it again.
	g.checkHeadSizeLimit()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 2, 2000000, 0)

	// Write 1000 more bytes.
	_, err = g.Head.Write([]byte(cmn.RandStr(999) + "\n"))
	require.NoError(t, err, "Error appending to head")
	g.Flush()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 2, 2001000, 1000)

	// Calling checkHeadSizeLimit does nothing.
	g.checkHeadSizeLimit()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 2, 2001000, 1000)

	// Cleanup
	destroyTestGroup(t, g)
}

func TestSearch(t *testing.T) {
	g := createTestGroupWithHeadSizeLimit(t, 10*1000)

	// Create some files in the group that have several INFO lines in them.
	// Try to put the INFO lines in various spots.
	for i := 0; i < 100; i++ {
		// The random junk at the end ensures that this INFO linen
		// is equally likely to show up at the end.
		_, err := g.Head.Write([]byte(fmt.Sprintf("INFO %v %v\n", i, cmn.RandStr(123))))
		require.NoError(t, err, "Failed to write to head")
		g.checkHeadSizeLimit()
		for j := 0; j < 10; j++ {
			_, err1 := g.Head.Write([]byte(cmn.RandStr(123) + "\n"))
			require.NoError(t, err1, "Failed to write to head")
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
		gr, match, err := g.Search("INFO", makeSearchFunc(i))
		require.NoError(t, err, "Failed to search for line, tc #%d", i)
		assert.True(t, match, "Expected Search to return exact match, tc #%d", i)
		line, err := gr.ReadLine()
		require.NoError(t, err, "Failed to read line after search, tc #%d", i)
		if !strings.HasPrefix(line, fmt.Sprintf("INFO %v ", i)) {
			t.Fatalf("Failed to get correct line, tc #%d", i)
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
					t.Fatalf("Got EOF after the wrong INFO #, tc #%d", i)
				}
			} else if err != nil {
				t.Fatalf("Error reading line, tc #%d, err:\n%s", i, err)
			}
			if !strings.HasPrefix(line, "INFO ") {
				continue
			}
			if !strings.HasPrefix(line, fmt.Sprintf("INFO %v ", cur)) {
				t.Fatalf("Unexpected INFO #. Expected %v got:\n%v, tc #%d", cur, line, i)
			}
			cur++
		}
		gr.Close()
	}

	// Now search for something that is too small.
	// We should get the first available line.
	{
		gr, match, err := g.Search("INFO", makeSearchFunc(-999))
		require.NoError(t, err, "Failed to search for line")
		assert.False(t, match, "Expected Search to not return exact match")
		line, err := gr.ReadLine()
		require.NoError(t, err, "Failed to read line after search")
		if !strings.HasPrefix(line, "INFO 0 ") {
			t.Error("Failed to fetch correct line, which is the earliest INFO")
		}
		err = gr.Close()
		require.NoError(t, err, "Failed to close GroupReader")
	}

	// Now search for something that is too large.
	// We should get an EOF error.
	{
		gr, _, err := g.Search("INFO", makeSearchFunc(999))
		assert.Equal(t, io.EOF, err)
		assert.Nil(t, gr)
	}

	// Cleanup
	destroyTestGroup(t, g)
}

func TestRotateFile(t *testing.T) {
	g := createTestGroupWithHeadSizeLimit(t, 0)
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
	assert.NoError(t, err, "Failed to read first rolled file")
	if string(body1) != "Line 1\nLine 2\nLine 3\n" {
		t.Errorf("Got unexpected contents: [%v]", string(body1))
	}

	// Read g.Head.Path
	body2, err := ioutil.ReadFile(g.Head.Path)
	assert.NoError(t, err, "Failed to read first rolled file")
	if string(body2) != "Line 4\nLine 5\nLine 6\n" {
		t.Errorf("Got unexpected contents: [%v]", string(body2))
	}

	// Cleanup
	destroyTestGroup(t, g)
}

func TestFindLast1(t *testing.T) {
	g := createTestGroupWithHeadSizeLimit(t, 0)

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
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "# b", match)

	// Cleanup
	destroyTestGroup(t, g)
}

func TestFindLast2(t *testing.T) {
	g := createTestGroupWithHeadSizeLimit(t, 0)

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
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "# b", match)

	// Cleanup
	destroyTestGroup(t, g)
}

func TestFindLast3(t *testing.T) {
	g := createTestGroupWithHeadSizeLimit(t, 0)

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
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "# b", match)

	// Cleanup
	destroyTestGroup(t, g)
}

func TestFindLast4(t *testing.T) {
	g := createTestGroupWithHeadSizeLimit(t, 0)

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
	assert.NoError(t, err)
	assert.False(t, found)
	assert.Empty(t, match)

	// Cleanup
	destroyTestGroup(t, g)
}

func TestWrite(t *testing.T) {
	g := createTestGroupWithHeadSizeLimit(t, 0)

	written := []byte("Medusa")
	g.Write(written)
	g.Flush()

	read := make([]byte, len(written))
	gr, err := g.NewReader(0)
	require.NoError(t, err, "failed to create reader")

	_, err = gr.Read(read)
	assert.NoError(t, err, "failed to read data")
	assert.Equal(t, written, read)

	// Cleanup
	destroyTestGroup(t, g)
}

// test that Read reads the required amount of bytes from all the files in the
// group and returns no error if n == size of the given slice.
func TestGroupReaderRead(t *testing.T) {
	g := createTestGroupWithHeadSizeLimit(t, 0)

	professor := []byte("Professor Monster")
	g.Write(professor)
	g.Flush()
	g.RotateFile()
	frankenstein := []byte("Frankenstein's Monster")
	g.Write(frankenstein)
	g.Flush()

	totalWrittenLength := len(professor) + len(frankenstein)
	read := make([]byte, totalWrittenLength)
	gr, err := g.NewReader(0)
	require.NoError(t, err, "failed to create reader")

	n, err := gr.Read(read)
	assert.NoError(t, err, "failed to read data")
	assert.Equal(t, totalWrittenLength, n, "not enough bytes read")
	professorPlusFrankenstein := professor
	professorPlusFrankenstein = append(professorPlusFrankenstein, frankenstein...)
	assert.Equal(t, professorPlusFrankenstein, read)

	// Cleanup
	destroyTestGroup(t, g)
}

// test that Read returns an error if number of bytes read < size of
// the given slice. Subsequent call should return 0, io.EOF.
func TestGroupReaderRead2(t *testing.T) {
	g := createTestGroupWithHeadSizeLimit(t, 0)

	professor := []byte("Professor Monster")
	g.Write(professor)
	g.Flush()
	g.RotateFile()
	frankenstein := []byte("Frankenstein's Monster")
	frankensteinPart := []byte("Frankenstein")
	g.Write(frankensteinPart) // note writing only a part
	g.Flush()

	totalLength := len(professor) + len(frankenstein)
	read := make([]byte, totalLength)
	gr, err := g.NewReader(0)
	require.NoError(t, err, "failed to create reader")

	// 1) n < (size of the given slice), io.EOF
	n, err := gr.Read(read)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, len(professor)+len(frankensteinPart), n, "Read more/less bytes than it is in the group")

	// 2) 0, io.EOF
	n, err = gr.Read([]byte("0"))
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 0, n)

	// Cleanup
	destroyTestGroup(t, g)
}

func TestMinIndex(t *testing.T) {
	g := createTestGroupWithHeadSizeLimit(t, 0)

	assert.Zero(t, g.MinIndex(), "MinIndex should be zero at the beginning")

	// Cleanup
	destroyTestGroup(t, g)
}

func TestMaxIndex(t *testing.T) {
	g := createTestGroupWithHeadSizeLimit(t, 0)

	assert.Zero(t, g.MaxIndex(), "MaxIndex should be zero at the beginning")

	g.WriteLine("Line 1")
	g.Flush()
	g.RotateFile()

	assert.Equal(t, 1, g.MaxIndex(), "MaxIndex should point to the last file")

	// Cleanup
	destroyTestGroup(t, g)
}
