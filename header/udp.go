package header

import (
	"encoding/binary"

	"github.com/YaoZengzeng/yustack/types"
)

const (
	udpSrcPort	= 0
	udpDstPort	= 2
	udpLength	= 4
	udpChecksum	= 6
)

// UDPFields contains the fields of a UDP packet. It is used to describe the
// fields of a packet that needs to be encoded
type UDPFields struct {
	// SrcPort is the "source port" field of a UDP packet
	SrcPort uint16

	// DstPort is the "destination port" field of a UDP packet
	DstPort uint16

	// Length is the "length" field of a UDP packet
	Length uint16

	// Checkum is the "checksum" field of a UDP packet
	Checkum uint16
}

const (
	// UDPMinimumSize is the minimum size of a valid UDP packet
	UDPMinimumSize = 8

	// UDPProtocolNumber is UDP's transport protocol number
	UDPProtocolNumber types.TransportProtocolNumber = 17
)

// UDP represents a UDP header stored in a byte array
type UDP []byte

// SourcePort returns the "source port" field  of the udp header
func (b UDP) SourcePort() uint16 {
	return binary.BigEndian.Uint16(b[udpSrcPort:])
}

// DestinationPort returns the "destination port" field of the udp header
func (b UDP) DestinationPort() uint16 {
	return binary.BigEndian.Uint16(b[udpDstPort:])
}
