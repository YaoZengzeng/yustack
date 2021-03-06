package context

import (
	"time"
	"testing"

	"github.com/YaoZengzeng/yustack/stack"
	"github.com/YaoZengzeng/yustack/network/ipv4"
	"github.com/YaoZengzeng/yustack/transport/tcp"
	"github.com/YaoZengzeng/yustack/link/channel"
	"github.com/YaoZengzeng/yustack/link/sniffer"
	"github.com/YaoZengzeng/yustack/waiter"
	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/seqnum"
	"github.com/YaoZengzeng/yustack/checker"
	"github.com/YaoZengzeng/yustack/header"
	"github.com/YaoZengzeng/yustack/buffer"
	"github.com/YaoZengzeng/yustack/checksum"
)

const (
	// StackAddr is the IPv4 address assigned to the stack
	StackAddr = "\x0a\x00\x00\x01"

	// StackPort is used as the listening port in tests for passive connects
	StackPort = 1234

	// TestAddr is the source address for packets sent to the stack via the
	// link layer endpoint
	TestAddr = "\x0a\x00\x00\x02"

	// TestPort is the TCP port used for packets sent to the stack via the link layer
	// endpoint
	TestPort = 4096

	// testInitialSequenceNumber is the initial sequence number sent in packets that
	// are sent in response to a SYN or in the initial SYN sent to the stack
	testInitialSequenceNumber = 789
)

// Headers is used to represent the TCP header fields when building a
// new packet
type Headers struct {
	// SrcPort holds the src port value to be used in the packet
	SrcPort uint16

	// DstPort holds the destination port value to be used in the packet
	DstPort uint16

	// SeqNum is the value of the sequence number field in the TCP header
	SeqNum seqnum.Value

	// AckNum represents the acknowledgement number fields in the TCP header
	AckNum seqnum.Value

	// Flags are the TCP flags in the TCP header
	Flags int

	// RcvWnd is the window to be advertised in the ReceiveWindow field of
	// the TCP header
	RcvWnd seqnum.Size

	// TCPOpts holds the options to be sent in the option field of the TCP
	// header
	TCPOpts []byte
}

// Context provides an initialized Network stack and a link layer endpoint
// for use in TCP tests
type Context struct {
	t 		*testing.T
	linkEP	*channel.Endpoint
	s 		*stack.Stack

	// IRS holds the initial sequence number in the SYN sent by endpoint in
	// case of an actice connect or the sequence number sent by the endpoint
	// in the SYN-ACK sent in response to a SYN when listening in passive
	// mode
	IRS seqnum.Value

	// Port holds the port bound by EP below in case of an active connect or
	// the listening port number in case of a passive connect
	Port uint16

	// EP is the test endpoint in the stack owned by this context. This endpoint
	// is used in various tests to either initiate an active context or is used
	// as a passive listening endpoint to accept inbound connections
	EP 		types.Endpoint

	// WQ is the wait queue associated with EP and is used to block for events
	WQ waiter.Queue
}

// New allocations and initializes a test context containing a new
// stack and a link-layer endpoint
func New(t *testing.T, mtu uint32) *Context {
	s := stack.New([]string{ipv4.ProtocolName}, []string{tcp.ProtocolName})

	id, linkEP := channel.New(256, mtu)
	if testing.Verbose() {
		id = sniffer.New(id)
	}

	if err := s.CreateNic(1, id); err != nil {
		t.Fatal("CreateNic failed: %v", err)
	}

	if err := s.AddAddress(1, ipv4.ProtocolNumber, StackAddr); err != nil {
		t.Fatal("AddAddress failed: %v", err)
	}

	s.SetRouteTable([]types.RouteEntry{
		{
			Destination:	"\x00\x00\x00\x00",
			Mask:			"\x00\x00\x00\x00",
			Gateway:		"",
			Nic:			1,
		},
	})

	return &Context{
		t:		t,
		s:		s,
		linkEP:	linkEP,
	}
}

// Stack returns a reference to the stack in the Context
func (c *Context) Stack() *stack.Stack {
	return c.s
}

// CreateConnected creates a connected TCP endpoint
func (c *Context) CreateConnected(iss seqnum.Value, rcvWnd seqnum.Size, epRcvBuf *types.ReceiveBufferSizeOption) {
	c.CreateConnectedWithRawOptions(iss, rcvWnd, epRcvBuf, nil)
}

