package types

import (
	"github.com/YaoZengzeng/yustack/buffer"
)

// TransportProtocolNumber is the number of a transport protocol
type TransportProtocolNumber uint32

// TransportProtocolFactory functions are used by the stack to instantiate
// transport protocol
type TransportProtocolFactory func() TransportProtocol

// TransportProtocol is the interface that needs to be implemented by transport
// protocol (e.g., tcp, udp) that want to be part of the networking stack
type TransportProtocol interface {
	// Number returns the transport protocol number
	Number() TransportProtocolNumber

	// MinimumPacketSize returns the minimum valid packet size of this
	// transport protocol. The stack automatically drops any packets smaller
	// than this targeted at this protocol
	MinimumPacketSize() int

	// ParsePorts returns the source and destination ports stored in a
	// packet of this protocol
	ParsePorts(v buffer.View) (src, dst uint16, err error)
}

type TransportProtocolState struct {
	Protocol 		TransportProtocol
}

// TransportEndpointId is the identifier of a transport layer protocol endpoint
type TransportEndpointId struct {
	// LocalPort is the local port associated with the endpoint
	LocalPort		uint16

	// LocalAddress is the local [network layer] address associated with
	// the endpoint
	LocalAddress	Address

	// RemotePort is the remote port associated with the endpoint
	RemotePort		uint16

	// RemoteAddress is the remote [network layer] address associated with
	// the endpoint
	RemoteAddress	Address
}

// TransportEndpoint is the interface that needs to be implemented by transport
// protocol (e.g., tcp, udp) endpoints that can handle packets
type TransportEndpoint interface {
	// HandlePacket is called by the stack when new packets arrive to
	// this transport endpoint
	HandlePacket(r *Route, id TransportEndpointId, vv *buffer.VectorisedView)
}
