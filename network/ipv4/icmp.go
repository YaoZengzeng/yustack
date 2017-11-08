package ipv4

import (
	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/buffer"
)

type echoRequest struct {
	r types.Route
	v buffer.View
}
