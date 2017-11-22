package tcp

import (
	"github.com/YaoZengzeng/yustack/buffer"
	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/header"
	"github.com/YaoZengzeng/yustack/stack"
	"github.com/YaoZengzeng/yustack/waiter"
)

const (
	// ProtocolName is the string representation of the tcp protocol name
	ProtocolName = "tcp"

	// ProtocolNumber is the tcp protocol number
	ProtocolNumber = header.TCPProtocolNumber
)

type protocol struct{}

// NewEndpoint creates a new tcp endpoint
func (*protocol) NewEndpoint(stack *stack.Stack, netProtocol types.NetworkProtocolNumber, waiterQueue *waiter.Queue) (types.Endpoint, error) {
	return newEndpoint(stack, netProtocol, waiterQueue), nil
}

// Number returns the tcp protocol number
func (*protocol) Number() types.TransportProtocolNumber {
	return ProtocolNumber
}

// MinimumPacketSize return the minimum valid tcp packet size
func (*protocol) MinimumPacketSize() int {
	return header.TCPMinimumSize
}

// ParsePorts returns the source and destination ports stored in the given
// tcp packet
func (*protocol) ParsePorts(v buffer.View) (src, dst uint16, err error) {
	h := header.TCP(v)
	return h.SourcePort(), h.DestinationPort(), nil
}

func init() {
	stack.RegisterTransportProtocolFactory(ProtocolName, func() stack.TransportProtocol {
		return &protocol{}
	})
}
