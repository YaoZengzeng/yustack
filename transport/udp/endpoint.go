package udp

import (
	"fmt"
	"sync"
	"log"
	"strings"

	"github.com/YaoZengzeng/yustack/stack"
	"github.com/YaoZengzeng/yustack/checksum"
	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/waiter"
	"github.com/YaoZengzeng/yustack/buffer"
	"github.com/YaoZengzeng/yustack/header"
)

type endpointState int

const (
	stateInitial	endpointState = iota
	stateBound
	stateConnected
	stateClosed
)

// endpoint represents a UDP endpoint. This struct serves as the interface
// between users of the endpoint and the protocol implementation; it is legal to
// have concurrent goroutines make calls into the endpoint, they are properly
// synchronized
type endpoint struct {
	// The following fields are initialized at creation time and do not
	// change throughout the lifetime of the endpoint
	stack 		*stack.Stack
	netProtocol	types.NetworkProtocolNumber
	waiterQueue	*waiter.Queue

	// The following fields are used to manage the receive, and are proteced
	// by rcvMu
	rcvMu			sync.Mutex
	rcvReady		bool
	rcvList			udpPacketList
	rcvBufSizeMax	int
	rcvBufSize		int
	rcvClosed		bool


	// The following fields are protected by the mu mutex
	mu 			sync.RWMutex
	id 			types.TransportEndpointId
	state 		endpointState
	bindAddr	types.Address
	bindNicId	types.NicId
}

func newEndpoint(stack *stack.Stack, netProtocol types.NetworkProtocolNumber, waiterQueue *waiter.Queue) *endpoint {
	return &endpoint{
		stack:			stack,
		netProtocol:	netProtocol,
		waiterQueue:	waiterQueue,
		rcvBufSizeMax:	32 * 1024,
	}
}

func (e *endpoint) registerWithStack(nicid types.NicId, netProtocols []types.NetworkProtocolNumber, id types.TransportEndpointId) (types.TransportEndpointId, error) {
	if id.LocalPort != 0 {
		// The endpoint already has a local port, just attempt to register it
		err := e.stack.RegisterTransportEndpoint(nicid, netProtocols, ProtocolNumber, id, e)
		return id, err
	}

	// We need to find a port for the endpoint
	_, err := e.stack.PickEphemeralPort(func(p uint16) (bool, error) {
		id.LocalPort = p
		err := e.stack.RegisterTransportEndpoint(nicid, netProtocols, ProtocolNumber, id, e)
		if err != nil {
			if strings.Compare(err.Error(), "port is in use") == 0 {
				return false, nil
			} else {
				return false, err
			}
		}

		return true, nil
	})

	return id, err
}

func (e *endpoint) bindLocked(address types.FullAddress) error {
	// Don't allow binding once endpoint is not in the initial state anymore
	if e.state != stateInitial {
		log.Printf("bindLocked: endpoint's state is not stateInitial\n")
		return types.ErrInvalidEndpointState
	}

	netProtocols := []types.NetworkProtocolNumber{e.netProtocol}

	// Not check if the address is valid for simplicity

	id := types.TransportEndpointId{
		LocalPort:		address.Port,
		LocalAddress:	address.Address,
	}
	id, err := e.registerWithStack(address.Nic, netProtocols, id)
	if err != nil {
		log.Printf("bindLocked: registerWithStack failed %v\n", err)
		return err
	}
	e.id = id

	// Mark endpoint as bound
	e.state = stateBound

	e.rcvMu.Lock()
	e.rcvReady = true
	e.rcvMu.Unlock()

	return nil
}

// Bind binds the endpoint to a specific local address and port
// Specifying a Nic is optional
func (e *endpoint) Bind(address types.FullAddress) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	err := e.bindLocked(address)
	if err != nil {
		log.Printf("Bind failed: %v\n", err)
		return err
	}

	return nil
}

