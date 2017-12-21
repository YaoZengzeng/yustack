package checker

import (
	"encoding/binary"
	"testing"

	"github.com/YaoZengzeng/yustack/header"
	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/checksum"

)

// NetworkChecker is a function to check a property of a network packet
type NetworkChecker func(*testing.T, []header.Network)

// TransportChecker is a function to check a property of a transport packet
type TransportChecker func(*testing.T, header.Transport)

// IPv4 checks the validity and properties of the given IPv4 packet. It is
// expected to be used in conjunction with other network checkers for specific
// properties. For example, to check the source and destination address, one
// would call:
//
// checker.IPv4(t, b, checker.SrcAddr(x), checker.DstAddr(y))
func IPv4(t *testing.T, b []byte, checkers ...NetworkChecker) {
	ipv4 := header.IPv4(b)

	if !ipv4.IsValid(len(b)) {
		t.Fatalf("Not a valid IPv4 packet")
	}

	xsum := ipv4.CalculateChecksum()
	if xsum != 0 && xsum != 0xffff {
		t.Fatalf("Bad checksum: 0x%x, checksum in packet: 0x%x", xsum, ipv4.Checksum())
	}

	for _, f := range checkers {
		f(t, []header.Network{ipv4})
	}
}

// SrcAddr creates a checker that checks the source address
func SrcAddr(addr types.Address) NetworkChecker {
	return func(t *testing.T, h []header.Network) {
		if a := h[0].SourceAddress(); a != addr {
			t.Fatalf("Bad source address, got %v, want %x", a, addr)
		}
	}
}

// DstAddr calculates a checker that checks the destination address
func DstAddr(addr types.Address) NetworkChecker {
	return func(t *testing.T, h []header.Network) {
		if a := h[0].DestinationAddress(); a != addr {
			t.Fatalf("Bad destination address, got %v, want %v", a, addr)
		}
	}
}

// PayloadLen creates a checker that checks the payload length
func PayloadLen(plen int) NetworkChecker {
	return func(t *testing.T, h []header.Network) {
		if l := len(h[0].Payload()); l != plen {
			t.Fatalf("Bad payload length, got %v, want %v", l, plen)
		}
	}
}

// TCP creates a checker that checks the transport protocol is TCP and
// potentially additional transport header fields
func TCP(checkers ...TransportChecker) NetworkChecker {
	return func(t *testing.T, h []header.Network) {
		first := h[0]
		last := h[len(h) - 1]

		if p := last.TransportProtocol(); p != header.TCPProtocolNumber {
			t.Fatalf("Bad protocol, got %v, want %v", p, header.TCPProtocolNumber)
		}

		// Verify the checksum
		tcp := header.TCP(last.Payload())
		l := uint16(len(tcp))

		xsum := checksum.Checksum([]byte(first.SourceAddress()), 0)
		xsum = checksum.Checksum([]byte(first.DestinationAddress()), xsum)
		xsum = checksum.Checksum([]byte{0, byte(last.TransportProtocol())}, xsum)
		xsum = checksum.Checksum([]byte{byte(l >> 8), byte(l)}, xsum)
		xsum = checksum.Checksum(tcp, xsum)

		if xsum != 0 && xsum != 0xffff {
			t.Fatalf("Bad checksum: 0x%x, checksum in segment: 0x%x", xsum, tcp.Checksum())
		}

		// Run the transport checkers
		for _, f := range checkers {
			f(t, tcp)
		}
	}
}

// SrcPort creates a checker that checks the source port
func SrcPort(port uint16) TransportChecker {
	return func(t *testing.T, h header.Transport) {
		if p := h.SourcePort(); p != port {
			t.Fatalf("Bad source port, got %v, want %v", p, port)
		}
	}
}

// DstPort creates a checker that checks the destination port
func DstPort(port uint16) TransportChecker {
	return func(t *testing.T, h header.Transport) {
		if p := h.DestinationPort(); p != port {
			t.Fatalf("Bad destination port, got %v, want %v", p, port)
		}
	}
}

// SeqNum creates a checker that checks the sequence number
func SeqNum(seq uint32) TransportChecker {
	return func(t *testing.T, h header.Transport) {
		tcp, ok := h.(header.TCP)
		if !ok {
			return
		}

		if s := tcp.SequenceNumber(); s != seq {
			t.Fatalf("Bad sequence number, got %v, want %v", s, seq)
		}
	}
}

