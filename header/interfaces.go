package header

import (
	"github.com/YaoZengzeng/yustack/types"
)

// Transport offers generic methods to query and/or update the fields of the
// header of a transport protocol buffer
type Transport interface {
	// Destination returns the value of the "destination port" field
	DestinationPort() uint16
}

// Network offers generic methods to query and/or update the fields of the
// header of a network protocol buffer
type Network interface {
	// SourceAddress returns the value of the "source address" field
	SourceAddress() types.Address

	// DestinationAddress returns the value of the "destination address"
	DestinationAddress() types.Address

	// TransportProtocol returns the number of the transport protocol
	// stored in the payload
	TransportProtocol() types.TransportProtocolNumber

	// Payload returns a byte slice containing the payload of the network
	// packet
	Payload() []byte
}