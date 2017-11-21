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
	// Bind binds the endpoint to a specific local address and port
	// Specifying a Nic is optional
	Bind(address FullAddress) error

	// Read reads data from the endpoint and optionally returns the sender
	// This method does not block if there is no data pending
	// It will also either return an error or data, never both
	Read(*FullAddress) (buffer.View, error)

	// Write writes data to the endpoint's peer, or the provided address if
	// one is specified. This method does not block if the data cannot be written
	//
	// Note that unlike io.Write.Write, it is not an error for Write to perform a
	// partial write
	Write(buffer.View, *FullAddress) (uintptr, error)
}

// FullAddress represents a full transport node address, as required by the
// Connect() and Bind() methods
type FullAddress struct {
	// Nic is the Id of the Nic this address refers to
	// This may not be used by all endpoint types
	Nic NicId

	// Address is the network address
	Address Address

	// Port is the transport port
	//
	// This may not be used by all endpoint types
	Port uint16
}
