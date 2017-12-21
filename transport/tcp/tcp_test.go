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
	"github.com/YaoZengzeng/yustack/buffer"
	"github.com/YaoZengzeng/yustack/seqnum"
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

func TestOutOfOrderFlood(t *testing.T) {
	c := context.New(t, defaultMTU)
	defer c.Cleanup()

	// Create a new connection with initial window size of 10
	opt := types.ReceiveBufferSizeOption(10)
	c.CreateConnected(789, 30000, &opt)

	if _, err := c.EP.Read(nil); err != types.ErrWouldBlock {
		t.Fatalf("Unexpected error from Read: %v", err)
	}

	// Send 100 packets before the actual one that is expected
	data := []byte{1, 2, 3, 4, 5, 6}
	for i := 0; i < 100; i++ {
		c.SendPacket(data[3:], &context.Headers{
				SrcPort:	context.TestPort,
				DstPort:	c.Port,
				Flags:		header.TCPFlagAck,
				SeqNum:		796,
				AckNum:		c.IRS.Add(1),
				RcvWnd:		30000,
		})

		checker.IPv4(t, c.GetPacket(),
			checker.TCP(
				checker.DstPort(context.TestPort),
				checker.SeqNum(uint32(c.IRS) + 1),
				checker.AckNum(790),
				checker.TCPFlags(header.TCPFlagAck),
			),
		)
	}

	// Send packet with seqnum 793. It must be discarded because the
	// out-of-order buffer was filled by the previous packets
	c.SendPacket(data[3:], &context.Headers{
		SrcPort:	context.TestPort,
		DstPort:	c.Port,
		Flags:		header.TCPFlagAck,
		SeqNum:		793,
		AckNum:		c.IRS.Add(1),
		RcvWnd:		30000,
	})

	checker.IPv4(t, c.GetPacket(),
		checker.TCP(
			checker.DstPort(context.TestPort),
			checker.SeqNum(uint32(c.IRS) + 1),
			checker.AckNum(790),
			checker.TCPFlags(header.TCPFlagAck),
		),
	)

	// Now send the expected packet, seqnum 790
	c.SendPacket(data[:3], &context.Headers{
		SrcPort:	context.TestPort,
		DstPort:	c.Port,
		Flags:		header.TCPFlagAck,
		SeqNum:		790,
		AckNum:		c.IRS.Add(1),
		RcvWnd:		30000,
	})

	// Check that only packet 790 is acknowledged
	checker.IPv4(t, c.GetPacket(),
		checker.TCP(
			checker.DstPort(context.TestPort),
			checker.SeqNum(uint32(c.IRS) + 1),
			checker.AckNum(793),
			checker.TCPFlags(header.TCPFlagAck),
		),
	)
}

func TestFullWindowReceive(t *testing.T) {
	c := context.New(t, defaultMTU)
	defer c.Cleanup()

	opt := types.ReceiveBufferSizeOption(10)
	c.CreateConnected(789, 30000, &opt)

	we, ch := waiter.NewChannelEntry(nil)
	c.WQ.EventRegister(&we, waiter.EventIn)
	defer c.WQ.EventUnregister(&we)

	_, err := c.EP.Read(nil)
	if err != types.ErrWouldBlock {
		t.Fatalf("Unexpected error from Read: %v", err)
	}

	// Fill up the window
	data := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
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
	case <-time.After(5 * time.Second):
		t.Fatalf("Timed out waiting for data to arrive")
	}

	// Check that data is acknowledged, and window goes to zero
	checker.IPv4(t, c.GetPacket(),
		checker.TCP(
			checker.DstPort(context.TestPort),
			checker.SeqNum(uint32(c.IRS) + 1),
			checker.AckNum(uint32(790 + len(data))),
			checker.TCPFlags(header.TCPFlagAck),
			checker.Window(0),
		),
	)

	// Receive data and check it
	v, err := c.EP.Read(nil)
	if err != nil {
		t.Fatalf("Unexpected error from Read: %v", err)
	}

	if bytes.Compare(data, v) != 0 {
		t.Fatalf("Data is different: expected %v, got %v", data, v)
	}

	// Check that we get an ACK for the newly non-zero window
	checker.IPv4(t, c.GetPacket(),
		checker.TCP(
			checker.DstPort(context.TestPort),
			checker.SeqNum(uint32(c.IRS) + 1),
			checker.AckNum(uint32(790 + len(data))),
			checker.TCPFlags(header.TCPFlagAck),
			checker.Window(10),
		),
	)
}

