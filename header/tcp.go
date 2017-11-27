package header

import (
	"encoding/binary"

	"github.com/YaoZengzeng/yustack/types"
)

const (
	srcPort		= 0
	dstPort		= 2
	seqNum		= 4
	ackNum		= 8
	dataOffset	= 12
	tcpFlags	= 13
	winSize		= 14
	tcpChecksum	= 16
	urgentPtr	= 18	
)

// Flags that may be set in a TCP segment
const (
	TCPFlagFin	= 1	<< iota
	TCPFlagSyn
	TCPFlagRst
	TCPFlagPsh
	TCPFlagAck
	TCPFlagUrg
)

// TCPFields contains the fields of a TCP packet. It is used to describe the
// fields of a packet that needs to be encoded
type TCPFields struct {
	SrcPort 	uint16

	DstPort 	uint16

	SeqNum 		uint32

	AckNum 		uint32

	DataOffset 	uint8

	Flags 		uint8

	WindowSize	uint16

	Checksum 	uint16

	UrgentPointer uint16
}

// TCP represents a TCP header stored in a byte order
type TCP []byte

const (
	// TCPMinimumSize is the minimum size of a valid TCP packet
	TCPMinimumSize = 20

	// TCPProtocolNumber is TCP's transport protocol number
	TCPProtocolNumber types.TransportProtocolNumber	= 6
)

func (b TCP) SourcePort() uint16 {
	return binary.BigEndian.Uint16(b[srcPort:])
}

func (b TCP) DestinationPort() uint16 {
	return binary.BigEndian.Uint16(b[dstPort:])
}

func (b TCP) SequenceNumber() uint32 {
	return binary.BigEndian.Uint32(b[seqNum:])
}

func (b TCP) AckNumber() uint32 {
	return binary.BigEndian.Uint32(b[ackNum:])
}

func (b TCP) DataOffset() uint8 {
	return (b[dataOffset] >> 4) * 4
}

func (b TCP) Payload() []byte {
	return b[b.DataOffset():]
}

func (b TCP) Flags() uint8 {
	return b[tcpFlags]
}

func (b TCP) WindowSize() uint16 {
	return binary.BigEndian.Uint16(b[winSize:])
}
