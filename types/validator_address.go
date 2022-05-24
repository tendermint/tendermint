package types

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	tmrand "github.com/tendermint/tendermint/libs/rand"
)

var (
	reSchemeIsHost = regexp.MustCompile(`^[^/:]+:\d+(/|$)`)
)

// ValidatorAddress is a ValidatorAddress that does not require node ID to be set
// We cannot just use p2p.ValidatorAddress because it causes dependency loop
type ValidatorAddress struct {
	NodeID   NodeID
	Hostname string
	Port     uint16
}

var (
	// ErrNoHostname is returned when no hostname is set for the validator address
	ErrNoHostname = errors.New("no hostname")
	// ErrNoPort is returned when no valid port is set for the validator address
	ErrNoPort = errors.New("no port")

	errEmptyAddress = errors.New("address is empty")
)

// ParseValidatorAddress parses provided address, which should be in `proto://nodeID@host:port` form.
// `proto://` and `nodeID@` parts are optional.
func ParseValidatorAddress(address string) (ValidatorAddress, error) {
	addr, err := parseValidatorAddressString(address)
	if err != nil {
		return ValidatorAddress{}, err
	}

	return addr, addr.Validate()
}

func stringHasScheme(str string) bool {
	return strings.Contains(str, "://")
}

// ParseValidatorAddressWithoutValidation  parses a node address URL into a ValidatorAddress, normalizing it.
// It does NOT validate parsed address
func parseValidatorAddressString(urlString string) (ValidatorAddress, error) {
	if urlString == "" {
		return ValidatorAddress{}, nil
	}
	// url.Parse requires a scheme, so if it fails to parse a scheme-less URL
	// we try to apply a default scheme.
	url, err := url.Parse(urlString)
	if (err != nil || url.Scheme == "") &&
		(!stringHasScheme(urlString) || reSchemeIsHost.MatchString(urlString)) {
		url, err = url.Parse("tcp://" + urlString)
	}
	if err != nil {
		return ValidatorAddress{}, fmt.Errorf("invalid node address %q: %w", urlString, err)
	}

	address := ValidatorAddress{}

	// Opaque URLs are expected to contain only a node ID.
	if url.Opaque != "" {
		address.NodeID = NodeID(url.Opaque)
		return address, nil
	}

	// Otherwise, just parse a normal networked URL.
	if url.User != nil {
		address.NodeID = NodeID(strings.ToLower(url.User.Username()))
	}

	address.Hostname = strings.ToLower(url.Hostname())

	if portString := url.Port(); portString != "" {
		port64, err := strconv.ParseUint(portString, 10, 16)
		if err != nil {
			return ValidatorAddress{}, fmt.Errorf("invalid port %q: %w", portString, err)
		}
		address.Port = uint16(port64)
	}

	return address, nil
}

// Validate ensures the validator address is correct.
// It ignores missing node IDs.
func (va ValidatorAddress) Validate() error {
	if va.Zero() {
		return errEmptyAddress
	}
	if va.Hostname == "" {
		return ErrNoHostname
	}
	if va.Port <= 0 {
		return ErrNoPort
	}
	if len(va.NodeID) > 0 {
		if err := va.NodeID.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Zero returns true if the ValidatorAddress is not initialized
func (va ValidatorAddress) Zero() bool {
	return va.Hostname == "" && va.Port == 0 && va.NodeID == ""
}

// String formats the address as a URL string.
func (va ValidatorAddress) String() string {
	if va.Zero() {
		return ""
	}

	address := "tcp://"
	if va.NodeID != "" {
		address += string(va.NodeID) + "@"

	}
	address += net.JoinHostPort(va.Hostname, strconv.Itoa(int(va.Port)))
	return address
}

// NetAddress converts ValidatorAddress to a NetAddress object
func (va ValidatorAddress) NetAddress() (*NetAddress, error) {
	return ParseAddressString(va.String())
}

// RandValidatorAddress generates a random validator address. Used in tests.
// It will panic in (very unlikely) case of error.
func RandValidatorAddress() ValidatorAddress {
	nodeID := tmrand.Bytes(20)
	port := rand.Int()%math.MaxUint16 + 1 // nolint
	addr, err := ParseValidatorAddress(fmt.Sprintf("tcp://%x@127.0.0.1:%d", nodeID, port))
	if err != nil {
		panic(fmt.Sprintf("cannot generate random validator address: %s", err))
	}
	if err := addr.Validate(); err != nil {
		panic(fmt.Sprintf("randomly generated validator address %s is invalid: %s", addr.String(), err))
	}
	return addr
}
