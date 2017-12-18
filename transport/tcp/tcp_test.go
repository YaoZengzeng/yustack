package tcp_test

import (
	"bytes"
	"time"
	"testing"

	"github.com/YaoZengzeng/yustack/header"
	"github.com/YaoZengzeng/yustack/transport/tcp/testing/context"
	"github.com/YaoZengzeng/yustack/transport/tcp"
	"github.com/YaoZengzeng/yustack/network/ipv4"
	"github.com/YaoZengzeng/yustack/waiter"
	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/checker"
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

func TestNonBlockingClose(t *testing.T) {
	c := context.New(t, defaultMTU)
	defer c.Cleanup()

	c.CreateConnected(789, 30000, nil)
	ep := c.EP
	c.EP = nil

	// Close the endpoint and measure how long it takes
	t0 := time.Now()
	ep.Close()
	if diff := time.Now().Sub(t0); diff > 3 * time.Second {
		t.Fatalf("Took too long to close: %v", diff)
	}
}

func TestConnectResetAfterClose(t *testing.T) {
	c := context.New(t, defaultMTU)
	defer c.Cleanup()

	c.CreateConnected(789, 3000, nil)
	ep := c.EP
	c.EP = nil

	// Close the endpoint, make sure we get a FIN segment, then acknowledge
	// to complete closure of sender, but don't send our own FIN
	ep.Close()
	checker.IPv4(t, c.GetPacket(),
		checker.TCP(
			checker.DstPort(context.TestPort),
			checker.SeqNum(uint32(c.IRS) + 1),
			checker.AckNum(790),
			checker.TCPFlags(header.TCPFlagAck | header.TCPFlagFin),
		),
	)
	c.SendPacket(nil, &context.Headers{
		SrcPort:	context.TestPort,
		DstPort:	c.Port,
		Flags:		header.TCPFlagAck,
		SeqNum:		790,
		AckNum:		c.IRS.Add(1),
		RcvWnd:		30000,
	})

	// Wait for the ep to give up waiting for a FIN, and send a RST
	time.Sleep(3 * time.Second)
	for {
		b := c.GetPacket()
		tcp := header.TCP(header.IPv4(b).Payload())
		if tcp.Flags() == header.TCPFlagAck | header.TCPFlagFin {
			// This is a retransmit of the FIN, ignore it
			continue
		}

		checker.IPv4(t, b,
			checker.TCP(
				checker.DstPort(context.TestPort),
				checker.SeqNum(uint32(c.IRS) + 1),
				checker.AckNum(790),
				checker.TCPFlags(header.TCPFlagAck | header.TCPFlagRst),
			),
		)
		break
	}
}

func TestSimpleReceive(t *testing.T) {
	c := context.New(t, defaultMTU)
	defer c.Cleanup()

	c.CreateConnected(789, 3000, nil)

	we, ch := waiter.NewChannelEntry(nil)
	c.WQ.EventRegister(&we, waiter.EventIn)
	defer c.WQ.EventUnregister(&we)

	if _, err := c.EP.Read(nil); err != types.ErrWouldBlock {
		t.Fatalf("Unexpected error from Read: %v", err)
	}

	data := []byte{1, 2, 3}
	c.SendPacket(data, &context.Headers{
		SrcPort:	context.TestPort,
		DstPort:	c.Port,
		Flags:		header.TCPFlagAck,
		SeqNum:		790,
		AckNum:		c.IRS.Add(1),
		RcvWnd:		30000,
	})

	// Wait for receive to be notified
	select {
	case <-ch:
	case <-time.After(1 * time.Second):
		t.Fatalf("Timed out waiting for data to arrive")
	}

	// Receive data
	v, err := c.EP.Read(nil)
	if err != nil {
		t.Fatalf("Unexpected error from Read: %v", err)
	}

	if bytes.Compare(data, v) != 0 {
		t.Fatalf("Data is different: expected %v, got %v", data, v)
	}

	// Check that ACK is received
	checker.IPv4(t, c.GetPacket(),
		checker.TCP(
			checker.DstPort(context.TestPort),
			checker.SeqNum(uint32(c.IRS) + 1),
			checker.AckNum(uint32(790 + len(data))),
			checker.TCPFlags(header.TCPFlagAck),
		),
	)
}

func TestOutOfOrderReceive(t *testing.T) {
	c := context.New(t, defaultMTU)
	defer c.Cleanup()

	c.CreateConnected(789, 30000, nil)

	we, ch := waiter.NewChannelEntry(nil)
	c.WQ.EventRegister(&we, waiter.EventIn)
	defer c.WQ.EventUnregister(&we)

	if _, err := c.EP.Read(nil); err != types.ErrWouldBlock {
		t.Fatalf("Unexpected error from Read: %v", err)
	}

	// Send second half of data first, with seqnum 3 ahead of expected
	data := []byte{1, 2, 3, 4, 5, 6}
	c.SendPacket(data[3:], &context.Headers{
		SrcPort:	context.TestPort,
		DstPort:	c.Port,
		Flags:		header.TCPFlagAck,
		SeqNum:		793,
		AckNum:		c.IRS.Add(1),
		RcvWnd:		30000,
	})

	// Check that we get an ACK specifying which seqnum is expected
	checker.IPv4(t, c.GetPacket(),
		checker.TCP(
			checker.DstPort(context.TestPort),
			checker.SeqNum(uint32(c.IRS) + 1),
			checker.AckNum(790),
			checker.TCPFlags(header.TCPFlagAck),
		),
	)

	// Wait 200ms and check that no data has been received
	time.Sleep(200 * time.Millisecond)
	if _, err := c.EP.Read(nil); err != types.ErrWouldBlock {
		t.Fatalf("Unexpected error from Read: %v", err)
	}

	// Send the first 3 bytes now
	c.SendPacket(data[:3], &context.Headers{
		SrcPort:	context.TestPort,
		DstPort:	c.Port,
		Flags:		header.TCPFlagAck,
		SeqNum:		790,
		AckNum:		c.IRS.Add(1),
		RcvWnd:		30000,
	})

	// Receive data
	read := make([]byte, 0, 6)
	for len(read) < len(data) {
		v, err := c.EP.Read(nil)
		if err != nil {
			if err == types.ErrWouldBlock {
				// Wait for receice to be notified
				select {
				case <-ch:
				case <-time.After(5 * time.Second):
					t.Fatalf("Timed out waiting for data to arrive")
				}
				continue
			}
			t.Fatalf("Unexpected error from Read: %v", err)
		}

		read = append(read, v...)
	}

	// Check that we received the data in proper order
	if bytes.Compare(data, read) != 0 {
		t.Fatalf("Data is different: expected %v, got %v", data, read)
	}

	// Check that the whole data is acknowledged
	checker.IPv4(t, c.GetPacket(),
		checker.TCP(
			checker.DstPort(context.TestPort),
			checker.SeqNum(uint32(c.IRS) + 1),
			checker.AckNum(uint32(790 + len(data))),
			checker.TCPFlags(header.TCPFlagAck),
		),
	)
}
