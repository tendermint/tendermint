package common

import (
	"fmt"
	"os"
	"os/signal"
)

func TrapSignal(cb func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, os.Kill)
	go func() {
		for sig := range c {
			fmt.Printf("captured %v, exiting...\n", sig)
			if cb != nil {
				cb()
			}
			os.Exit(1)
		}
	}()
	select {}
}

func Exit(s string) {
	fmt.Printf(s + "\n")
	os.Exit(1)
}
