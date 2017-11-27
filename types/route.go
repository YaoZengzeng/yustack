package types

import (
	"github.com/YaoZengzeng/yustack/checksum"
	"github.com/YaoZengzeng/yustack/buffer"
)

// RouteEntry is a row in the routing table. It specifies through which Nic (and
// gateway) sets of packets should be routed. A row is considered viable if the
// masked target address matches the destination address in the row
type RouteEntry struct {
	// Destination is the address that must be matched against the masked
	// target address to check if this row is viable
	Destination 	Address

	// Mask specifies which bits of the Destination and the target address
	// must match for this row to be viable
	Mask 			Address

	// Gateway is the gateway to be used if this row is viable
	Gateway 		Address

	// Nic is the id of the nic to be used if this row is viable
	Nic 			NicId
}

// Match determines if r is viable for the given destination address
func (r *RouteEntry) Match(addr Address) bool {
	if len(addr) != len(r.Destination) {
		return false
	}

	for i := 0; i < len(r.Destination); i++ {
		if (addr[i] & r.Mask[i]) != r.Destination[i] {
			return false
		}
	}

	return true
}

// Route represents a route through the networking stack to a given destination
type Route struct {
	// RemoteAddress is the final destination of the route
	RemoteAddress 		Address

	// RemoteLinkAddress is the link layer (MAC) address of the
	// final destination of the route
	RemoteLinkAddress 	LinkAddress

	// LocalAddress is the local address where the route starts
	LocalAddress 		Address

	// LocalLinkAddress is the link layer (MAC) address of where the route starts
	LocalLinkAddress 	LinkAddress

	// NextHop is the next node in the path to the destination
	NextHop 			Address

	// NetProto	is the network layer protocol
	NetProto 			NetworkProtocolNumber

	// NetEp is the network endpoint through which the route starts
	NetEp				NetworkEndpoint
}

// MaxHeaderLength forwards the call to the network endpoint's implementation
func (r *Route) MaxHeaderLength() uint16 {
	return r.NetEp.MaxHeaderLength()
}

// WritePacket writes the packet through the given route
func (r *Route) WritePacket(hdr *buffer.Prependable, payload buffer.View, protocol TransportProtocolNumber) error {
	return r.NetEp.WritePacket(r, hdr, payload, protocol)
}

// MakeRoute initializes a new route. It takes ownership of the provided
// reference to a network endpoint
func MakeRoute(netProto NetworkProtocolNumber, localAddr, remoteAddr Address, netEp NetworkEndpoint) *Route {
	return &Route{
		NetProto:		netProto,
		LocalAddress:	localAddr,
		RemoteAddress:	remoteAddr,
		NetEp:			netEp,
	}
}

// PseudoHeaderChecksum forwards the call to the network endpoint's
// implementation
func (r *Route) PseudoHeaderChecksum(protocol TransportProtocolNumber) uint16 {
	return checksum.PseudoHeaderChecksum(uint32(protocol), string(r.LocalAddress), string(r.RemoteAddress))
}

// NicId returns the id of the Nic from which this route originates
func (r *Route) NicId() NicId {
	return r.NetEp.NicId()
}

// Clone clone a route such that the original one can be released and the new
// one will remain valid
func (r *Route) Clone() Route {
	// Just walk around for simplicity
	return *r
}