// GetPacket reads a packet from the link layer endpoint and verifies
// that it is an IPv4 packet with the expected source and destination
// addresses. It will fail with an error if no packet is received for
// 2 seconds
func (c *Context) GetPacket() []byte {
	select {
	case p := <-c.linkEP.C:
		if p.Protocol != ipv4.ProtocolNumber {
			c.t.Fatalf("Bad network protocol: got %v, wanted %v", p.Protocol, ipv4.ProtocolNumber)
		}
		b := make([]byte, len(p.Header) + len(p.Payload))
		copy(b, p.Header)
		copy(b[len(p.Header):], p.Payload)

		checker.IPv4(c.t, b, checker.SrcAddr(StackAddr), checker.DstAddr(TestAddr))
		return b

	case <-time.After(2 * time.Second):
		c.t.Fatalf("Packet wasn't written out")
	}

	return nil
}

// SendPacket builds and sends a TCP segment(with the provided payload and TCP
// headers) in an IPv4 packet via the link layer endpoint
func (c *Context) SendPacket(payload []byte, h *Headers) {
	// Allocate a buffer for data and headers
	buf := buffer.NewView(header.TCPMinimumSize + header.IPv4MinimumSize + len(h.TCPOpts) + len(payload))
	copy(buf[len(buf) - len(payload):], payload)
	copy(buf[len(buf) - len(payload) - len(h.TCPOpts):], h.TCPOpts)

	// Initialize the IP header
	ip := header.IPv4(buf)
	ip.Encode(&header.IPv4Fields{
		IHL:			header.IPv4MinimumSize,
		TotalLength:	uint16(len(buf)),
		TTL:			64,
		Protocol:		uint8(tcp.ProtocolNumber),
		SrcAddr:		TestAddr,
		DstAddr:		StackAddr,
	})
	ip.SetChecksum(^ip.CalculateChecksum())


	// Initialize the TCP header.
	t := header.TCP(buf[header.IPv4MinimumSize:])
	t.Encode(&header.TCPFields{
		SrcPort:    h.SrcPort,
		DstPort:    h.DstPort,
		SeqNum:     uint32(h.SeqNum),
		AckNum:     uint32(h.AckNum),
		DataOffset: uint8(header.TCPMinimumSize + len(h.TCPOpts)),
		Flags:      uint8(h.Flags),
		WindowSize: uint16(h.RcvWnd),
	})

	// Calculate the TCP pseudo-header checksum.
	xsum := checksum.Checksum([]byte(TestAddr), 0)
	xsum = checksum.Checksum([]byte(StackAddr), xsum)
	xsum = checksum.Checksum([]byte{0, uint8(tcp.ProtocolNumber)}, xsum)

	// Calculate the TCP checksum and set it.
	length := uint16(header.TCPMinimumSize + len(h.TCPOpts) + len(payload))
	xsum = checksum.Checksum(payload, xsum)
	t.SetChecksum(^t.CalculateChecksum(xsum, length))

	// Inject packet
	var views [1]buffer.View
	vv := buf.ToVectorisedView(views)
	c.linkEP.Inject(ipv4.ProtocolNumber, &vv)
}

// CreateConnectedWithRawOptions creates a connected TCP endpoint and sends
// the specified option bytes as the Option field in initial SYN packet
//
// It also sets the receive buffer for the endpoint to the specified
// value in epRcvBuf
func (c *Context) CreateConnectedWithRawOptions(iss seqnum.Value, rcvWnd seqnum.Size, epRcvBuf *types.ReceiveBufferSizeOption, options []byte) {
	var err error
	c.EP, err = c.s.NewEndpoint(tcp.ProtocolNumber, ipv4.ProtocolNumber, &c.WQ)
	if err != nil {
		c.t.Fatalf("NewEndpoint failed: %v", err)
	}

	if epRcvBuf != nil {
		if err := c.EP.SetSockOpt(*epRcvBuf); err != nil {
			c.t.Fatalf("SetSockOpt failed: %v", err)
		}
	}

	// Start connection attempt
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)
	c.WQ.EventRegister(&waitEntry, waiter.EventOut)
	defer c.WQ.EventUnregister(&waitEntry)

	err = c.EP.Connect(types.FullAddress{Address: TestAddr, Port: TestPort})
	if err != types.ErrConnectStarted {
		c.t.Fatalf("Unexpected return value from Connect: %v", err)
	}

	// Receive SYN packet
	b := c.GetPacket()
	checker.IPv4(c.t, b,
		checker.TCP(
			checker.DstPort(TestPort),
			checker.TCPFlags(header.TCPFlagSyn),
		),
	)

	tcp := header.TCP(header.IPv4(b).Payload())
	c.IRS = seqnum.Value(tcp.SequenceNumber())

	c.SendPacket(nil, &Headers{
		SrcPort:	tcp.DestinationPort(),
		DstPort:	tcp.SourcePort(),
		Flags:		header.TCPFlagSyn | header.TCPFlagAck,
		SeqNum:		iss,
		AckNum:		c.IRS.Add(1),
		RcvWnd:		rcvWnd,
		TCPOpts:	options,
	})

	// Receive ACK packet
	checker.IPv4(c.t, c.GetPacket(),
		checker.TCP(
			checker.DstPort(TestPort),
			checker.TCPFlags(header.TCPFlagAck),
			checker.SeqNum(uint32(c.IRS) + 1),
			checker.AckNum(uint32(iss) + 1),
		),
	)

	// Wait for connection to be established
	select {
	case <-notifyCh:
		err = c.EP.GetSockOpt(types.ErrorOption{})
		if err != nil {
			c.t.Fatalf("Unexpected error when connecting: %v", err)
		}

	case <-time.After(1 * time.Second):
		c.t.Fatalf("Time out waiting for connection")
	}

	c.Port = tcp.SourcePort()
}

