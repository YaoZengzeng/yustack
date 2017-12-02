package tcp

import (
	"sync"
	"log"

	"github.com/YaoZengzeng/yustack/seqnum"
	"github.com/YaoZengzeng/yustack/sleep"
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

// DefaultBufferSize is the default size of the receive and send buffers
const DefaultBufferSize = 208 * 1024

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
	isRegistered	bool
	boundNicId		types.NicId

	// effectiveNetProtocols contains the network protocols actually in use. In most
	// cases it will only contain "netProtocol", but in cases like IPv6 endpoints
	// with v6only set to false, this could include multiple protocols (e.g., IPv6 and
	// IPv4) or a single different protocol (e.g., IPv4 when IPv6 endpoint is bound or
	// connected to an IPv4 mapped address)
	effectiveNetProtocols []types.NetworkProtocolNumber

	// The following fields are used to manage the receive queue. The
	// protocol goroutine adds ready-for-delivery segments to rcvList,
	// which are returned by Read() calls to users.
	//
	// Once the peer has closed the its send side, rcvClosed is set to true
	// to indicate to users that no more data is coming.
	rcvListMu  sync.Mutex
	rcvList    segmentList
	rcvClosed  bool
	rcvBufSize int
	rcvBufUsed int

	// workerRunning specifies if a worker goroutine is running
	workerRunning bool

	// segmentQueue is used to hand received segments to the protocol
	// goroutine. Segments are queued as long as the queue is not full,
	// and dropped when it is
	segmentQueue segmentQueue

	// newSegmentWaker is used to indicate to the protocol goroutine that
	// it needs to wake up and handle new segments queued to it.
	newSegmentWaker sleep.Waker

	// notificationWaker is used to indicate to the protocol goroutine that
	// it needs to wake up and check for notifications.
	notificationWaker sleep.Waker

	// acceptedChan is used by a listening endpoint protocol goroutine to
	// send newly accepted connections to the endpoint so that they can be
	// read by Accept() calls
	acceptedChan chan *endpoint

	// The following are only used from the protocol goroutine, and
	// therefore don't need locks to protect them
	rcv *receiver
	snd *sender
}

func newEndpoint(stack *stack.Stack, netProtocol types.NetworkProtocolNumber, waiterQueue *waiter.Queue) *endpoint {
	e := &endpoint{
		stack:			stack,
		netProtocol:	netProtocol,
		waiterQueue:	waiterQueue,
		rcvBufSize:		DefaultBufferSize,
	}
	e.segmentQueue.setLimit(2 * e.rcvBufSize)

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

// receiveBufferAvailable calculates how many bytes are still available in the
// receive buffer
func (e *endpoint) receiveBufferAvailable() int {
	e.rcvListMu.Lock()
	size := e.rcvBufSize
	used := e.rcvBufUsed
	e.rcvListMu.Unlock()

	// We may use more bytes than the buffer size when the receive buffer
	// shrinks.
	if used >= size {
		return 0
	}

	return size - used
}

// Listen puts the endpoint in "listen" mode, which allows it to accept
// new connections
func (e *endpoint) Listen(backlog int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Endpoint must be bound before it can transition to listen mode
	if e.state != stateBound {
		return types.ErrInvalidEndpointState
	}

	// Register the endpoint.
	if err := e.stack.RegisterTransportEndpoint(e.boundNicId, e.effectiveNetProtocols, ProtocolNumber, e.id, e); err != nil {
		return err
	}

	e.isRegistered = true
	e.state = stateListen
	e.acceptedChan = make(chan *endpoint, backlog)
	e.workerRunning = true


	go e.protocolListenLoop(seqnum.Size(e.receiveBufferAvailable()))

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



// HandlePacket is called by the stack when new packets arrive to this transport
// endpoint.
func (e *endpoint) HandlePacket(r *types.Route, id types.TransportEndpointId, vv *buffer.VectorisedView) {
	s := newSegment(r, id, vv)
	if !s.parse() {
		log.Printf("HandlePacket: parse failed\n`")
		return
	}

	// Send packet to worker goroutine.
	if e.segmentQueue.enqueue(s) {
		e.newSegmentWaker.Assert()
	} else {
		// The queue is full, so we drop the segment.
		log.Printf("HandlePacket: the queue is full, dropped\n")
	}
}

