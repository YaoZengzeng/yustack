package ipv4

import (
	"log"

	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/header"
	"github.com/YaoZengzeng/yustack/buffer"
)

type echoRequest struct {
	r *types.Route
	v buffer.View
}

func (e *endpoint) echoReplier() {
	for req := range e.echoRequests {
		sendICMPv4(req.r, header.ICMPv4EchoReply, 0, req.v)
	}
}

func sendICMPv4(r *types.Route, typ header.ICMPv4Type, code byte, data buffer.View) error {
	hdr := buffer.NewPrependable(header.ICMPv4MinimumSize + int(r.MaxHeaderLength()))

	icmpv4 := header.ICMPv4(hdr.Prepend(header.ICMPv4MinimumSize))
	icmpv4.SetType(typ)
	icmpv4.SetCode(code)
	icmpv4.SetChecksum(^header.Checksum(icmpv4, header.Checksum(data, 0)))

	return r.WritePacket(&hdr, data, header.ICMPv4ProtocolNumber)
}

func (e *endpoint) handleICMP(r *types.Route, vv *buffer.VectorisedView) {
	v := vv.First()
	if len(v) < header.ICMPv4MinimumSize {
		log.Printf("handleICMP: the packet is not big enough\n")
		return
	}

	h := header.ICMPv4(v)

	switch h.Type() {
	case header.ICMPv4Echo:
		vv.TrimFront(header.ICMPv4MinimumSize)
		e.echoRequests <- echoRequest{r: r, v: vv.ToView()}
	case header.ICMPv4EchoReply:
		log.Printf("handleICMP: branch header.ICMPv4EchoReply has not implemented\n")
	}
}

