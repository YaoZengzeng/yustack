package tcp

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"hash"
	"sync"
	"io"
	"log"
	"time"

	"github.com/YaoZengzeng/yustack/stack"
	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/seqnum"
	"github.com/YaoZengzeng/yustack/sleep"
	"github.com/YaoZengzeng/yustack/header"
	"github.com/YaoZengzeng/yustack/waiter"
)

const (
	// tsLen is the length, in bits, of the timestamp in the SYN cookie
	tsLen = 8

	// tsMask is a mask for timestamp values (i.e., tsLen bits)
	tsMask = (1 << tsLen) - 1

	// tsOffset is the offset, in bits, of the timestamp in the SYN cookie
	tsOffset = 24

	// hashMask is the mask for hash values (i.e., tsOffset bits)
	hashMask = (1 << tsOffset) - 1

	// maxTSDiff is the maximum allowed difference between a received cookie
	// timestamp and the current timestamp. If the difference is greater
	// than maxTSDiff, the cookie is expired
	maxTSDiff = 2
)

var (
	// mssTable is a slice containing the possible MSS values that we
	// encode in the SYN cookie with two bits
	mssTable = []uint16{536, 1300, 1440, 1460}
)

func encodeMSS(mss uint16) uint32 {
	for i := len(mssTable) - 1; i > 0; i-- {
		if mss >= mssTable[i] {
			return uint32(i)
		}
	}
	return 0
}

// listenContext is used by a listening endpoint to store state and used while
// listening for connections. This struct is allocated by the listen goroutine
// and must not be accessed or have its methods called concurrently as they
// may mutate the stored objects
type listenContext struct {
	stack 	*stack.Stack
	rcvWnd	seqnum.Size
	nonce [2][sha1.BlockSize]byte

	hasherMu		sync.Mutex
	hasher 			hash.Hash
	netProtocol 	types.NetworkProtocolNumber
}

// timeStamp returns an 8-bit timestamp with a granularity of 64 seconds
func timeStamp() uint32 {
	return uint32(time.Now().Unix() >> 6) & tsMask
}

// newListenContext creates a new listen context
func newListenContext(stack *stack.Stack, rcvWnd seqnum.Size, netProtocol types.NetworkProtocolNumber) *listenContext {
	l := &listenContext{
		stack:			stack,
		rcvWnd:			rcvWnd,
		hasher:			sha1.New(),
		netProtocol:	netProtocol,	
	}

	rand.Read(l.nonce[0][:])
	rand.Read(l.nonce[1][:])

	return l
}

// cookieHash calculates the cookieHash for the given id, timestamp and nonce
// index. The hash is used to create and validate cookies
func (l *listenContext) cookieHash(id types.TransportEndpointId, ts uint32, nonceIndex int) uint32 {
	// Initialize block with fixed-size data: local ports and v
	var payload [8]byte
	binary.BigEndian.PutUint16(payload[0:], id.LocalPort)
	binary.BigEndian.PutUint16(payload[2:], id.RemotePort)

	// Feed everything to the hasher
	l.hasherMu.Lock()
	l.hasher.Reset()
	l.hasher.Write(payload[:])
	l.hasher.Write(l.nonce[nonceIndex][:])
	io.WriteString(l.hasher, string(id.LocalAddress))
	io.WriteString(l.hasher, string(id.RemoteAddress))

	// Finalize the calculation of the hash and return the first 4 bytes
	h := make([]byte, 0, sha1.Size)
	h = l.hasher.Sum(h)
	l.hasherMu.Unlock()

	return binary.BigEndian.Uint32(h[:])
}

// createCookie creates a SYN cookie for the given id and incoming sequence number
func (l *listenContext) createCookie(id types.TransportEndpointId, seq seqnum.Value, data uint32) seqnum.Value {
	// 8-bits timestamp
	ts := timeStamp()
	// tsOffset is 24
	v := l.cookieHash(id, 0, 0) + uint32(seq) + (ts << tsOffset)
	v += (l.cookieHash(id, ts, 1) + data) & hashMask

	return seqnum.Value(v)
}

// isCookieValid checks if the supplied cookie is valid for the given id and
// sequence number. If it is, it also returns the data originally encoded in
// the cookie when createCookie was created
func (l *listenContext) isCookieValid(id types.TransportEndpointId, cookie seqnum.Value, seq seqnum.Value) (uint32, bool) {
	ts := timeStamp()
	v := uint32(cookie) - l.cookieHash(id, 0, 0) - uint32(seq)
	cookieTS := v >> tsOffset
	if ((ts - cookieTS) & tsMask) > maxTSDiff {
		return 0, false
	}

	return (v - l.cookieHash(id, cookieTS, 1)) & hashMask, true
}

