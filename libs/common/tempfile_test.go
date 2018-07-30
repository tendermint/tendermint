package common

// Need access to internal variables, so can't use _test package

import (
	"bytes"
	fmt "fmt"
	"io/ioutil"
	"os"
	testing "testing"

	"github.com/stretchr/testify/require"
)

func TestWriteFileAtomic(t *testing.T) {
	var (
		data             = []byte(RandStr(RandIntn(2048)))
		old              = RandBytes(RandIntn(2048))
		perm os.FileMode = 0600
	)

	f, err := ioutil.TempFile("/tmp", "write-atomic-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	if err = ioutil.WriteFile(f.Name(), old, 0664); err != nil {
		t.Fatal(err)
	}

	if err = WriteFileAtomic(f.Name(), data, perm); err != nil {
		t.Fatal(err)
	}

	rData, err := ioutil.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data, rData) {
		t.Fatalf("data mismatch: %v != %v", data, rData)
	}

	stat, err := os.Stat(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	if have, want := stat.Mode().Perm(), perm; have != want {
		t.Errorf("have %v, want %v", have, want)
	}
}

// This tests atomic write file when there is a single duplicate file.
// Expected behavior is for a new file to be created, and the original write file to be unaltered.
func TestWriteFileAtomicDuplicateFile(t *testing.T) {
	var (
		defaultSeed    uint64 = 1
		testString            = "This is a glorious test string"
		expectedString        = "Did the test file's string appear here?"

		fileToWrite = "/tmp/TestWriteFileAtomicDuplicateFile-test.txt"
	)
	// Create a file at the seed, and reset the seed.
	atomicWriteFileRand = defaultSeed
	firstFileRand := randWriteFileSuffix()
	atomicWriteFileRand = defaultSeed
	fname := "/tmp/" + atomicWriteFilePrefix + firstFileRand
	f, err := os.OpenFile(fname, atomicWriteFileFlag, 0777)
	defer os.Remove(fname)
	// Defer here, in case there is a panic in WriteFileAtomic.
	defer os.Remove(fileToWrite)

	require.Nil(t, err)
	f.WriteString(testString)
	WriteFileAtomic(fileToWrite, []byte(expectedString), 0777)
	// Check that the first atomic file was untouched
	firstAtomicFileBytes, err := ioutil.ReadFile(fname)
	require.Nil(t, err, "Error reading first atomic file")
	require.Equal(t, []byte(testString), firstAtomicFileBytes, "First atomic file was overwritten")
	// Check that the resultant file is correct
	resultantFileBytes, err := ioutil.ReadFile(fileToWrite)
	require.Nil(t, err, "Error reading resultant file")
	require.Equal(t, []byte(expectedString), resultantFileBytes, "Written file had incorrect bytes")

	// Check that the intermediate write file was deleted
	// Get the second write files' randomness
	atomicWriteFileRand = defaultSeed
	_ = randWriteFileSuffix()
	secondFileRand := randWriteFileSuffix()
	_, err = os.Stat("/tmp/" + atomicWriteFilePrefix + secondFileRand)
	require.True(t, os.IsNotExist(err), "Intermittent atomic write file not deleted")
}

// This tests atomic write file when there are many duplicate files.
// Expected behavior is for a new file to be created under a completely new seed,
// and the original write files to be unaltered.
func TestWriteFileAtomicManyDuplicates(t *testing.T) {
	var (
		defaultSeed    uint64 = 2
		testString            = "This is a glorious test string, from file %d"
		expectedString        = "Did any of the test file's string appear here?"

		fileToWrite = "/tmp/TestWriteFileAtomicDuplicateFile-test.txt"
	)
	// Initialize all of the atomic write files
	atomicWriteFileRand = defaultSeed
	for i := 0; i < atomicWriteFileMaxNumConflicts+2; i++ {
		fileRand := randWriteFileSuffix()
		fname := "/tmp/" + atomicWriteFilePrefix + fileRand
		f, err := os.OpenFile(fname, atomicWriteFileFlag, 0777)
		require.Nil(t, err)
		f.WriteString(fmt.Sprintf(testString, i))
		defer os.Remove(fname)
	}

	atomicWriteFileRand = defaultSeed
	// Defer here, in case there is a panic in WriteFileAtomic.
	defer os.Remove(fileToWrite)

	WriteFileAtomic(fileToWrite, []byte(expectedString), 0777)
	// Check that all intermittent atomic file were untouched
	atomicWriteFileRand = defaultSeed
	for i := 0; i < atomicWriteFileMaxNumConflicts+2; i++ {
		fileRand := randWriteFileSuffix()
		fname := "/tmp/" + atomicWriteFilePrefix + fileRand
		firstAtomicFileBytes, err := ioutil.ReadFile(fname)
		require.Nil(t, err, "Error reading first atomic file")
		require.Equal(t, []byte(fmt.Sprintf(testString, i)), firstAtomicFileBytes,
			"atomic write file %d was overwritten", i)
	}

	// Check that the resultant file is correct
	resultantFileBytes, err := ioutil.ReadFile(fileToWrite)
	require.Nil(t, err, "Error reading resultant file")
	require.Equal(t, []byte(expectedString), resultantFileBytes, "Written file had incorrect bytes")
}
