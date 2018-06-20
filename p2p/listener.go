package p2p

type Listener interface {
	ExternalAddress() NetAddress
	String() string
}
