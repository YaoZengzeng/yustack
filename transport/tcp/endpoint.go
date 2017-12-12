package tcp

import (
	"sync"
	"log"
	"sync/atomic"

	"github.com/YaoZengzeng/yustack/seqnum"
	"github.com/YaoZengzeng/yustack/sleep"
	"github.com/YaoZengzeng/yustack/buffer"
	"github.com/YaoZengzeng/yustack/stack"
	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/waiter"
	"github.com/YaoZengzeng/yustack/tmutex"
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

// Reason for notifying the protocol goroutine
const (
	notifyNonZeroReceiveWindow = 1 << iota
	notifyReceiveWindowChanged
	notifyClose
)

// DefaultBufferSize is the default size of the receive and send buffers
const DefaultBufferSize = 208 * 1024

// endpoint represents a TCP endpoint. This struct serves as the interface
// between users of the endpoint and the protocol implementation; it is legal to
// have concurrent goroutines make calls into the endpoint, they are properly
// synchronized. The protocol implementation, however, runs in a single
// goroutine
type endpoint struct {
	// workMu is used to arbitrate which goroutine may perform protocol
	// work. Only the main protocol goroutine is expected to call Lock()
	// on it, but other goroutines (e.g., send) may call TryLock() to eagerly
	// perform work without having to wait for the main one to wake up
	workMu	tmutex.Mutex
	// The following fields are initialized at creation time and do not
	// change throughout the lifetime of the endpoint
	stack 		*stack.Stack
	netProtocol	types.NetworkProtocolNumber
	waiterQueue	*waiter.Queue

	// lastError represents the last error that the endpoint reported;
	// access to it is protected by the following mutex
	lastErrorMu sync.Mutex
	lastError 	error

	// The following fields are protected by the mutex
	mu 				sync.RWMutex
	id 				types.TransportEndpointId
	state 			endpointState
	isPortReserved	bool
	isRegistered	bool
	boundNicId		types.NicId
	route 			types.Route

	// effectiveNetProtocols contains the network protocols actually in use. In most
	// cases it will only contain "netProtocol", but in cases like IPv6 endpoints
	// with v6only set to false, this could include multiple protocols (e.g., IPv6 and
	// IPv4) or a single different protocol (e.g., IPv4 when IPv6 endpoint is bound or
	// connected to an IPv4 mapped address)
	effectiveNetProtocols []types.NetworkProtocolNumber

	// hardError is meaningfull only when state is stateError, it stores the
	// error to be returned when read/write syscalls are called and the
	// endpoint is in this state
	hardError error

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

	// workerCleanup specifies if the worker goroutine must perform cleanup
	// before exitting. This can only be set to true when workerRunning is
	// also true, and they're both protected by the mutex
	workerCleanup bool

	// segmentQueue is used to hand received segments to the protocol
	// goroutine. Segments are queued as long as the queue is not full,
	// and dropped when it is
	segmentQueue segmentQueue

	// The following fields are used to manage the send buffer. When segments
	// are ready to be sent, they are added to sndQueue and the protocol goroutine
	// is signaled via sndWaker
	//
	// When the send side is closed, the protocol goroutine is notified via sndCloseWaker
	// and sndBufSize is set to -1
	sndBufMu 		sync.Mutex
	sndBufSize		int
	sndBufUsed		int
	sndBufInQueue	seqnum.Size
	sndQueue 		segmentList
	sndWaker 		sleep.Waker
	sndCloseWaker 	sleep.Waker

	// newSegmentWaker is used to indicate to the protocol goroutine that
	// it needs to wake up and handle new segments queued to it.
	newSegmentWaker sleep.Waker

	// notificationWaker is used to indicate to the protocol goroutine that
	// it needs to wake up and check for notifications.
	notificationWaker sleep.Waker

	// notifyFlags is a bitmask of flags used to indicate to the protocol
	// goroutine what it was notified; this is only associated atomically
	notifyFlags uint32

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
		sndBufSize:		DefaultBufferSize,
	}
	e.segmentQueue.setLimit(2 * e.rcvBufSize)
	e.workMu.Init()
	e.workMu.Lock()

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

// startAcceptedLoop sets up required state and starts a goroutine with the
// main loop for accepted connections
func (e *endpoint) startAcceptedLoop(waiterQueue *waiter.Queue) {
	e.waiterQueue = waiterQueue
	e.workerRunning = true
	go e.protocolMainLoop(true)
}

// Accept returns a new endpoint if a peer has established a connection
// to an endpoint previously set to listen mode
func (e *endpoint) Accept() (types.Endpoint, *waiter.Queue, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Endpoint must be in listen state before it can accept connections
	if e.state != stateListen {
		return nil, nil, types.ErrInvalidEndpointState
	}

	// Get the new accepted endpoint
	var n *endpoint
	select {
	case n = <-e.acceptedChan:
	default:
		return nil, nil, types.ErrWouldBlock	
	}

	// Start the protocol goroutine
	wq := &waiter.Queue{}
	n.startAcceptedLoop(wq)

	return n, wq, nil
}

// Read reads data from the endpoint
func (e *endpoint) Read(*types.FullAddress) (buffer.View, error) {
	e.mu.RLock()

	// The endpoint can be read if it's connected, or if it's already closed
	// but has some pending unread data
	if s := e.state; s != stateConnected && s != stateClosed {
		e.mu.RUnlock()
		return buffer.View{}, types.ErrInvalidEndpointState
	}

	e.rcvListMu.Lock()
	v, err := e.readLocked()
	e.rcvListMu.Unlock()

	e.mu.RUnlock()

	return v, err
}

// readyToRead is called by the protocol goroutine when a new segment is ready
// to be read, or when the connection is closed for receiving (in which case
// s will be nil)
func (e *endpoint) readyToRead(s *segment) {
	e.rcvListMu.Lock()
	if s != nil {
		e.rcvBufUsed += s.data.Size()
		e.rcvList.PushBack(s)
	} else {
		e.rcvClosed = true
	}
	e.rcvListMu.Unlock()

	e.waiterQueue.Notify(waiter.EventIn)
}

func (e *endpoint) readLocked() (buffer.View, error) {
	if e.rcvBufUsed == 0 {
		if e.rcvClosed || e.state != stateConnected {
			return buffer.View{}, types.ErrClosedForReceive
		}
		return buffer.View{}, types.ErrWouldBlock
	}

	s := e.rcvList.Front()
	views := s.data.Views()
	v := views[s.viewToDeliver]
	s.viewToDeliver++

	if s.viewToDeliver >= len(views) {
		e.rcvList.Remove(s)
	}

	e.rcvBufUsed -= len(v)

	return v, nil
}

// Write writes data to the endpoint's peer
func (e *endpoint) Write(v buffer.View, to *types.FullAddress) (uintptr, error) {
	// Linux completely ignores any address passed to sendto(2) for TCP sockets
	// (without the MSG_FASTOPEN flag)

	e.mu.RLock()
	defer e.mu.RUnlock()

	// The endpoint cannot be written to if it's not connected
	if e.state != stateConnected {
		log.Printf("Write: state is not connected\n")
		return 0, types.ErrClosedForSend
	}

	// Nothing to do if the buffer is empty
	if len(v) == 0 {
		return 0, nil
	}

	e.sndBufMu.Lock()
	// Check if the connection has already been closed for sends
	if e.sndBufSize < 0 {
		e.sndBufMu.Unlock()
		log.Printf("Write: send buffer size < 0")
		return 0, types.ErrClosedForSend
	}

	// Check if we're already over the limit
	avail := e.sndBufSize - e.sndBufUsed
	if avail <= 0 {
		e.sndBufMu.Unlock()
		return 0, types.ErrWouldBlock
	}

	l := len(v)
	s := newSegmentFromView(&e.route, e.id, v)

	// Add data to the send queue
	e.sndBufUsed += l
	e.sndBufInQueue += seqnum.Size(l)
	e.sndQueue.PushBack(s)

	e.sndBufMu.Unlock()

	if e.workMu.TryLock() {
		// Do the work inline
		e.handleWrite()
		e.workMu.Unlock()
	} else {
		// Let the protocol goroutine do the work
		e.sndWaker.Assert()
	}

	return uintptr(l), nil
}

// Connect connects the endpoint to its peer
func (e *endpoint) Connect(addr types.FullAddress) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	netProtocol := e.netProtocol

	nicid := addr.Nic
	switch e.state {
	case stateBound:
		// If we're already bound to a Nic but the caller is requesting
		// that we use a different one now, we cannot proceed
		if e.boundNicId == 0 {
			break
		}

		if nicid != 0 && nicid != e.boundNicId {
			return types.ErrNoRoute
		}

		nicid = e.boundNicId

	case stateInitial:
		// Nothing to do. We'll eventually fill-in the gaps in the ID
		// (if any) when we find a route

	case stateConnecting:
		// A connection request has already been issued but hasn't
		// completed yet
		return types.ErrAlreadyConnecting

	case stateConnected:
		// The endpoint is already connected
		return types.ErrAlreadyConnected

	default:
		return types.ErrInvalidEndpointState
	}

	// Find a route to the desired destination
	r, err := e.stack.FindRoute(nicid, e.id.LocalAddress, addr.Address, netProtocol)
	if err != nil {
		return err
	}

	netProtocols := []types.NetworkProtocolNumber{netProtocol}
	e.id.LocalAddress = r.LocalAddress
	e.id.RemoteAddress = addr.Address
	e.id.RemotePort = addr.Port

	if e.id.LocalPort != 0 {
		// The endpoint is bound to a port, attempt to register it
		err := e.stack.RegisterTransportEndpoint(nicid, netProtocols, ProtocolNumber, e.id, e)
		if err != nil {
			return err
		}
	} else {
		// The endpoint doesn't have a local port yet, so try to get one
		_, err := e.stack.PickEphemeralPort(func (p uint16) (bool, error) {
			e.id.LocalPort = p
			err := e.stack.RegisterTransportEndpoint(nicid, netProtocols, ProtocolNumber, e.id, e)
			switch err {
			case nil:
				return true, nil
			case types.ErrPortInUse:
				return false, nil
			default:
				return false, err
			}
		})
		if err != nil {
			return err
		}
	}

	e.isRegistered = true
	e.state = stateConnecting
	e.route = r.Clone()
	e.boundNicId = nicid
	e.effectiveNetProtocols = netProtocols
	e.workerRunning = true

	go e.protocolMainLoop(false)

	return types.ErrConnectStarted
}

