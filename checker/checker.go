package checker

import (
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