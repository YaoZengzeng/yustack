package types

import (
	"github.com/YaoZengzeng/yustack/buffer"
)

// NetworkProtocolNumber is the number of a network protocol
type NetworkProtocolNumber uint32

// NetworkProtocol is the interface that needs to be implemented by network
// protocols (e.g., ipv4, ipv6) that want to be part of the networking stack.
type NetworkProtocol interface {
	// Number returns the network protocol number.
	Number() NetworkProtocolNumber

	// ParseAddresses returns the source and destination addresses stored in a
	// packet of this protocol
	ParseAddresses(v buffer.View) (src, dst Address)

	// NewEndpoint creates a new endpoint of this protocol
	NewEndpoint(nicid NicId, addr Address, dispatcher TransportDispatcher, sender LinkEndpoint) (NetworkEndpoint, error)

	// MinimumPacketSize returns the minimum valid packet size of this
	// network protocol. The stack automatically drops any packets smaller
	// than this targeted at this protocol
	MinimumPacketSize() int
}

// NetworkProtocolFactory provides methods to be used by the stack to
// instantiate network protocols.
type NetworkProtocolFactory func() NetworkProtocol

// NetworkEndpointId is the identifier of a network layer protocol endpoint
// Currently the local address is sufficient because all supported protocols
// (i.e., IPv4) have different sizes for their addresses
type NetworkEndpointId struct {
	LocalAddress	Address
}

// NetworkEndpoint is the interface that needs to be implemented by endpoints
// of network layer protocols (eg., ipv4)
type NetworkEndpoint interface {
	// MTU is the maximum transmission unit for this endpoint. This is
	// generally calculated as the MTU of the underlying data link endpoint
	// minus the network endpoint max header length
	MTU() uint32

	// HandlePacket is called by the link layer when new packets arrive to
	// this network endpoint
	HandlePacket(r *Route, vv *buffer.VectorisedView)

	// Id returns the network protocol endpoint Id
	Id() *NetworkEndpointId

	// MaxHeaderLength returns the maximum size the network (and lower
	// level layers combined) headers can have. Higher levels use this
	// information to reserve space in the front of the packets they're
	// building
	MaxHeaderLength() uint16

	// WritePacket writes the packet to the given destination address and protocol
	WritePacket(r *Route, hdr *buffer.Prependable, payload buffer.View, protocol TransportProtocolNumber) error

	// NicId returns the id of the Nic this endpoint belongs to
	NicId() NicId
}