// AckNum creates a checker that checks the ack number
func AckNum(seq uint32) TransportChecker {
	return func(t *testing.T, h header.Transport) {
		tcp, ok := h.(header.TCP)
		if !ok {
			return
		}

		if s := tcp.AckNumber(); s != seq {
			t.Fatalf("Bad ack number, got %v, want %v", s, seq)
		}
	}
}

// TCPFlags creates a checker that checks the tcp flags
func TCPFlags(flags uint8) TransportChecker {
	return func(t *testing.T, h header.Transport) {
		tcp, ok := h.(header.TCP)
		if !ok {
			return
		}

		if f := tcp.Flags(); f != flags {
			t.Fatalf("Bad flags, got 0x%x, want 0x%x", f, flags)
		}
	}
}

// Window creates a checker that checks the tcp window
func Window(window uint16) TransportChecker {
	return func(t *testing.T, h header.Transport) {
		tcp, ok := h.(header.TCP)
		if !ok {
			return
		}

		if w := tcp.WindowSize(); w != window {
			t.Fatalf("Bad window, got 0x%x, want 0x%x", w, window)
		}
	}
}

// TCPFlagsMatch creates a checker that checks the tcp flags, masked by the
// given mask, match the supplied flags
func TCPFlagsMatch(flags, mask uint8) TransportChecker {
	return func(t *testing.T, h header.Transport) {
		tcp, ok := h.(header.TCP)
		if !ok {
			return
		}

		if f := tcp.Flags(); (f & mask) != (flags & mask) {
			t.Fatalf("Bad masked flags, got 0x%x, want 0x%x, mask 0x%x", f, flags, mask)
		}
	}
}

// TCPSynOptions creates a checker that checks the presence of TCP options in
// SYN segments.
//
// If wndScale is negative, the window scale option must not be present
func TCPSynOptions(wantOpts header.TCPSynOptions) TransportChecker {
	return func(t *testing.T, h header.Transport) {
		tcp, ok := h.(header.TCP)
		if !ok {
			return
		}

		opts := tcp.Options()
		limit := len(opts)
		foundMSS := false
		foundWS := false
		foundTS := false
		tsVal := uint32(0)
		tsEcr := uint32(0)
		for i := 0; i < limit; {
			switch opts[i] {
			case header.TCPOptionEOL:
				i = limit
			case header.TCPOptionNOP:
				i++
			case header.TCPOptionMSS:
				v := uint16(opts[i + 2]) << 8 | uint16(opts[i + 3])
				if wantOpts.MSS != v {
					t.Fatalf("Bad MSS: got %v, want %v", v, wantOpts.MSS)
				}
				foundMSS = true
				i += 4
			case header.TCPOptionWS:
				if wantOpts.WS < 0 {
					t.Fatalf("WS present when it shouldn't be")
				}
				v := int(opts[i + 2])
				if v != wantOpts.WS {
					t.Fatalf("Bad WS: got %v, want %v", v, wantOpts.WS)
				}
				foundWS = true
				i += 3
			case header.TCPOptionTS:
				if i + 10 > limit || opts[i + 1] != 10 {
					t.Fatalf("Bad length %d for TS option, limit: %d", opts[i + 1], limit)
				}
				tsVal = binary.BigEndian.Uint32(opts[i + 2:])
				tsEcr = uint32(0)
				if tcp.Flags() & header.TCPFlagAck != 0 {
					// If the syn is an SYN-ACK then read
					// the tsEcr value as well
					tsEcr = binary.BigEndian.Uint32(opts[i + 6:])
				}
				foundTS = true
				i += 10
			default:
				i += int(opts[i + 1])
			}
		}

		if !foundMSS {
			t.Fatalf("MSS option not found. Options: %x", opts)
		}

		if !foundWS && wantOpts.WS >= 0 {
			t.Fatalf("WS option not found. Options: %x", opts)
		}

		if wantOpts.TS && !foundTS {
			t.Fatalf("TS option not found. Options: %x", opts)
		}

		if foundTS && tsVal == 0 {
			t.Fatalf("TS option specified but the timestamp value is zero")
		}

		if foundTS && tsEcr == 0 && wantOpts.TSEcr != 0 {
			t.Fatalf("TS option specified but TSEcr is incorrect: got %d, want: %d", tsEcr, wantOpts.TSEcr)
		}
	}
}
