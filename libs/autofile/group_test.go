package autofile

import (
	"io"
	"io/ioutil"
	"os"
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
	g.FlushAndSync()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 0, 999000, 999000)

	// Even calling checkHeadSizeLimit manually won't rotate it.
	g.checkHeadSizeLimit()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 0, 999000, 999000)

	// Write 1000 more bytes.
	err := g.WriteLine(cmn.RandStr(999))
	require.NoError(t, err, "Error appending to head")
	g.FlushAndSync()

	// Calling checkHeadSizeLimit this time rolls it.
	g.checkHeadSizeLimit()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 1, 1000000, 0)

	// Write 1000 more bytes.
	err = g.WriteLine(cmn.RandStr(999))
	require.NoError(t, err, "Error appending to head")
	g.FlushAndSync()

	// Calling checkHeadSizeLimit does nothing.
	g.checkHeadSizeLimit()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 1, 1001000, 1000)

	// Write 1000 bytes 999 times.
	for i := 0; i < 999; i++ {
		err = g.WriteLine(cmn.RandStr(999))
		require.NoError(t, err, "Error appending to head")
	}
	g.FlushAndSync()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 1, 2000000, 1000000)

	// Calling checkHeadSizeLimit rolls it again.
	g.checkHeadSizeLimit()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 2, 2000000, 0)

	// Write 1000 more bytes.
	_, err = g.Head.Write([]byte(cmn.RandStr(999) + "\n"))
	require.NoError(t, err, "Error appending to head")
	g.FlushAndSync()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 2, 2001000, 1000)

	// Calling checkHeadSizeLimit does nothing.
	g.checkHeadSizeLimit()
	assertGroupInfo(t, g.ReadGroupInfo(), 0, 2, 2001000, 1000)

	// Cleanup
	destroyTestGroup(t, g)
}

func TestRotateFile(t *testing.T) {
	g := createTestGroupWithHeadSizeLimit(t, 0)
	g.WriteLine("Line 1")
	g.WriteLine("Line 2")
	g.WriteLine("Line 3")
	g.FlushAndSync()
	g.RotateFile()
	g.WriteLine("Line 4")
	g.WriteLine("Line 5")
	g.WriteLine("Line 6")
	g.FlushAndSync()

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

func TestWrite(t *testing.T) {
	g := createTestGroupWithHeadSizeLimit(t, 0)

	written := []byte("Medusa")
	g.Write(written)
	g.FlushAndSync()

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
	g.FlushAndSync()
	g.RotateFile()
	frankenstein := []byte("Frankenstein's Monster")
	g.Write(frankenstein)
	g.FlushAndSync()

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
	g.FlushAndSync()
	g.RotateFile()
	frankenstein := []byte("Frankenstein's Monster")
	frankensteinPart := []byte("Frankenstein")
	g.Write(frankensteinPart) // note writing only a part
	g.FlushAndSync()

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
	g.FlushAndSync()
	g.RotateFile()

	assert.Equal(t, 1, g.MaxIndex(), "MaxIndex should point to the last file")

	// Cleanup
	destroyTestGroup(t, g)
}
