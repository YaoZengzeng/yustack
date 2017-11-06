package main

import (
	"log"
	"net"
	"os"

	"github.com/YaoZengzeng/yustack/network/ipv4"
	"github.com/YaoZengzeng/yustack/stack"
	"github.com/YaoZengzeng/yustack/types"
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

	log.Printf("address is %v, protocol is %v", addr, proto)

	// Create the stack with only ipv4 temporarily, then add a tun-based
	// NIC and address.
	stack.New([]string{ipv4.ProtocolName}, nil)

	select {}
}