func TestNoWindowShrinking(t *testing.T) {
	c := context.New(t, defaultMTU)
	defer c.Cleanup()

	// Start off with a window size of 10, then shrink it to 5
	opt := types.ReceiveBufferSizeOption(10)
	c.CreateConnected(789, 30000, &opt)

	opt = 5
	if err := c.EP.SetSockOpt(opt); err != nil {
		t.Fatalf("SetSockOpt failed: %v", err)
	}

	we, ch := waiter.NewChannelEntry(nil)
	c.WQ.EventRegister(&we, waiter.EventIn)
	defer c.WQ.EventUnregister(&we)

	_, err := c.EP.Read(nil)
	if err != types.ErrWouldBlock {
		t.Fatalf("Unexpected error from Read: %v", err)
	}

	// Send 3 bytes, check that the peer acknowledges them
	data := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	c.SendPacket(data[:3], &context.Headers{
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
	case <-time.After(5 * time.Second):
		t.Fatalf("Timed out waiting for data to arrive")
	}

	// Check that data is acknowledged, and that window doesn't go to zero
	// just yet because it was previously set to 10. It must go to 7 now
	checker.IPv4(t, c.GetPacket(),
		checker.TCP(
			checker.DstPort(context.TestPort),
			checker.SeqNum(uint32(c.IRS) + 1),
			checker.AckNum(793),
			checker.TCPFlags(header.TCPFlagAck),
			checker.Window(7),
		),
	)

	// Send 7 more bytes, check that the window fills up
	c.SendPacket(data[3:], &context.Headers{
		SrcPort:	context.TestPort,
		DstPort:	c.Port,
		Flags:		header.TCPFlagAck,
		SeqNum:		793,
		AckNum:		c.IRS.Add(1),
		RcvWnd:		30000,
	})

	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("Timed out waiting for data to arrive")
	}

	checker.IPv4(t, c.GetPacket(),
		checker.TCP(
			checker.DstPort(context.TestPort),
			checker.SeqNum(uint32(c.IRS) + 1),
			checker.AckNum(uint32(790 + len(data))),
			checker.TCPFlags(header.TCPFlagAck),
			checker.Window(0),
		),
	)

	// Receive data and check it
	read := make([]byte, 0, 10)
	for len(read) < len(data) {
		v, err := c.EP.Read(nil)
		if err != nil {
			t.Fatalf("Unexpected error from Read: %v", err)
		}

		read = append(read, v...)
	}

	if bytes.Compare(data, read) != 0 {
		t.Fatalf("Data is different: expected %v, got %v", data, read)
	}

	// Check that we get an ACK for the newly non-zero window, which is the
	// new size
	checker.IPv4(t, c.GetPacket(),
		checker.TCP(
			checker.DstPort(context.TestPort),
			checker.SeqNum(uint32(c.IRS) + 1),
			checker.AckNum(uint32(790 + len(data))),
			checker.TCPFlags(header.TCPFlagAck),
			checker.Window(5),
		),
	)
}

