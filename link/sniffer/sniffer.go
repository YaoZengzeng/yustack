package sniffer

import (
	"fmt"
	"log"
	"sync/atomic"

	"github.com/YaoZengzeng/yustack/stack"
	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/buffer"
	"github.com/YaoZengzeng/yustack/header"
)

var LogPackets uint32 = 1

type endpoint struct {
	dispatcher	types.NetworkDispatcher
	lower		types.LinkEndpoint
}

// New creates a new sniffer link-layer endpoint. It wraps around
// endpoint and logs packets and they traverse the endpoint
func New(lower types.LinkEndpointID) types.LinkEndpointID {
	return stack.RegisterLinkEndpoint(&endpoint{
		lower:	stack.FindLinkEndpoint(lower),
	})
}

// DeliverNetworkPacket implements the types.NetworkDispatcher interface. It is
// called by the link-layer endpoint being wrapped when a packet arrives, and
// logs the packet before forwarding to the actual dispatcher
func (e *endpoint) DeliverNetworkPacket(linkEp types.LinkEndpoint, remoteLinkAddr types.LinkAddress, protocol types.NetworkProtocolNumber, vv *buffer.VectorisedView) {
	if atomic.LoadUint32(&LogPackets) == 1 {
		LogPacket("recv", protocol, vv.First(), nil)
	}
	e.dispatcher.DeliverNetworkPacket(e, remoteLinkAddr, protocol, vv)
}

// Attach implements the types.LinkEndpoint interface. It saves the dispatcher
// and registers with lower endpoint as its dispatcher so that "e" is called
// for inbound packets
func (e *endpoint) Attach(dispatcher types.NetworkDispatcher) {
	e.dispatcher = dispatcher
	e.lower.Attach(e)
}

func (e *endpoint) MTU() uint32 {
	return e.lower.MTU()
}

func (e *endpoint) MaxHeaderLength() uint16 {
	return e.lower.MaxHeaderLength()
}

func (e *endpoint) LinkAddress() types.LinkAddress {
	return e.lower.LinkAddress()
}

// WritePacket implements the types.LinkEndpoint interface. It is called by
// higher-level protocols to write packets; it just logs the packet and forwards
// the request to the lower endpoint
func (e *endpoint) WritePacket(r *types.Route, hdr *buffer.Prependable, payload buffer.View, protocol types.NetworkProtocolNumber) error {
	if atomic.LoadUint32(&LogPackets) == 1 {
		LogPacket("send", protocol, hdr.UsedBytes(), payload)
	}
	return e.lower.WritePacket(r, hdr, payload, protocol)
}

// LogPacket logs the given packet
func LogPacket(prefix string, protocol types.NetworkProtocolNumber, b, plb []byte) {
	// Figure out the network layer info
	var transProto uint8
	src := types.Address("unknown")
	dst := types.Address("unknown")
	id := 0
	size := uint16(0)
	switch protocol {
	case header.IPv4ProtocolNumber:
		ipv4 := header.IPv4(b)
		src = ipv4.SourceAddress()
		dst = ipv4.DestinationAddress()
		transProto = ipv4.Protocol()
		size = ipv4.TotalLength() - uint16(ipv4.HeaderLength())
		b = b[ipv4.HeaderLength():]
		id = int(ipv4.ID())

	default:
		log.Printf("%s unknown network protocol", prefix)
		return
	}

	// Figure out the transport layer info
	transName := "unknown"
	srcPort := uint16(0)
	dstPort := uint16(0)
	details := ""
	switch types.TransportProtocolNumber(transProto) {
	case header.TCPProtocolNumber:
		transName = "tcp"
		tcp := header.TCP(b)
		srcPort = tcp.SourcePort()
		dstPort = tcp.DestinationPort()
		size -= uint16(tcp.DataOffset())

		// Initialize the TCP flags
		flags := tcp.Flags()
		flagsStr := []byte("FSRPAU")
		for i := range flagsStr {
			if flags & (1 << uint(i)) == 0 {
				flagsStr[i] = ' '
			}
		}
		details = fmt.Sprintf("flags:0x%02x (%v) seqnum: %v ack: %v win: %v xsum:0x%x", flags, string(flagsStr), tcp.SequenceNumber(), tcp.AckNumber(), tcp.WindowSize(), tcp.Checksum())
		if flags & header.TCPFlagSyn != 0{
			details += fmt.Sprintf(" options: %+v", header.ParseSynOptions(tcp.Options(), flags & header.TCPFlagAck != 0))
		} else {
			details += fmt.Sprintf(" options: %+v", tcp.ParsedOptions())
		}

	default:
		log.Printf("%s %v -> %v unknown transport protocol: %d", prefix, src, dst, transProto)
	}

	log.Printf("%s %s %v:%v -> %v:%v len:%d id:%04x %s", prefix, transName, src, srcPort, dst, dstPort, size, id, details)
}
