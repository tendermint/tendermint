package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	stdlog "log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	auto "github.com/tendermint/tendermint/internal/libs/autofile"
	"github.com/tendermint/tendermint/libs/log"
)

const Version = "0.0.1"
const readBufferSize = 1024 // 1KB at a time

// Parse command-line options
func parseFlags() (headPath string, chopSize int64, limitSize int64, version bool, err error) {
	var flagSet = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	var chopSizeStr, limitSizeStr string
	flagSet.StringVar(&headPath, "head", "logjack.out", "Destination (head) file.")
	flagSet.StringVar(&chopSizeStr, "chop", "100M", "Move file if greater than this")
	flagSet.StringVar(&limitSizeStr, "limit", "10G", "Only keep this much (for each specified file). Remove old files.")
	flagSet.BoolVar(&version, "version", false, "Version")

	if err = flagSet.Parse(os.Args[1:]); err != nil {
		return
	}

	chopSize, err = parseByteSize(chopSizeStr)
	if err != nil {
		return
	}
	limitSize, err = parseByteSize(limitSizeStr)
	if err != nil {
		return
	}
	return
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer cancel()
	defer func() { fmt.Println("logjack shutting down") }()

	// Read options
	headPath, chopSize, limitSize, version, err := parseFlags()
	if err != nil {
		stdlog.Fatalf("problem parsing arguments: %q", err.Error())
	}

	if version {
		stdlog.Printf("logjack version %s", Version)
	}

	// Open Group
	group, err := auto.OpenGroup(ctx, log.NewNopLogger(), headPath, auto.GroupHeadSizeLimit(chopSize), auto.GroupTotalSizeLimit(limitSize))
	if err != nil {
		stdlog.Fatalf("logjack couldn't create output file %q", headPath)
	}

	if err = group.Start(ctx); err != nil {
		stdlog.Fatalf("logjack couldn't start with file %q", headPath)
	}

	// Forever read from stdin and write to AutoFile.
	buf := make([]byte, readBufferSize)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			if err == io.EOF {
				return
			}
			stdlog.Fatalln("logjack errored:", err.Error())
		}
		_, err = group.Write(buf[:n])
		if err != nil {
			stdlog.Fatalf("logjack failed write %q with error: %q", headPath, err.Error())
		}
		if err := group.FlushAndSync(); err != nil {
			stdlog.Fatalf("logjack flushsync %q fail with error: %q", headPath, err.Error())
		}
	}
}

func parseByteSize(chopSize string) (int64, error) {
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
		return 0, err
	}

	return int64(chopSizeInt) * multiplier, nil
}
