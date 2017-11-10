package header

import (
	"encoding/binary"

	"github.com/YaoZengzeng/yustack/types"
)

type ICMPv4 []byte

const (
	// ICMPv4MinimumSize is the minimum size of a valid ICMP packet
	ICMPv4MinimumSize = 4

	// ICMPv4ProtocolNumber is the ICMP transport protocol number
	ICMPv4ProtocolNumber types.TransportProtocolNumber = 1
)

// ICMPv4Type is the ICMP type field described in RFC 792
type ICMPv4Type byte

// Typical values of ICMPv4Type defined in RFC 792
const (
	ICMPv4EchoReply			ICMPv4Type = 0
	ICMPv4Echo 				ICMPv4Type = 8
)

// Type is the ICMP type field
func (b ICMPv4) Type() ICMPv4Type {
	return ICMPv4Type(b[0])
}

// SetType sets the ICMP type field
func (b ICMPv4) SetType(t ICMPv4Type) { b[0] = byte(t) }

// Code is the ICMP code field. Its meaning depends on the value of Type
func (b ICMPv4) Code() byte { return b[1] }

// SetCode sets the ICMP code field
func (b ICMPv4) SetCode(c byte) { b[1] = c }

// SetChecksum sets the ICMP checksum field
func (b ICMPv4) SetChecksum(checksum uint16) {
	binary.BigEndian.PutUint16(b[2:], checksum)
}