// CheckNoPacketTimeout verifies that no packet is received during the time
// specified by wait
func (c *Context) CheckNoPacketTimeout(errMsg string, wait time.Duration) {
	select {
	case <-c.linkEP.C:
			c.t.Fatalf(errMsg)

	case <-time.After(wait):
	}
}

// CheckNoPacket verifies that no packet is received for 1 second
func (c *Context) CheckNoPacket(errMsg string) {
	c.CheckNoPacketTimeout(errMsg, 1 * time.Second)
}

// Cleanup closes the context endpoint if required
func (c *Context) Cleanup() {
	if c.EP != nil {
		c.EP.Close()
	}
}

// PassiveConnectionWithOptions initiates a new connectin (with the specified TCP
// options enabled) to the port on which the Context.ep is listening for new connections.
// It also validates that the SYN-ACK has the expected values for the enabled options.
//
// NOTE: MSS is not a negotiated option and it can be asymmetric in each direction. This
// function uses the mayPayload to set the MSS to be sent to the peer on a connect and validates
// that the MSS in the SYN-ACK response is equal to the MTU - (tcphdr len + iphdr len)
//
// wndScale is the expected window scale in the SYN-ACK and synOptions.WS is the value of the window
// scaling option to be sent in the SYN. If synOptions.WS > 0 then we send the WindowScale option
func (c *Context) PassiveConnectWithOptions(maxPayload, wndScale int, synOptions header.TCPSynOptions) {
	opts := []byte{
		header.TCPOptionMSS, 4, byte(maxPayload / 256), byte(maxPayload % 256),
	}

	if synOptions.WS >= 0 {
		opts = append(opts, []byte{
			header.TCPOptionWS, 3, byte(synOptions.WS), header.TCPOptionNOP,
		}...)
	}

	// Send a SYN request
	iss := seqnum.Value(testInitialSequenceNumber)
	c.SendPacket(nil, &Headers{
		SrcPort:	TestPort,
		DstPort:	StackPort,
		Flags:		header.TCPFlagSyn,
		SeqNum:		iss,
		RcvWnd:		30000,
		TCPOpts:	opts,
	})

	// Receive the SYN-ACK reply. Make sure MSS is present
	b := c.GetPacket()
	tcp := header.TCP(header.IPv4(b).Payload())
	c.IRS = seqnum.Value(tcp.SequenceNumber())

	tcpCheckers := []checker.TransportChecker{
		checker.SrcPort(StackPort),
		checker.DstPort(TestPort),
		checker.TCPFlags(header.TCPFlagAck | header.TCPFlagSyn),
		checker.AckNum(uint32(iss) + 1),
		checker.TCPSynOptions(header.TCPSynOptions{MSS: synOptions.MSS, WS: wndScale}),
	}

	// If TS option was enabled in the original SYN then add a checker to
	// validate the Timestamp option in the SYN-ACk
/*	if synOptions.TS {
		tcpCheckers = append(tcpCheckers, checker.TCPTimestampChecker(synOptions.TS, 0, synOptions.TSVal))
	} else {
		tcpCheckers = append(tcpCheckers, checker.TCPTimestampChecker(false, 0, 0))
	}*/

	checker.IPv4(c.t, b, checker.TCP(tcpCheckers...))
	rcvWnd := seqnum.Size(30000)
	ackHeaders := &Headers{
		SrcPort:	TestPort,
		DstPort:	StackPort,
		Flags:		header.TCPFlagAck,
		SeqNum:		iss + 1,
		AckNum:		c.IRS + 1,
		RcvWnd:		rcvWnd,
	}

	// If WS was expected to be in effect then scale the advertised window
	// correspoindingly
	if synOptions.WS > 0 {
		ackHeaders.RcvWnd = rcvWnd >> byte(synOptions.WS)
	}

	// Send ACK
	c.SendPacket(nil, ackHeaders)

	c.Port = StackPort
}