func TestSimpleSend(t *testing.T) {
	c := context.New(t, defaultMTU)
	defer c.Cleanup()

	c.CreateConnected(789, 30000, nil)

	data := []byte{1, 2, 3}
	view := buffer.NewView(len(data))
	copy(view, data)

	if _, err := c.EP.Write(view, nil); err != nil {
		t.Fatalf("Unexpected error from Write: %v", err)
	}

	// Check that data is received
	b := c.GetPacket()
	checker.IPv4(t, b,
		checker.PayloadLen(len(data) + header.TCPMinimumSize),
		checker.TCP(
			checker.DstPort(context.TestPort),
			checker.SeqNum(uint32(c.IRS) + 1),
			checker.AckNum(790),
			checker.TCPFlagsMatch(header.TCPFlagAck, ^uint8(header.TCPFlagPsh)),
		),
	)

	if p := b[header.IPv4MinimumSize + header.TCPMinimumSize:]; bytes.Compare(data, p) != 0 {
		t.Fatalf("Data is different: expected %v, got %v", data, p)
	}

	// Acknowledge the data
	c.SendPacket(nil, &context.Headers{
		SrcPort:	context.TestPort,
		DstPort:	c.Port,
		Flags:		header.TCPFlagAck,
		SeqNum:		790,
		AckNum:		c.IRS.Add(1 + seqnum.Size(len(data))),
		RcvWnd:		30000,
	})
}

func TestZeroWindowSend(t *testing.T) {
	c := context.New(t, defaultMTU)
	defer c.Cleanup()

	c.CreateConnected(789, 0, nil)

	data := []byte{1, 2, 3}
	view := buffer.NewView(len(data))
	copy(view, data)

	_, err := c.EP.Write(view, nil)
	if err != nil {
		t.Fatalf("Unexpected error from Write: %v", err)
	}

	// Since the window is currently zero, check that no packet is received
	c.CheckNoPacket("Packet received when window is zero")

	// Open up the window. Data should be received now
	c.SendPacket(nil, &context.Headers{
		SrcPort:	context.TestPort,
		DstPort:	c.Port,
		Flags:		header.TCPFlagAck,
		SeqNum:		790,
		AckNum:		c.IRS.Add(1),
		RcvWnd:		30000,
	})

	// Check that data is received
	b := c.GetPacket()
	checker.IPv4(t, b,
		checker.PayloadLen(len(data) + header.TCPMinimumSize),
		checker.TCP(
			checker.DstPort(context.TestPort),
			checker.SeqNum(uint32(c.IRS) + 1),
			checker.AckNum(790),
			checker.TCPFlagsMatch(header.TCPFlagAck, ^uint8(header.TCPFlagPsh)),
		),
	)

	if p := b[header.IPv4MinimumSize + header.TCPMinimumSize:]; bytes.Compare(data, p) != 0 {
		t.Fatalf("Data is different: expected %v, got %v", data, p)
	}

	// Acknowledge the data
	c.SendPacket(nil, &context.Headers{
		SrcPort:	context.TestPort,
		DstPort:	c.Port,
		Flags:		header.TCPFlagAck,
		SeqNum:		790,
		AckNum:		c.IRS.Add(1 + seqnum.Size(len(data))),
	})
}

func TestScaledWindowConnect(t *testing.T) {
	// This test ensures that window scaling is used when the peer
	// does advertise it and connection is established with Connect()
	c := context.New(t, defaultMTU)
	defer c.Cleanup()

	// Set the window size greater than the maximum non-scaled window
	opt := types.ReceiveBufferSizeOption(65535 * 3)
	c.CreateConnectedWithRawOptions(789, 30000, &opt, []byte{
		header.TCPOptionWS, 3, 0, header.TCPOptionNOP,
	})

	data := []byte{1, 2, 3}
	view := buffer.NewView(len(data))
	copy(view, data)

	if _, err := c.EP.Write(view, nil); err != nil {
		t.Fatalf("Unexpected error from Write: %v", err)
	}

	// Check that data is received, and that advertised window is 0xbfff,
	// that it is scaled
	b := c.GetPacket()
	checker.IPv4(t, b,
		checker.PayloadLen(len(data) + header.TCPMinimumSize),
		checker.TCP(
			checker.DstPort(context.TestPort),
			checker.SeqNum(uint32(c.IRS) + 1),
			checker.AckNum(790),
			checker.Window(0xbfff),
			checker.TCPFlagsMatch(header.TCPFlagAck, ^uint8(header.TCPFlagPsh)),
		),
	)
}

