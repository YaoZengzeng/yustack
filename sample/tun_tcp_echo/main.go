package main

import (
	"strings"
	"log"
	"os"
	"net"

	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/link/tundev"
	"github.com/YaoZengzeng/yustack/waiter"
	"github.com/YaoZengzeng/yustack/network/ipv4"
	"github.com/YaoZengzeng/yustack/stack"
	"github.com/YaoZengzeng/yustack/transport/tcp"
)

const (
	stackAddr = "\x0a\x01\x00\x01"
	stackPort = 12345
)

const (
	nicId = 1
)

func echo(wq *waiter.Queue, ep types.Endpoint) {
	// Create wait queue entry that notifies a channel
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)

	wq.EventRegister(&waitEntry, waiter.EventIn)
	defer wq.EventUnregister(&waitEntry)

	for {
		v, err := ep.Read(nil)
		if err != nil {
			if err == types.ErrWouldBlock {
				<-notifyCh
				continue
			}

			return
		}
		log.Printf("Read %s\n", string(v))
		
		ep.Write(v, nil)
	}
}

func main() {
	if len(os.Args) != 3 {
		log.Fatal("Usage: ", os.Args[0], "<tun-device> <local-address>")
	}

	tunName := os.Args[1]
	address := os.Args[2]

	// Parse the IP address. Only support both ipv4.
	parseAddr := net.ParseIP(address)
	if parseAddr == nil {
		log.Fatalf("Bad IP address: %v", address)
	}

	var addr types.Address
	var proto types.NetworkProtocolNumber
	if parseAddr.To4() != nil {
		addr = types.Address(parseAddr.To4())
		proto = ipv4.ProtocolNumber
	} else {
		log.Fatalf("Unknown IP type: %v", address)
	}

	// Create the stack with only ipv4 temporarily, then add a tun-based
	// NIC and address.
	s := stack.New([]string{ipv4.ProtocolName}, []string{tcp.ProtocolName})

	linkId, err := tundev.New(tunName)
	if err != nil {
		log.Fatal(err)
	}

	if err := s.CreateNic(nicId, linkId); err != nil {
		log.Fatal(err)
	}

	if err := s.AddAddress(nicId, proto, addr); err != nil {
		log.Fatal(err)
	}

	// Add default route
	s.SetRouteTable([]types.RouteEntry{
		{
			Destination:		types.Address(strings.Repeat("\x00", len(addr))),
			Mask:				types.Address(strings.Repeat("\x00", len(addr))),
			Gateway:			"",
			Nic:				nicId,
		},
	})

	// Create tcp endpointm, bind it, then work as an echo server
	var wq waiter.Queue
	ep, err := s.NewEndpoint(tcp.ProtocolNumber, proto, &wq)
	if err != nil {
		log.Fatalf("tun_udp_echo: NewEndpoint failed: %v\n", err)
	}

	err = ep.Bind(types.FullAddress{0, "", uint16(stackPort)})
	if err != nil {
		log.Fatalf("Bind failed: %v\n", err)
	}

	err = ep.Listen(10);
	if err != nil {
		log.Fatalf("Listen failed: %v\n", err)
	}

	// Wait for connections to appear
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)
	wq.EventRegister(&waitEntry, waiter.EventIn)
	defer wq.EventUnregister(&waitEntry)

	for {
		n, wq, err := ep.Accept()
		if err != nil {
			if err == types.ErrWouldBlock {
				<-notifyCh
				continue
			}

			log.Fatalf("Accept() failed: %v", err)
		}

		go echo(wq, n)
	}

}

