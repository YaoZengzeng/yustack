package stack

import (
	"github.com/YaoZengzeng/yustack/types"
)

var (
	networkProtocols = make(map[string]types.NetworkProtocolFactory)
)

// RegisterNetworkProtocolFactory registers a new network protocol factory with
// the stack so that it becomes available to users of the stack. This function
// is intended to be called by init() functions of the protocols.
func RegisterNetworkProtocolFactory(name string, p types.NetworkProtocolFactory) {
	networkProtocols[name] = p
}
