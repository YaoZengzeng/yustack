package types

// LinkAddress is a byte slice cast as a string that represents a link address.
// It is typically a 6-byte MAC address
type LinkAddress string

// LinkEndpointID represents a data link layer endpoint
type LinkEndpointID uint64

// LinkEndpoint is the interface implemented by data link layer protocols (e.g.,
// ethernet, loopback, raw) and used by network layer protocols to send packets
// out through the implementer's data link endpoint
type LinkEndpoint interface {
	// MTU is the maximum transmission unit for this endpoint. This is usually dictated
	// by the backing physical network; when such a physical network doesn't exist, the
	// limit is generally 64k, which includes the maximum size of an IP packet
	MTU() uint32

	// LinkAddress returns the link address (typically a MAC) of the link endpoint
	LinkAddress() LinkAddress

	// MaxHeaderLength returns the maximum size of the data link (and lower level layers
	// combined) headers can have.Higher levels use this information to reserve space in
	// front of the packets they're building
	MaxHeaderLength() uint16
}