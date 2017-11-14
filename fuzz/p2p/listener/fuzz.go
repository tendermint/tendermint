package listener

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"

	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tmlibs/log"
)

func Fuzz(data []byte) int {
	str := string(data)
	splits := strings.Split(str, ",")
	var protocol, addr string
	protocol = splits[0]
	if len(splits) > 1 {
		addr = splits[1]
	}
	stack, err := recoverer(func() {
		buf := new(bytes.Buffer)
		lg := log.NewTMLogger(buf)
		ln := p2p.NewDefaultListener(protocol, addr, true, lg)
		ln.Stop()
	})

	if cannotBindErr(err) {
		// Nothing we can do about failing
		// to bind to an address :(
		return 0
	}
	shouldCrash := addrShouldCrash(protocol, addr)
	if err == nil && shouldCrash {
		panic(fmt.Errorf("%s should crash", addr))
	}
	if err != nil && !shouldCrash {
		panic(fmt.Errorf("%s should not crash:\nyet got crashed with stack: %s\nerr: %s", addr, stack, err))
	}

	// Successfully handled, increase priority
	return 1
}

var cannotBind = []string{
	"assign requested address",
	"bind: permission denied",
	"no such host",
}

func cannotBindErr(err error) bool {
	if err == nil {
		return false
	}
	for _, substr := range cannotBind {
		if strings.Contains(err.Error(), substr) {
			return true
		}
	}
	return false
}

func addrShouldCrash(protocol, addr string) bool {
	switch protocol {
	// This "known" protocol switch is from
	// https://github.com/golang/go/blob/f7df55d174b886f8aea0243aa40e8debffbdffc0/src/net/cgo_unix.go#L63-L68
	case "tcp", "tcp4", "tcp6", "udp", "udp4", "udp6":
	default: // All other networks are unknown
		return true
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return true
	}

	if strings.TrimSpace(port) == "" {
		return true
	}
	iPort, err := strconv.Atoi(port)
	if err != nil {
		return true
	}
	switch {
	case iPort < 0 || iPort > 65536:
		return true
	case iPort == 80 || iPort == 443:
		return true
	}
	return !canResolve(host)
}

func recoverer(fn func()) (stack []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			stack = debug.Stack()
			switch e := r.(type) {
			case error:
				err = e
			default:
				err = fmt.Errorf("%v", e)
			}
		}
	}()
	fn()
	return
}

var cachedIPLookups *sync.Map

type ipLookup struct {
	err error
	ips []net.IPAddr
}

func init() {
	cachedIPLookups = new(sync.Map)
}

func canResolve(host string) bool {
	// We do direct resolving because
	// If we get say "0.", this is invalid, we need valid full
	// octet-ed IP, except for "0.0.0" which seems to bind alright.
	// Contains only "0" and "."
	// because for example
	// "00.0" binds as per
	//   https://gist.github.com/odeke-em/e9dfdea4cedf99bcb948c80e123d5c9f
	// which is fuzzy to validate, so instead just do resolution.

	// Otherwise, if we encounter "", that corresponds to "localhost"
	if host == "" {
		return true
	}

	var ips []net.IPAddr
	var err error
	save, ok := cachedIPLookups.Load(host)
	if ok && save != nil {
		il, ok := save.(*ipLookup)
		if ok && il != nil {
			ips, err = il.ips, il.err
			goto resultOut
		}
		// Otherwise fallthrough and look iup
	}

	// Otherwise look it up
	ips, err = net.DefaultResolver.LookupIPAddr(context.TODO(), host)
	cachedIPLookups.Store(host, &ipLookup{err: err, ips: ips})

resultOut:
	return err == nil && len(ips) > 0
}
