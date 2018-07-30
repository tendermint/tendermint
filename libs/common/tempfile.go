package common

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	atomicWriteFileRand   uint64
	atomicWriteFileRandMu sync.Mutex
)

func writeFileRandReseed() uint64 {
	// Scale the PID, to minimize the chance that two processes seeded at similar times
	// don't get the same seed. Note that PID typically ranges in [0, 2**15), but can be
	// up to 2**22 under certain configurations. We left bit-shift the PID by 20, so that
	// a PID difference of one corresponds to a time difference of 2048 seconds.
	// The important thing here is that now for a seed conflict, they would both have to be on
	// the correct nanosecond offset, and second-based offset, which is much less likely then
	// just a conflict with the correct nanosecond offset.
	return uint64(time.Now().UnixNano() + int64(os.Getpid()<<20))
}

// Use a fast thread safe LCG for atomic write file names.
// Returns a string corresponding to a 64 bit int.
// If it was a negative int, the leading number is a 0.
func randWriteFileSuffix() string {
	atomicWriteFileRandMu.Lock()
	r := atomicWriteFileRand
	if r == 0 {
		r = writeFileRandReseed()
	}
	// constants from Donald Knuth MMIX
	// This LCG's has a period equal to 2**64
	r = r*6364136223846793005 + 1442695040888963407

	atomicWriteFileRand = r
	atomicWriteFileRandMu.Unlock()
	// Can have a negative name, replace this in the following
	suffix := strconv.Itoa(int(r))
	if string(suffix[0]) == "-" {
		// Replace first "-" with "0". This is purely for UI clarity
		suffix = strings.Replace(suffix, "-", "0", 1)
	}
	return suffix
}

// WriteFileAtomic creates a temporary file with data and the perm given and
// swaps it atomically with filename if successful.
// This implementation is inspired by the golang stdlibs method of creating
// tempfiles. Notable differences are that we use different flags, a 64 bit LCG
// and handle negatives differently.
func WriteFileAtomic(filename string, data []byte, perm os.FileMode) (err error) {
	var (
		dir = filepath.Dir(filename)
		// Create in case it doesn't exist and force kernel
		// flush, which still leaves the potential of lingering disk cache.
		// Never overwrites files
		flag = os.O_WRONLY | os.O_CREATE | os.O_SYNC | os.O_TRUNC | os.O_EXCL
		f    *os.File
	)

	nconflict := 0
	for i := 0; i < 10000; i++ {
		name := filepath.Join(dir, "write-file-atomic-"+randWriteFileSuffix())
		f, err = os.OpenFile(name, flag, perm)
		// If the file already exists, try a new file
		if os.IsExist(err) {
			// If the files already exist too many times,
			// reseed as this indicates we likely hit another instances
			// seed, and that instance hasn't handled its deletions correctly.
			if nconflict++; nconflict > 5 {
				atomicWriteFileRandMu.Lock()
				atomicWriteFileRand = writeFileRandReseed()
				atomicWriteFileRandMu.Unlock()
			}
			continue
		} else if err != nil {
			return err
		}
		break
	}

	// Clean up in any case. Defer stacking order is last-in-first-out.
	defer os.Remove(f.Name())
	defer f.Close()

	if n, err := f.Write(data); err != nil {
		return err
	} else if n < len(data) {
		return io.ErrShortWrite
	}
	// Close the file before renaming it, otherwise it will cause "The process
	// cannot access the file because it is being used by another process." on windows.
	f.Close()

	return os.Rename(f.Name(), filename)
}

//--------------------------------------------------------------------------------

func Tempfile(prefix string) (*os.File, string) {
	file, err := ioutil.TempFile("", prefix)
	if err != nil {
		PanicCrisis(err)
	}
	return file, file.Name()
}

func Tempdir(prefix string) (*os.File, string) {
	tempDir := os.TempDir() + "/" + prefix + RandStr(12)
	err := EnsureDir(tempDir, 0700)
	if err != nil {
		panic(Fmt("Error creating temp dir: %v", err))
	}
	dir, err := os.Open(tempDir)
	if err != nil {
		panic(Fmt("Error opening temp dir: %v", err))
	}
	return dir, tempDir
}
