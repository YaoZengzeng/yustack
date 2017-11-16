package types

import (
	"github.com/YaoZengzeng/yustack/buffer"
)

// Address is a byte slice cast as a string that represents the address of a
// network node. Or, when we support the case of unix endpoints, it may represent a path.
type Address string

// NicId is a number that uniquely identifies a Nic
type NicId int32

// NetworkDispatcher contains the methods used by the network stack to deliver
// packets to the appropriate network endpoint after it has been handled by the
// data link layer
type NetworkDispatcher interface {
	// DeliverNetworkPacket finds the appropriate network protocol
	// endpoint and hands the packet for further processing
	DeliverNetworkPacket(linkEp LinkEndpoint, remoteLinkAddr LinkAddress, protocol NetworkProtocolNumber, vv *buffer.VectorisedView)
}

// TransportDispatcher contains the methods used by the network stack to deliver
// packets to the appropriate transport endpoint after it has been handled by the
// network layer
type TransportDispatcher interface {
	// DeliverTransportPacket delivers the packets to the appropriate
	// transport protocol endpoint
	DeliverTransportPacket(r *Route, protocol TransportProtocolNumber, vv *buffer.VectorisedView)
}

// Endpoint is the interface implemented by transport protocols (e.g., tcp, udp)
// that exposes functionality link read, write, connect, etc to uses of the networking
// stack
type Endpoint interface {
	
}