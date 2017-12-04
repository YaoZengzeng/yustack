package header

import (
	"log"
	"encoding/binary"

	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/checksum"
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

const (
	// MaxWndScale is the maximum allowed window scaling
	MaxWndScale = 14
)

// Options that may be present in a TCP segment
const (
	TCPOptionEOL = 0
	TCPOptionNOP = 1
	TCPOptionMSS = 2
	TCPOptionWS  = 3
	TCPOptionTS	 = 8
)

const (
	debug = true
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

// TCPSynOptions is used to return the parsed TCP options in a syn segment
type TCPSynOptions struct {
	// MSS is the maximum segment size provided by the peer in the SYN
	MSS	uint16

	// WS is the window scale option provided by the peer in the SYN
	WS 	int

	// TS is true if the timestamp option was provided in the syn/syn-ack
	TS bool

	// TSVal is the value of the TSVal field in the timestamp option
	TSVal uint32

	// TSEcr is the value of the TSEcr field in the timestamp option
	TSEcr uint32
}

// TCPOptions are used to parse and cache the TCP segment options for a non
// syn/syn-ack segment
type TCPOptions struct {
	// TS is true if the TimeStamp option is enabled
	TS bool

	// TSVal is the value in the TSVal field of the segment
	TSVal uint32

	// TSEcr is the value in the TSEcr field of the segment
	TSEcr uint32
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

// SetChecksum sets the checksum field of the tcp header
func (b TCP) SetChecksum(checksum uint16) {
	binary.BigEndian.PutUint16(b[tcpChecksum:], checksum)
}

// CalculateChecksum calculates the checksum of the tcp segment given
// the totalLen and partialChecksum
// totalLen is the total length of the segment
// partialChecksum is the checksum of the network-layer pseudo-header
// (excluding the total length) and the checksum of the segment data
func (b TCP) CalculateChecksum(partialChecksum uint16, totalLen uint16) uint16 {
	// Add the length portion of the checksum to the pseudo-checksum
	tmp := make([]byte, 2)
	binary.BigEndian.PutUint16(tmp, totalLen)
	cksm := checksum.Checksum(tmp, partialChecksum)

	// Calculate the rest of the checksum
	return checksum.Checksum(b[:b.DataOffset()], cksm)
}

// Encode encodes all the fields of the tcp header
func (b TCP) Encode(t *TCPFields) {
	binary.BigEndian.PutUint32(b[seqNum:], t.SeqNum)
	binary.BigEndian.PutUint32(b[ackNum:], t.AckNum)
	b[tcpFlags] = t.Flags
	binary.BigEndian.PutUint16(b[winSize:], t.WindowSize)
	binary.BigEndian.PutUint16(b[srcPort:], t.SrcPort)
	binary.BigEndian.PutUint16(b[dstPort:], t.DstPort)
	b[dataOffset] = (t.DataOffset / 4) << 4
	binary.BigEndian.PutUint16(b[tcpChecksum:], t.Checksum)
	binary.BigEndian.PutUint16(b[urgentPtr:], t.UrgentPointer)
}

// ParseSynOptions parses the options received in a SYN segment and returns the
// relevant ones. opts should point to the option part of the TCP header
func ParseSynOptions(opts []byte, isAck bool) TCPSynOptions {
	limit := len(opts)

	synOpts := TCPSynOptions{
		// If an MSS option is not received at connection setup,
		// TCP MUST assume a default send MSS of 536
		MSS:	536,

		// If no window scale option is specified, WS in options is
		// returned as -1; this is because the absence of the option
		// indicates that we can't use window scaling on the receive
		// end either
		WS:		-1,
	}

	for i := 0; i < limit; {
		switch opts[i] {
		case TCPOptionEOL:
			// End of Option List
			i = limit
		case TCPOptionNOP:
			// No-Operation
			i++
		case TCPOptionMSS:
			// Maximum Segment Size -> length is 4
			if i + 4 > limit || opts[i + 1] != 4 {
				return synOpts
			}
			mss := uint16(opts[i + 2]) << 8 | uint16(opts[i + 3])
			if mss == 0 {
				return synOpts
			}
			synOpts.MSS = mss
			i += 4
			if debug {
				log.Printf("ParseSynOptions: TCPOptionMSS mss is %v\n", mss)
			}
		case TCPOptionWS:
			// Window Scale -> length is 3
			if i + 3 > limit || opts[i + 1] != 3 {
				return synOpts
			}
			ws := int(opts[i + 2])
			if ws > MaxWndScale {
				ws = MaxWndScale
			}
			synOpts.WS = ws
			i += 3
			if debug {
				log.Printf("ParseSynOptions: TCPOptionWS window scale is %v\n", ws)
			}
		case TCPOptionTS:
			// TimeStamp -> length is 10
			if i + 10 > limit || opts[i + 1] != 10 {
				return synOpts
			}
			synOpts.TSVal = binary.BigEndian.Uint32(opts[i + 2:])
			if isAck {
				// If the segment is a SYN-ACK then store the TimeStamp Echo Reply
				// in the segment
				synOpts.TSEcr = binary.BigEndian.Uint32(opts[i + 6:])
			}
			synOpts.TS = true
			i += 10
			if debug {
				log.Printf("ParseSynOptions: TCPOptionTS\n")
			}
		default:
			// We don't recognize this option, just skip over it
			if i + 2 > limit {
				return synOpts
			}
			l := int(opts[i + 1])
			// If the length is incorrect or if l + i overflows the
			// total options length then return false
			if l < 2 || i + l > limit {
				return synOpts
			}
			i += l
		}
	}

	return synOpts
}

// ParseTCPOptions extracts and stores all known options in the provided byte
// slice in a TCPOptions structure
func ParseTCPOptions(b []byte) TCPOptions {
	opts := TCPOptions{}
	limit := len(b)

	for i := 0; i < limit; {
		switch b[i] {
		case TCPOptionEOL:
			i = limit
		case TCPOptionNOP:
			i++
		case TCPOptionTS:
			if i + 10 > limit || b[i + 1] != 10 {
				return opts
			}
			if debug {
				log.Printf("ParseTCPOptions: TCPOptionTS\n")
			}
			opts.TS = true
			opts.TSVal = binary.BigEndian.Uint32(b[i + 2:])
			opts.TSEcr = binary.BigEndian.Uint32(b[i + 6:])
			i += 10
		default:
			// We don't recognize this option, just skip over it
			if i + 2 > limit {
				return opts
			}
			l := int(b[i + 1])
			// If the length is incorrect or if l + i overflows the
			// total options length then return false
			if l < 2 || i + l > limit {
				return opts
			}
			i += l
		}
	}
	return opts
}
