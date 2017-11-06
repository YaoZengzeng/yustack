package types

// NetworkProtocolNumber is the number of a network protocol
type NetworkProtocolNumber uint32

// NetworkProtocol is the interface that needs to be implemented by network
// protocols (e.g., ipv4, ipv6) that want to be part of the networking stack.
type NetworkProtocol interface {
	// Number returns the network protocol number.
	Number() NetworkProtocolNumber
}

// NetworkProtocolFactory provides methods to be used by the stack to
// instantiate network protocols.
type NetworkProtocolFactory func() NetworkProtocol