// cleanup frees all resources associated with the endpoint. It is called after
// Close() is called and the worker goroutine (if any) is done with its work
func (e *endpoint) cleanup() {
	// Close all endpoints that might have been accepted by TCP but not by
	// the client
	if e.acceptedChan != nil {
		close(e.acceptedChan)
		for n := range e.acceptedChan {
			n.resetConnection(types.ErrConnectionAborted)
			n.Close()
		}
	}

	if e.isRegistered {
		e.stack.UnregisterTransportEndpoint(e.boundNicId, e.effectiveNetProtocols, ProtocolNumber, e.id)
	}
}

func (e *endpoint) fetchNotifications() uint32 {
	return atomic.SwapUint32(&e.notifyFlags, 0)
}

func (e *endpoint) notifyProtocolGoroutine(n uint32) {
	for {
		v := atomic.LoadUint32(&e.notifyFlags)
		if v & n == n {
			// The flags are already set
			return
		}

		if atomic.CompareAndSwapUint32(&e.notifyFlags, v, v | n) {
			if v == 0 {
				// We are causing a transition from no flags to
				// at least one flag set, so we must cause the
				// protocol goroutine to wake up
				e.notificationWaker.Assert()
			}
			return
		}
	}
}

// Close puts the endpoint in a closed state and frees all resources associated
// with it. It must be called only once and with no other concurrent calls to
// the endpoint
func (e *endpoint) Close() {
	// Issue a shutdown so that the peer knows we won't send any more data
	// if we're connected, or stop accepting if we're listening
	e.Shutdown(types.ShutdownWrite | types.ShutdownRead)

	// While we hold the lock, determine if the cleanup should happen
	// inline or if we should tell the worker (if any) to do the cleanup
	e.mu.Lock()
	worker := e.workerRunning
	if worker {
		e.workerCleanup = true
	}

	e.mu.Unlock()

	// Now that we don't hold the lock anymore, either perform the local
	// cleanup or kick the worker to make sure it knows it needs to cleanup
	if !worker {
		e.cleanup()
	} else {
		e.notifyProtocolGoroutine(notifyClose)
	}
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

// Shutdown closes the read and/or write end of of the endpoint connection to its
// peers
func (e *endpoint) Shutdown(flags types.ShutdownFlags) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	switch e.state {
	case stateConnected:
		// Close for write
		if (flags & types.ShutdownWrite) != 0 {
			e.sndBufMu.Lock()

			if e.sndBufSize < 0 {
				// Already closed
				e.sndBufMu.Unlock()
				break
			}

			// Queue fin segment
			s := newSegmentFromView(&e.route, e.id, nil)
			e.sndQueue.PushBack(s)
			e.sndBufInQueue++

			// Mark endpoint as closed
			e.sndBufSize = -1

			e.sndBufMu.Unlock()

			// Tell protocol goroutine to close
			e.sndCloseWaker.Assert()
		}

	case stateListen:
		log.Printf("Shutdown: stateListen has not implemented yet\n")

	default:
		return types.ErrInvalidEndpointState
	}

	return nil
}

// GetSockOpt implements types.Endpoint.GetSockOpt
func (e *endpoint) GetSockOpt(opt interface{}) error {
	switch opt.(type) {
	case types.ErrorOption:
		e.lastErrorMu.Lock()
		err := e.lastError
		e.lastError = nil
		e.lastErrorMu.Unlock()
		return err
	}

	return types.ErrUnknownProtocolOption
}
