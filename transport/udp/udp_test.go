package udp_test

import (
	"bytes"
	"time"
	"math/rand"
	"testing"

	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/buffer"
	"github.com/YaoZengzeng/yustack/link/channel"
	"github.com/YaoZengzeng/yustack/network/ipv4"
	"github.com/YaoZengzeng/yustack/transport/udp"
	"github.com/YaoZengzeng/yustack/stack"
	"github.com/YaoZengzeng/yustack/waiter"
	"github.com/YaoZengzeng/yustack/header"
)

const (
	stackAddr = "\x0a\x00\x00\x01"
	stackPort = 1234
	testAddr  = "\x0a\x00\x00\x02"
	testPort  = 4096

	// defaultMTU is the MTU, in bytes, used throughout the tests, except
	// where another value is explicitly used. It is chosen to match the MTU
	// of loopback interfaces on linux systems
	defaultMTU = 65536
)

type testContext struct {
	t 		*testing.T
	linkEp	*channel.Endpoint
	s 		*stack.Stack

	ep 		types.Endpoint
	wq		waiter.Queue
}

type headers struct {
	srcPort uint16
	dstPort uint16
}

func newDualTestContext(t *testing.T, mtu uint32) *testContext {
	s := stack.New([]string{ipv4.ProtocolName}, []string{udp.ProtocolName})

	id, linkEp := channel.New(256, mtu)

	if err := s.CreateNic(1, id); err != nil {
		t.Fatal("CreateNic failed: %v", err)
	}

	if err := s.AddAddress(1, ipv4.ProtocolNumber, stackAddr); err != nil {
		t.Fatal("AddAddress failed: %v", err)
	}

	s.SetRouteTable([]types.RouteEntry{
		{
			Destination:	types.Address("\x00\x00\x00\x00"),
			Mask:			types.Address("\x00\x00\x00\x00"),
			Gateway:		"",
			Nic:			1,
		},
	})

	return &testContext{
		t: 		t,
		s: 		s,
		linkEp:	linkEp,
	}
}

func TestV4ReadOnV4(t *testing.T) {
	c := newDualTestContext(t, defaultMTU)

	// Create v4 UDP endpoint
	var err error
	c.ep, err = c.s.NewEndpoint(udp.ProtocolNumber, ipv4.ProtocolNumber, &c.wq)
	if err != nil {
		c.t.Fatal("NewEndpoint failed: %v", err)
	}

	// Bind to wildcard
	err = c.ep.Bind(types.FullAddress{Port: stackPort})
	if err != nil {
		c.t.Fatal("Bind failed: %v", err)
	}

	// Test acceptance
	testV4Read(c)
}

func newPayload() []byte {
	b := make([]byte, 30 + rand.Intn(100))
	for i := range b {
		b[i] = byte(rand.Intn(256))
	}
	return b
}

func (c *testContext) sendPacket(payload []byte, h *headers) {
	// Allocate a buffer for data and headers
	buf := buffer.NewView(header.UDPMinimumSize + header.IPv4MinimumSize + len(payload))
	copy(buf[len(buf) - len(payload) : ], payload)

	// Initialize the IP header
	ip := header.IPv4(buf)
	ip.Encode(&header.IPv4Fields{
		IHL:			header.IPv4MinimumSize,
		TotalLength:	uint16(len(buf)),
		TTL:			64,
		Protocol:		uint8(udp.ProtocolNumber),
		SrcAddr:		testAddr,
		DstAddr:		stackAddr,
	})
	ip.SetChecksum(^ip.CalculateChecksum())

	// Initialize the UDP header
	u := header.UDP(buf[header.IPv4MinimumSize:])
	u.Encode(&header.UDPFields{
		SrcPort:	h.srcPort,
		DstPort:	h.dstPort,
		Length:		uint16(header.UDPMinimumSize + len(payload)),
	})

	// Calculate the UDP pseudo-header checksum
	xsum := header.Checksum([]byte(testAddr), 0)
	xsum = header.Checksum([]byte(stackAddr), xsum)
	xsum = header.Checksum([]byte{0, uint8(udp.ProtocolNumber)}, xsum)

	// Calculate the UDP checksum and set it
	length := uint16(header.UDPMinimumSize + len(payload))
	xsum = header.Checksum(payload, xsum)
	u.SetChecksum(^u.CalculateChecksum(xsum, length))

	// Inject packet
	var views [1]buffer.View
	vv := buf.ToVectorisedView(views)
	c.linkEp.Inject(ipv4.ProtocolNumber, &vv)
}

func testV4Read(c *testContext) {
	// Send a packet
	payload := newPayload()
	c.sendPacket(payload, &headers{
		srcPort: testPort,
		dstPort: stackPort,
	})

	// Try to receive the data
	we, ch := waiter.NewChannelEntry(nil)
	c.wq.EventRegister(&we, waiter.EventIn)
	defer c.wq.EventUnregister(&we)

	var addr types.FullAddress
	v, err := c.ep.Read(&addr)
	if err == types.ErrWouldBlock {
		// Wait for data to become available
		select {
		case <-ch:
			v, err = c.ep.Read(&addr)
			if err != nil {
				c.t.Fatal("Read failed: %v", err)
			}

		case <-time.After(1 * time.Second):
				c.t.Fatal("Time out waiting for data")
		}
	}

	// Check the peer address
	if addr.Address != testAddr {
		c.t.Fatal("Unexpected remote address: got %v, want %v", addr.Address, testAddr)
	}

	// Check the payload
	if !bytes.Equal(payload, v) {
		c.t.Fatal("Bad payload: got %x, want %x", v, payload)
	}
}