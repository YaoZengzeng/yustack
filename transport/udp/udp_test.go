package udp_test

import (
	"testing"

	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/link/channel"
	"github.com/YaoZengzeng/yustack/network/ipv4"
	"github.com/YaoZengzeng/yustack/transport/udp"
	"github.com/YaoZengzeng/yustack/stack"
)

const (
	stackAddr = "\x0a\x00\x00\x01"

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
	_ = newDualTestContext(t, defaultMTU)

	// Create v4 UDP endpoint
}