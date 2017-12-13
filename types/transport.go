package types

import (
	"github.com/YaoZengzeng/yustack/buffer"
)

// TransportProtocolNumber is the number of a transport protocol
type TransportProtocolNumber uint32

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

// ReceiveBufferSizeOption is used by SetSockOpt/GetSockOpt to specify the
// receive buffer size option
type ReceiveBufferSizeOption int
