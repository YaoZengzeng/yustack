package main

import (
	"os"
	"log"
	"net"
	"bufio"
	"strconv"
	"strings"

	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/network/ipv4"
	"github.com/YaoZengzeng/yustack/transport/tcp"
	"github.com/YaoZengzeng/yustack/stack"
	"github.com/YaoZengzeng/yustack/waiter"
	"github.com/YaoZengzeng/yustack/link/tundev"
	"github.com/YaoZengzeng/yustack/buffer"
	"github.com/YaoZengzeng/yustack/link/sniffer"
)

const (
	nicId = 1
)

// writer reads from standard input and writes to the endpoint until standard
// input is closed. It signals that it's done by closing the provided channel
func writer(ch chan struct{}, ep types.Endpoint) {
	defer func() {
		ep.Shutdown(types.ShutdownWrite)
		close(ch)
	}()

	r := bufio.NewReader(os.Stdin)
	for {
		v := buffer.NewView(1024)
		n, err := r.Read(v)
		if err != nil {
			return
		}

		v.CapLength(n)
		for len(v) > 0 {
			n, err := ep.Write(v, nil)
			if err != nil {
				log.Printf("Write failed: %v", err)
				return
			}

			v.TrimFront(int(n))
		}
	}
}

func main() {
	if len(os.Args) != 6 {
		log.Fatal("Usage: ", os.Args[0], " <tun-dev> <local-ipv4-address> <local-port> <remote-ipv4-address> <remote-port>")
	}

	tunName := os.Args[1]
	addrName := os.Args[2]
	portName := os.Args[3]
	remoteAddrName := os.Args[4]
	remotePortName := os.Args[5]

	addr := types.Address(net.ParseIP(addrName).To4())
	remote := types.FullAddress{
		Nic:		1,
		Address:	types.Address(net.ParseIP(remoteAddrName).To4()),
	}

	var localPort uint16
	if v, err := strconv.Atoi(portName); err != nil {
		log.Fatal("Unable to convert port %v: %v", portName, err)
	} else {
		localPort = uint16(v)
	}

	if v, err := strconv.Atoi(remotePortName); err != nil {
		log.Fatal("Unable to convert port %v: %v", remotePortName, err)
	} else {
		remote.Port = uint16(v)
	}

	// Create the stack with ipv4 and tcp protocols, then add a tun-based
	// Nic and ipv4 address
	s := stack.New([]string{ipv4.ProtocolName}, []string{tcp.ProtocolName})

	linkId, err := tundev.New(tunName)
	if err != nil {
		log.Fatal(err)
	}

	if err := s.CreateNic(nicId, sniffer.New(linkId)); err != nil {
		log.Fatal(err)
	}

	if err := s.AddAddress(1, ipv4.ProtocolNumber, addr); err != nil {
		log.Fatal(err)
	}

	// Add default route
	s.SetRouteTable([]types.RouteEntry{
		{
			Destination:		types.Address(strings.Repeat("\x00", len(addr))),
			Mask:				types.Address(strings.Repeat("\x00", len(addr))),
			Gateway:			"",
			Nic:				1,	
		},
	})

	// Create TCP endpoint
	var wq waiter.Queue
	ep, err := s.NewEndpoint(tcp.ProtocolNumber, ipv4.ProtocolNumber, &wq)
	if err != nil {
		log.Fatal(err)
	}

	// Bind if a port is specified
	if localPort != 0 {
		if err := ep.Bind(types.FullAddress{0, "", localPort}); err != nil {
			log.Fatal("Bind failed: ", err)
		}
	}

	// Issue connect request and wait for it to complete
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)
	wq.EventRegister(&waitEntry, waiter.EventOut)
	err = ep.Connect(remote)
	if err == types.ErrConnectStarted {
		log.Printf("Connect is pending...")
		<-notifyCh
	} else {
		log.Fatalf("Unable to connect: %v", err)
	}
	wq.EventUnregister(&waitEntry)

	log.Printf("Connected")

	// Start the writer in its own goroutine
	writeCompletedCh :=make(chan struct{})
	go writer(writeCompletedCh, ep)

	// Read data and write to standard output until the peer closes the
	// connection from its side
	wq.EventRegister(&waitEntry, waiter.EventIn)
	for {
		v, err := ep.Read(nil)
		if err != nil {
			if err == types.ErrClosedForReceive {
				break
			}

			if err == types.ErrWouldBlock {
				<-notifyCh
				continue
			}

			log.Fatal("Read() failed:", err)
		}

		os.Stdout.Write(v)
	}
	wq.EventUnregister(&waitEntry)

	// The reader has completed. Now wait for the writer as well
	<-writeCompletedCh

	ep.Close()
}
