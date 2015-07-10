package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	. "github.com/tendermint/tendermint/common"
)

const Version = "0.0.1"
const sleepSeconds = 1 // Every minute

// Parse command-line options
func parseFlags() (chopSize int64, limitSize int64, version bool, logFiles []string) {
	var chopSizeStr, limitSizeStr string
	flag.StringVar(&chopSizeStr, "chopSize", "1M", "Move file if greater than this")
	flag.StringVar(&limitSizeStr, "limitSize", "1G", "Only keep this much (for each specified file). Remove old files.")
	flag.BoolVar(&version, "version", false, "Version")
	flag.Parse()
	logFiles = flag.Args()
	chopSize = parseBytesize(chopSizeStr)
	limitSize = parseBytesize(limitSizeStr)
	return
}

func main() {

	// Read options
	chopSize, limitSize, version, logFiles := parseFlags()
	if version {
		fmt.Println(Fmt("logjack version %v", Version))
		return
	}

	// Print args.
	// fmt.Println(chopSize, limitSiz,e version, logFiles)

	go func() {
		for {
			for _, logFilePath := range logFiles {
				minIndex, maxIndex, totalSize, baseSize := readLogInfo(logFilePath)
				if chopSize < baseSize {
					moveLog(logFilePath, Fmt("%v.%03d", logFilePath, maxIndex+1))
				}
				if limitSize < totalSize {
					// NOTE: we only remove one file at a time.
					removeLog(Fmt("%v.%03d", logFilePath, minIndex))
				}
			}
			time.Sleep(sleepSeconds * time.Second)
		}
	}()

	// Trap signal
	TrapSignal(func() {
		fmt.Println("logjack shutting down")
	})
}

func moveLog(oldPath, newPath string) {
	err := os.Rename(oldPath, newPath)
	if err != nil {
		panic(err)
	}
}

func removeLog(path string) {
	err := os.Remove(path)
	if err != nil {
		panic(err)
	}
}

// This is a strange function.  Refactor everything to make it less strange?
func readLogInfo(logPath string) (minIndex, maxIndex int, totalSize int64, baseSize int64) {

	logDir := filepath.Dir(logPath)
	logFile := filepath.Base(logPath)
	minIndex, maxIndex = -1, -1

	dir, err := os.Open(logDir)
	if err != nil {
		panic(err)
	}
	fi, err := dir.Readdir(0)
	if err != nil {
		panic(err)
	}
	for _, fileInfo := range fi {
		indexedFilePattern := regexp.MustCompile("^.+\\.([0-9]{3,})$")
		if fileInfo.Name() == logFile {
			baseSize = fileInfo.Size()
			continue
		} else if strings.HasPrefix(fileInfo.Name(), logFile) {
			totalSize += fileInfo.Size()
			submatch := indexedFilePattern.FindSubmatch([]byte(fileInfo.Name()))
			if len(submatch) != 0 {
				// Matches
				logIndex, err := strconv.Atoi(string(submatch[1]))
				if err != nil {
					panic(err)
				}
				if maxIndex < logIndex {
					maxIndex = logIndex
				}
				if minIndex == -1 || logIndex < minIndex {
					minIndex = logIndex
				}
			}
		}
	}
	return minIndex, maxIndex, totalSize, baseSize
}

func parseBytesize(chopSize string) int64 {
	// Handle suffix multiplier
	var multiplier int64 = 1
	if strings.HasSuffix(chopSize, "T") {
		multiplier = 1042 * 1024 * 1024 * 1024
		chopSize = chopSize[:len(chopSize)-1]
	}
	if strings.HasSuffix(chopSize, "G") {
		multiplier = 1042 * 1024 * 1024
		chopSize = chopSize[:len(chopSize)-1]
	}
	if strings.HasSuffix(chopSize, "M") {
		multiplier = 1042 * 1024
		chopSize = chopSize[:len(chopSize)-1]
	}
	if strings.HasSuffix(chopSize, "K") {
		multiplier = 1042
		chopSize = chopSize[:len(chopSize)-1]
	}

	// Parse the numeric part
	chopSizeInt, err := strconv.Atoi(chopSize)
	if err != nil {
		panic(err)
	}

	return int64(chopSizeInt) * multiplier
}
