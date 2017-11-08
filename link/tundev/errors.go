package tundev

import (
	"syscall"

	"github.com/YaoZengzeng/yustack/types"
)

var translations = map[syscall.Errno]*types.Error{
	syscall.EEXIST:        types.ErrDuplicateAddress,
	syscall.ENETUNREACH:   types.ErrNoRoute,
	syscall.EINVAL:        types.ErrInvalidEndpointState,
	syscall.EALREADY:      types.ErrAlreadyConnecting,
	syscall.EISCONN:       types.ErrAlreadyConnected,
	syscall.EADDRINUSE:    types.ErrPortInUse,
	syscall.EADDRNOTAVAIL: types.ErrBadLocalAddress,
	syscall.EPIPE:         types.ErrClosedForSend,
	syscall.EWOULDBLOCK:   types.ErrWouldBlock,
	syscall.ECONNREFUSED:  types.ErrConnectionRefused,
	syscall.ETIMEDOUT:     types.ErrTimeout,
	syscall.EINPROGRESS:   types.ErrConnectStarted,
	syscall.EDESTADDRREQ:  types.ErrDestinationRequired,
	syscall.ENOTSUP:       types.ErrNotSupported,
	syscall.ENOTTY:        types.ErrQueueSizeNotSupported,
	syscall.ENOTCONN:      types.ErrNotConnected,
	syscall.ECONNRESET:    types.ErrConnectionReset,
	syscall.ECONNABORTED:  types.ErrConnectionAborted,
}

// TranslateErrno translate an errno from the syscall package into a
// *types.Error
//
// Not all errnos are supported and this function will panic on unrecognized
// errnos
func TranslateErrno(e syscall.Errno) error {
	if err, ok := translations[e]; ok {
		return err
	}

	return types.ErrInvalidEndpointState
}