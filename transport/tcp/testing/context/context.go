package context

import (
	"testing"

	"github.com/YaoZengzeng/yustack/stack"
	"github.com/YaoZengzeng/yustack/network/ipv4"
	"github.com/YaoZengzeng/yustack/transport/tcp"
	"github.com/YaoZengzeng/yustack/link/channel"
	"github.com/YaoZengzeng/yustack/link/sniffer"
	"github.com/YaoZengzeng/yustack/types"
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
)

// Context provides an initialized Network stack and a link layer endpoint
// for use in TCP tests
type Context struct {
	t 		*testing.T
	linkEP	*channel.Endpoint
	s 		*stack.Stack

	// EP is the test endpoint in the stack owned by this context. This endpoint
	// is used in various tests to either initiate an active context or is used
	// as a passive listening endpoint to accept inbound connections
	EP 		types.Endpoint
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

// Cleanup closes the context endpoint if required
func (c *Context) Cleanup() {
	if c.EP != nil {
		c.EP.Close()
	}
}