// createConnectedEndpoint creates a new connected endpoint, with the connection
// parameters given by the arguments
func (l *listenContext) createConnectedEndpoint(s *segment, iss seqnum.Value, irs seqnum.Value, rcvdSynOpts *header.TCPSynOptions) (*endpoint, error) {
	// Create a new endpoint
	netProtocol := l.netProtocol

	n := newEndpoint(l.stack, netProtocol, nil)
	n.id = s.id
	n.boundNicId = s.route.NicId()
	n.route = s.route.Clone()
	n.effectiveNetProtocols = []types.NetworkProtocolNumber{netProtocol}

	// Register new endpoint so that packets are routed to it
	if err := n.stack.RegisterTransportEndpoint(n.boundNicId, n.effectiveNetProtocols, ProtocolNumber, n.id, n); err != nil {
		log.Printf("createConnectedEndpoint: RegisterTransportEndpoint failed: %v\n", err)
		return nil, err
	}

	n.isRegistered = true
	n.state = stateConnected

	// Create sender and receiver
	//
	// The receiver at least temporarily has a zero receive window scale,
	// but the caller may change it (before starting the protocol loop)
	n.snd = newSender(n, iss, irs, s.window, rcvdSynOpts.MSS, rcvdSynOpts.WS)
	n.rcv = newReceiver(n, irs, l.rcvWnd, 0)

	return n, nil
}

// createEndpoint creates a new endpoint in connected state and then performs
// the TCP 3-way handshake
func (l *listenContext) createEndpointAndPerformHandshake(s *segment, opts *header.TCPSynOptions) (*endpoint, error) {
	// Create new endpoint
	irs := s.sequenceNumber
	cookie := l.createCookie(s.id, irs, encodeMSS(opts.MSS))
	ep, err := l.createConnectedEndpoint(s, cookie, irs, opts)
	if err != nil {
		return nil, err
	}

	// Perform the 3-way handshake
	h, err := newHandshake(ep, l.rcvWnd)
	if err != nil {
		log.Printf("createEndpointAndPerformHandshake: newHandshake failed: %v\n", err)
		return nil, err
	}
	log.Printf("createEndpointAndPerformHandshake: newHandshake succeeded\n")

	h.resetToSynRcvd(cookie, irs, opts)
	if err := h.execute(); err != nil {
		log.Printf("createEndpointAndPerformHandshake: handshake execute failed: %v\n", err)
		return nil, err
	}

	return ep, nil
}

// deliverAccepted delivers the newly-accepted endpoint to the listener. If the
// endpoint has transitioned out of the listen state, the new endpoint is closed
// instead
func (e *endpoint) deliverAccepted(n *endpoint) {
	e.mu.RLock()
	if e.state == stateListen {
		e.acceptedChan <- n
		e.waiterQueue.Notify(waiter.EventIn)
	} else {
		log.Printf("deliverAccepted: endpoint's state is not in stateListen, closed\n")
	}
	e.mu.RUnlock()
}

// handleSynSegment is called in its own goroutine once the listening endpoint
// receive a SYN segment. It is responsible for completing the handshake and
// queueing the new segment for acceptance
//
// A limited number of these goroutines are allowed before TCP starts using SYN
// cookies to accept connections
func (e *endpoint) handleSynSegment(ctx *listenContext, s *segment, opts *header.TCPSynOptions) {
	n, err := ctx.createEndpointAndPerformHandshake(s, opts)
	if err != nil {
		return
	}

	e.deliverAccepted(n)
}

// handleListenSegment is called when a listening endpoint receives a segment
// and needs to handle it
func (e *endpoint) handleListenSegment(ctx *listenContext, s *segment) {
	switch s.flags {
	case flagSyn:
		opts := parseSynSegmentOptions(s)
		go e.handleSynSegment(ctx, s, &opts)

	case flagAck:
		log.Printf("handleListenSegment: flagAck has not implemented yet\n")
	}
}

// protocolListenLoop is the main loop of a listening TCP endpoint. It runs in
// its own goroutine and is responsible for handling connection requests
func (e *endpoint) protocolListenLoop(rcvWnd seqnum.Size) error {
	ctx := newListenContext(e.stack, rcvWnd, e.netProtocol)

	var s sleep.Sleeper
	s.AddWaker(&e.notificationWaker, wakerForNotification)
	s.AddWaker(&e.newSegmentWaker, wakerForNewSegment)
	for {
		switch index, _ := s.Fetch(true); index {
		case wakerForNotification:
			log.Printf("protocolListenLoop: branch wakerForNotification has not implemented yet\n")

		case wakerForNewSegment:
			// Process at most maxSegmentsPerWake segments
			mayRequeue := true
			for i := 0; i < maxSegmentsPerWake; i++ {
				s := e.segmentQueue.dequeue()
				if s == nil {
					mayRequeue = false
					break
				}

				e.handleListenSegment(ctx, s)
			}

			// If the queue is not empty, make sure we'll wake up
			// in the next iteration
			if mayRequeue && !e.segmentQueue.empty() {
				e.newSegmentWaker.Assert()
			}
		}
	}
}
