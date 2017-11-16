package udp

import (
	"github.com/YaoZengzeng/yustack/buffer"
	"github.com/YaoZengzeng/yustack/header"
	"github.com/YaoZengzeng/yustack/stack"
	"github.com/YaoZengzeng/yustack/types"
)

const (
	// ProtocolName is the string representation of the udp protocol name
	ProtocolName = "udp"

	// ProtocolNumber is the udp protocol number
	ProtocolNumber = header.UDPProtocolNumber
)

type protocol struct{}

// Number returns the udp protocol number
func (*protocol) Number() types.TransportProtocolNumber {
	return ProtocolNumber
}

// MinimumPacketSize returns the minimum valid udp packet size
func (*protocol) MinimumPacketSize() int {
	return header.UDPMinimumSize
}

// ParsePorts returns the source and destination ports stored in the given udp
func (*protocol) ParsePorts(v buffer.View) (src, dst uint16, err error) {
	h := header.UDP(v)
	return h.SourcePort(), h.DestinationPort(), nil
}

func init() {
	stack.RegisterTransportProtocolFactory(ProtocolName, func() types.TransportProtocol {
		return &protocol{}
	})
}
