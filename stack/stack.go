// Package stack provides the glue between networking protocols and the
// consumers of the networking stack.

package stack

import (
	"github.com/YaoZengzeng/yustack/types"
)

// Stack is a networking stack, with all supported protocols, NICs, and route table.
type Stack struct {
	networkProtocols map[types.NetworkProtocolNumber]types.NetworkProtocol
}

// New allocates a new networking stack with only the requested networking and
// transport protocols configured with default options.
func New(network []string, transport []string) *Stack {
	s := &Stack{
		networkProtocols: make(map[types.NetworkProtocolNumber]types.NetworkProtocol),
	}

	// Add specified network protocols.
	for _, name := range network {
		netProtocolFactory, ok := networkProtocols[name]
		if !ok {
			continue
		}
		netProtocol := netProtocolFactory()
		s.networkProtocols[netProtocol.Number()] = netProtocol
	}

	return s
}
