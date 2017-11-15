package ipv4_test

import (
	"time"
	"testing"

	"github.com/YaoZengzeng/yustack/network/ipv4"
	"github.com/YaoZengzeng/yustack/stack"
	"github.com/YaoZengzeng/yustack/link/channel"
	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/buffer"
)

const stackAddr = "\x0a\x00\x00\x01"

type testContext struct {
	t 		*testing.T
	linkEp	*channel.Endpoint
	s 		*stack.Stack
}

func newTestContext(t *testing.T) *testContext {
	s := stack.New([]string{ipv4.ProtocolName}, []string{ipv4.PingProtocolName})

	const defaultMTU = 65536
	id, linkEp := channel.New(256, defaultMTU)

	if err := s.CreateNic(1, id); err != nil {
		t.Fatalf("CreateNic failed: %v", err)
	}

	if err := s.AddAddress(1, ipv4.ProtocolNumber, stackAddr); err != nil {
		t.Fatalf("AddAddress failed: %v", err)
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
		t:		t,
		s:		s,
		linkEp:	linkEp,
	}
}

func (c *testContext) cleanup() {
	close(c.linkEp.C)
}

func (c *testContext) loopback() {
	go func() {
		for pkt := range c.linkEp.C {
			v := make(buffer.View, len(pkt.Header) + len(pkt.Payload))
			copy(v, pkt.Header)
			copy(v[len(pkt.Header):], pkt.Payload)
			vv := v.ToVectorisedView([1]buffer.View{})
			c.linkEp.Inject(pkt.Protocol, &vv)
		}
	}()
}

func TestEcho(t *testing.T) {
	c := newTestContext(t)
	defer c.cleanup()
	c.loopback()

	ch := make(chan ipv4.PingReply, 1)
	p := ipv4.Pinger{
		Stack:		c.s,
		NicId:		1,
		Address:	stackAddr,
		Wait:		10 * time.Millisecond,
		Count:		1,	// Only ping once
	}
	if err := p.Ping(ch); err != nil {
		t.Fatalf("icmp.Ping failed: %v", err)
	}

	ping := <-ch
	if ping.Error != nil {
		t.Errorf("bad ping response: %v", ping.Error)
	}
}

func TestEchoSequence(t *testing.T) {
	c := newTestContext(t)
	defer c.cleanup()
	c.loopback()

	const numPings = 3
	ch := make(chan ipv4.PingReply, numPings)
	p := ipv4.Pinger{
		Stack:		c.s,
		NicId:		1,
		Address:	stackAddr,
		Wait:		10 * time.Millisecond,
		Count:		numPings,
	}

	if err := p.Ping(ch); err != nil {
		t.Fatalf("icmp.Ping failed: %v", err)
	}

	for i := uint16(0); i < numPings; i++ {
		ping := <-ch
		if ping.Error != nil {
			t.Errorf("i = %d, bad ping response: %v", i, ping.Error)
		}
		if ping.SeqNumber != i {
			t.Errorf("SeqNumber = %d, want %d", ping.SeqNumber, i)
		}
	}
}
