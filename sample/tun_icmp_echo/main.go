package main

import (
	"log"
	"net"
	"os"
	"time"
	"strings"

	"github.com/YaoZengzeng/yustack/network/ipv4"
	"github.com/YaoZengzeng/yustack/stack"
	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/link/tundev"
)

const (
	stackAddr = "\x0a\x01\x00\x01"
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
	s := stack.New([]string{ipv4.ProtocolName}, []string{ipv4.PingProtocolName})

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

	ch := make(chan ipv4.PingReply, 1)
	p := ipv4.Pinger{
		Stack: 		s,
		NicId:		1,
		Address:	stackAddr,
		Wait:		10 * time.Millisecond,
		Count:		1,	// one ping only
	}
	if err := p.Ping(ch); err != nil {
		log.Fatal("icmp.Ping failed\n")
	}

	ping := <-ch
	if ping.Error != nil {
		log.Fatal("bad ping response: %v\n", ping.Error)
	}

	log.Printf("ping test succeeded\n")

	select {}
}