// Write writes data to the endpoint's peer. This method does not block if the data cannot
// be written
func (e *endpoint) Write(v buffer.View, to *types.FullAddress) (uintptr, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if to == nil {
		return 0, fmt.Errorf("udp.Write: to should not be nil")
	}

	nicid := to.Nic
	netProto := e.netProtocol


	// Find the route
	route, err := e.stack.FindRoute(nicid, e.bindAddr, to.Address, netProto)
	if err != nil {
		log.Printf("udp.Write: FindRoute failed\n")
		return 0, nil
	}
	dstPort := to.Port
	sendUDP(route, v, e.id.LocalPort, dstPort)

	return uintptr(len(v)), nil
}

// sendUDP sends an UDP segment via the provided network endpoint and under the
// provided identity
func sendUDP(r *types.Route, data buffer.View, localPort, remotePort uint16) error {
	// Allocate a buffer for the UDP header
	hdr := buffer.NewPrependable(header.UDPMinimumSize + int(r.MaxHeaderLength()))

	// Initialize the header
	udp := header.UDP(hdr.Prepend(header.UDPMinimumSize))

	length := uint16(hdr.UsedLength())
	xsum := r.PseudoHeaderChecksum(ProtocolNumber)
	if data != nil {
		length += uint16(len(data))
		xsum = checksum.Checksum(data, xsum)
	}

	udp.Encode(&header.UDPFields{
		SrcPort:	localPort,
		DstPort:	remotePort,
		Length:		length,
	})

	udp.SetChecksum(^udp.CalculateChecksum(xsum, length))

	return r.WritePacket(&hdr, data, ProtocolNumber)
}

// HandlePacket is called by the stack when new packets arrives to this transport
// endpoint
func (e *endpoint) HandlePacket(r *types.Route, id types.TransportEndpointId, vv *buffer.VectorisedView) {
	// Get the header then trim it from the view
	hdr := header.UDP(vv.First())
	if int(hdr.Length()) > vv.Size() {
		// Malformed packet
		return
	}

	vv.TrimFront(header.UDPMinimumSize)

	e.rcvMu.Lock()

	// Drop the packet if our buffer is currently full
	if !e.rcvReady || e.rcvClosed || e.rcvBufSize >= e.rcvBufSizeMax {
		e.rcvMu.Unlock()
		return
	}

	wasEmpty := e.rcvBufSize == 0

	// Push new packet into receive list and increment the buffer size
	pkt := &udpPacket{
		senderAddress:	types.FullAddress{
			Nic:		r.NicId(),
			Address:	id.RemoteAddress,
			Port:		hdr.SourcePort(),
		},
	}
	pkt.data = vv.Clone(pkt.views[:])
	e.rcvList.PushBack(pkt)
	e.rcvBufSize += vv.Size()

	e.rcvMu.Unlock()

	// Notify any waiters that there's data to be read now
	if wasEmpty {
		e.waiterQueue.Notify(waiter.EventIn)
	}
}

// Read reads data from the endpoint. This method does not block if
// there is no data pending
func (e *endpoint) Read(address *types.FullAddress) (buffer.View, error) {
	e.rcvMu.Lock()

	if e.rcvList.Empty() {
		err := types.ErrWouldBlock
		if e.rcvClosed {
			err = types.ErrClosedForReceive
		}
		e.rcvMu.Unlock()
		return buffer.View{}, err
	}

	p := e.rcvList.Front()
	e.rcvList.Remove(p)
	e.rcvBufSize -= p.data.Size()

	e.rcvMu.Unlock()

	if address != nil {
		*address = p.senderAddress
	}

	return p.data.ToView(), nil
}

// Listen is not supported by UDP, it just fails
func (*endpoint) Listen(int) error {
	return types.ErrNotSupported
}

// Accept is not supported by UDP, it just fails
func (*endpoint) Accept() (types.Endpoint, *waiter.Queue, error) {
	return nil, nil, types.ErrNotSupported
}

// Connect connects the endpoint to its peer. Specifying a Nic is optional
func (*endpoint) Connect(addr types.FullAddress) error {
	return types.ErrNotSupported
}
