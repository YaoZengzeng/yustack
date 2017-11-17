package stack

import (
	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/buffer"
	"github.com/YaoZengzeng/yustack/waiter"
)

// TransportProtocol is the interface that needs to be implemented by transport
// protocol (e.g., tcp, udp) that want to be part of the networking stack
type TransportProtocol interface {
	// Number returns the transport protocol number
	Number() types.TransportProtocolNumber

	// NewEndpoint creates a new endpoint of the transport protocol
	NewEndpoint(stack *Stack, netProtocol types.NetworkProtocolNumber, waitQueue *waiter.Queue) (types.Endpoint, error)

	// MinimumPacketSize returns the minimum valid packet size of this
	// transport protocol. The stack automatically drops any packets smaller
	// than this targeted at this protocol
	MinimumPacketSize() int

	// ParsePorts returns the source and destination ports stored in a
	// packet of this protocol
	ParsePorts(v buffer.View) (src, dst uint16, err error)
}

// TransportProtocolFactory functions are used by the stack to instantiate
// transport protocol
type TransportProtocolFactory func() TransportProtocol

type TransportProtocolState struct {
	Protocol 		TransportProtocol
}
