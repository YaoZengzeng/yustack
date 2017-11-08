package header

import (
	"github.com/YaoZengzeng/yustack/types"
)

const (
	versIHL		=	0
	tos 		=	1
	totalLen	= 	2
	id 			=	4
	flagsFO		=	6
	ttl 		=	8
	protocol 	=	9
	checksum	=	10
	srcAddr		=	12
	dstAddr		=	16
)

// IPv4 represents an ipv4 header stored in a byte array
// Most of the methods of IPv4 access to the underlying slcie without
// checking the boundaries and could panic because of 'index out of range'
// Always call IsValid() to validate an instance of IPv4 before using other methods
type IPv4 []byte

const (
	// IPv4MinimumSize is the minimum size of a valid IPv4 packet
	IPv4MinimumSize = 20

	// IPv4AddressSize is the size, in bytes, of an IPv4 address
	IPv4AddressSize = 4

	// IPv4ProtocolNumber is IPv4's network protocol number
	IPv4ProtocolNumber types.NetworkProtocolNumber = 0x0800

	// IPv4 version is the version of the ipv4 protocol
	IPv4Version = 4
)

// IPVersion returns the version of IP used in the given packet. It returns -1
// it the packet is not large enough to contain the version field
func IPVersion(b []byte) int {
	// Length must be at least offset+length of version field
	if len(b) < versIHL + 1 {
		return -1
	}
	return int(b[versIHL] >> 4)
}

// SourceAddress returns the "source address" field of the ipv4 header
func (b IPv4) SourceAddress() types.Address {
	return types.Address(b[srcAddr : srcAddr + IPv4AddressSize])
}

// DestinationAddress returns the "destination address" field of the ipv4 header
func (b IPv4) DestinationAddress() types.Address {
	return types.Address(b[dstAddr : dstAddr + IPv4AddressSize])
}
