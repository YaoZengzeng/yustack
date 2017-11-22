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
	_, err = s.NewEndpoint(tcp.ProtocolNumber, proto, &wq)
	if err != nil {
		log.Fatalf("tun_udp_echo: NewEndpoint failed: %v\n", err)
	}
}