func TestNonScaledWindowConnect(t *testing.T) {
	// This test ensures that window scaling is not used when the peer
	// doesn't advertise it and connection is established with Connect()
	c := context.New(t, defaultMTU)
	defer c.Cleanup()

	// Set the window size greater than the maximum non-scaled window
	opt := types.ReceiveBufferSizeOption(65535 * 3)
	c.CreateConnected(789, 30000, &opt)

	data := []byte{1, 2, 3}
	view := buffer.NewView(len(data))
	copy(view, data)

	if _, err := c.EP.Write(view, nil); err != nil {
		t.Fatalf("Unexpected error from Write: %v", err)
	}

	// Check that data is received, and that advertised window is 0xffff,
	// that is not scaled
	b := c.GetPacket()
	checker.IPv4(t, b,
		checker.PayloadLen(len(data) + header.TCPMinimumSize),
		checker.TCP(
			checker.DstPort(context.TestPort),
			checker.SeqNum(uint32(c.IRS) + 1),
			checker.AckNum(790),
			checker.Window(0xffff),
			checker.TCPFlagsMatch(header.TCPFlagAck, ^uint8(header.TCPFlagPsh)),
		),
	)
}

func TestScaledWindowAccept(t *testing.T) {
	// This test ensures that window scaling is used when the peer
	// does advertise it and connection is established with Accept()
	c := context.New(t, defaultMTU)
	defer c.Cleanup()

	// Create EP and start listening
	wq := &waiter.Queue{}
	ep, err := c.Stack().NewEndpoint(tcp.ProtocolNumber, ipv4.ProtocolNumber, wq)
	if err != nil {
		t.Fatalf("NewEndpoint failed: %v", err)
	}
	defer ep.Close()

	// Set the window size greater than the maximum non-scaled window
	if err := ep.SetSockOpt(types.ReceiveBufferSizeOption(65535 * 3)); err != nil {
		t.Fatalf("SetSockOpt failed: %v", err)
	}

	if err := ep.Bind(types.FullAddress{Port: context.StackPort}); err != nil {
		t.Fatalf("Bind failed: %v", err)
	}

	if err := ep.Listen(10); err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	// Do 3-way handshake
	c.PassiveConnectWithOptions(100, 2, header.TCPSynOptions{MSS: defaultIPv4MSS})

	// Try to accept the connection
	we, ch := waiter.NewChannelEntry(nil)
	wq.EventRegister(&we, waiter.EventIn)
	defer wq.EventUnregister(&we)

	c.EP, _, err = ep.Accept()
	if err == types.ErrWouldBlock {
		// Wait for connection to be established
		select {
		case <-ch:
			c.EP, _, err = ep.Accept()
			if err != nil {
				t.Fatalf("Accept failed: %v", err)
			}

		case <-time.After(1 * time.Second):
			t.Fatalf("Timed out waiting for accept")
		}
	}

	data := []byte{1, 2, 3}
	view := buffer.NewView(len(data))
	copy(view, data)

	if _, err := c.EP.Write(view, nil); err != nil {
		t.Fatalf("Unexpected error from Write: %v", err)
	}

	// Check that data is received and that advertised window is 0xbfff,
	// that it is scaled
	b := c.GetPacket()
	checker.IPv4(t, b,
		checker.PayloadLen(len(data) + header.TCPMinimumSize),
		checker.TCP(
			checker.DstPort(context.TestPort),
			checker.SeqNum(uint32(c.IRS) + 1),
			checker.AckNum(790),
			checker.Window(0xbfff),
			checker.TCPFlagsMatch(header.TCPFlagAck, ^uint8(header.TCPFlagPsh)),
		),
	)
}
