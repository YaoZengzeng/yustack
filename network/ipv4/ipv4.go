// Package ipv4 contains the implementation of the ipv4 network protocol. To use
// it in the networking stack, this package must be added to the project, and
// activated on the stack by passing ipv4.ProtocolName (or "ipv4") as one of the
// network protocols when calling stack.New(). The endpoins can be created by passing
// ipv4.ProtocolNumber as the network protocol number when calling protocol.NewEndpoint().
package ipv4

import (
	"github.com/YaoZengzeng/yustack/header"
	"github.com/YaoZengzeng/yustack/stack"
	"github.com/YaoZengzeng/yustack/types"
)

const (
	// ProtocolName is the string representation of the ipv4 protocol name.
	ProtocolName = "ipv4"

	// ProtocolNumber is the ipv4 protocol number.
	ProtocolNumber = header.IPv4ProtocolNumber
)

type protocol struct{}

// NewProtocol creates a new ipv4 protocol descriptor. This is exported only for tests
// that short-circuit the stack. Regular use of the protocol is done via the stack, which
// gets a protocol descriptor from the init() function below.
func NewProtocol() types.NetworkProtocol {
	return &protocol{}
}

// Number returns the ipv4 protocol number
func (p *protocol) Number() types.NetworkProtocolNumber {
	return ProtocolNumber
}

func init() {
	stack.RegisterNetworkProtocolFactory(ProtocolName, func() types.NetworkProtocol {
		return &protocol{}
	})
}
