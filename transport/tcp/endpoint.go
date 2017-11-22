package tcp

import (
	"github.com/YaoZengzeng/yustack/buffer"
	"github.com/YaoZengzeng/yustack/stack"
	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/waiter"
)

// endpoint represents a TCP endpoint. This struct serves as the interface
// between users of the endpoint and the protocol implementation; it is legal to
// have concurrent goroutines make calls into the endpoint, they are properly
// synchronized. The protocol implementation, however, runs in a single
// goroutine
type endpoint struct {
	// The following fields are initialized at creation time and do not
	// change throughout the lifetime of the endpoint
	stack 		*stack.Stack
	netProtocol	types.NetworkProtocolNumber
	waiterQueue	*waiter.Queue
}

func newEndpoint(stack *stack.Stack, netProtocol types.NetworkProtocolNumber, waiterQueue *waiter.Queue) *endpoint {
	e := &endpoint{
		stack:			stack,
		netProtocol:	netProtocol,
		waiterQueue:	waiterQueue,
	}

	return e
}

// Bind binds the endpoint to a specific local address and port
// Specifying a Nic is optional
func (e *endpoint) Bind(address types.FullAddress) error {
	return nil
}

// Read reads data from the endpoint
func (e *endpoint) Read(*types.FullAddress) (buffer.View, error) {
	return nil, nil
}

// Write writes data to the endpoint's peer
func (e *endpoint) Write(v buffer.View, to *types.FullAddress) (uintptr, error) {
	return 0, nil
}

