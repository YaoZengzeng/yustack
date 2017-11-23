package tcp

import (
	"sync"

	"github.com/YaoZengzeng/yustack/buffer"
	"github.com/YaoZengzeng/yustack/stack"
	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/waiter"
)

type endpointState int

const (
	stateInitial	endpointState = iota
	stateBound
	stateListen
	stateConnecting
	stateConnected
	stateClosed
	stateError
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

	// The following fields are protected by the mutex
	mu 				sync.RWMutex
	id 				types.TransportEndpointId
	state 			endpointState
	isPortReserved	bool

	// effectiveNetProtocols contains the network protocols actually in use. In most
	// cases it will only contain "netProtocol", but in cases like IPv6 endpoints
	// with v6only set to false, this could include multiple protocols (e.g., IPv6 and
	// IPv4) or a single different protocol (e.g., IPv4 when IPv6 endpoint is bound or
	// connected to an IPv4 mapped address)
	effectiveNetProtocols []types.NetworkProtocolNumber
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
	e.mu.Lock()
	defer e.mu.Unlock()

	// Don't allow binding once endpoint is not in the initial state
	// anymore. This is because once the endpoint goes into a connected or
	// listen state, it is already bound
	if e.state != stateInitial {
		return types.ErrAlreadyBound
	}

	netProtocols := []types.NetworkProtocolNumber{e.netProtocol}

	// Reserve the port
	port, err := e.stack.ReservePort(netProtocols, ProtocolNumber, address.Address, address.Port)
	if err != nil {
		return err
	}

	e.isPortReserved = true
	e.effectiveNetProtocols = netProtocols
	e.id.LocalPort = port

	// Mark endpoint as bound
	e.state = stateBound

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

