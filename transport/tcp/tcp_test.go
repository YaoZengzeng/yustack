package tcp_test

import (
	"testing"

	"github.com/YaoZengzeng/yustack/header"
	"github.com/YaoZengzeng/yustack/transport/tcp/testing/context"
	"github.com/YaoZengzeng/yustack/transport/tcp"
	"github.com/YaoZengzeng/yustack/network/ipv4"
	"github.com/YaoZengzeng/yustack/waiter"
	"github.com/YaoZengzeng/yustack/types"
)

const (
	// defaultMTU is the MTU, in bytes, used throughout the tests, except
	// where another value is explicitly used. It is chosen to match the MTU
	// of loopback interfaces on linux systems
	defaultMTU = 65535

	// defaultIPv4MSS is the MSS sent by the network stack in SYN/SYN-ACK for an
	// IPv4 endpoint when the MTU is set to defaultMTU in the test
	defaultIPv4MSS = defaultMTU - header.IPv4MinimumSize - header.TCPMinimumSize
)

func TestGiveUpContext(t *testing.T) {
	c := context.New(t, defaultMTU)
	defer c.Cleanup()

	var wq waiter.Queue
	ep, err := c.Stack().NewEndpoint(tcp.ProtocolNumber, ipv4.ProtocolNumber, &wq)
	if err != nil {
		t.Fatal("NewEndpoint failed: %v", err)
	}

	// Register for notification, then start connection attempt
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)
	wq.EventRegister(&waitEntry, waiter.EventOut)
	defer wq.EventUnregister(&waitEntry)

	err = ep.Connect(types.FullAddress{Address: context.TestAddr, Port: context.TestPort})
	if err != types.ErrConnectStarted {
		t.Fatal("Unexpected return value from Connect: %v", err)
	}

	// Close the connection, wait for completion
	ep.Close()

	// Wait for ep to become writable
	<-notifyCh
	err = ep.GetSockOpt(types.ErrorOption{})
}

func TestActiveHandshake(t *testing.T) {
	c := context.New(t, defaultMTU)
	defer c.Cleanup()

	c.CreateConnected(789, 30000, nil)
}
