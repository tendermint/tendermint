package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ds-test-framework/scheduler/config"
	"github.com/ds-test-framework/scheduler/testlib"
	"github.com/ds-test-framework/tendermint-test/testcases/rskip"
)

func main() {

	termCh := make(chan os.Signal, 1)
	signal.Notify(termCh, os.Interrupt, syscall.SIGTERM)

	server, err := testlib.NewTestingServer(
		&config.Config{
			APIServerAddr: "172.28.45.124:7074",
			NumReplicas:   4,
			LogConfig: config.LogConfig{
				Path: "/tmp/tendermint/log/checker.log",
			},
		},
		[]*testlib.TestCase{
			// testcases.DummyTestCase(),
			rskip.OneTestcase(1, 2),
			// lockedvalue.One(),
			// lockedvalue.Two(),
			// lockedvalue.Three(),
			// sanity.OneTestCase(),
			// sanity.TwoTestCase(),
			// sanity.ThreeTestCase(),
			// sanity.HigherProp(),
			// bfttime.OneTestCase(),
		},
	)

	if err != nil {
		fmt.Printf("Failed to start server: %s\n", err.Error())
		os.Exit(1)
	}

	go func() {
		<-termCh
		server.Stop()
	}()

	server.Start()

}
