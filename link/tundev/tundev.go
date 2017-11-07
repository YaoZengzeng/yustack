package tundev

import (
	"syscall"
	"unsafe"

	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/stack"
)

type endpoint struct {
	// fd is the file descriptor used to send and receive packets
	fd int

	// mtu (maximum transmission unit) is the maximum size of a packets
	mtu uint32
}

// MTU implements stack.LinkEndpoint.MTU. It returns the value initialized
// during construction
func (e *endpoint) MTU() uint32 {
	return e.mtu
}

// MaxHeaderLength returns the maximum size of the header. Given that
// it doesn't have a header, it just returns 0
func (e *endpoint) MaxHeaderLength() uint16 {
	return 0
}

// LinkAddress returns the link address of this endpoint
func (e *endpoint) LinkAddress() types.LinkAddress {
	return ""
}

// getmtu determines the MTU of a network interface device
func getmtu(name string) (uint32, error) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return 0, err
	}
	defer syscall.Close(fd)

	var ifreq struct {
		name 	[16]byte
		mtu 	int32
		_		[20]byte
	}

	copy(ifreq.name[:], name)
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.SIOCGIFMTU, uintptr(unsafe.Pointer(&ifreq)))
	if errno != 0 {
		return 0, errno
	}

	return uint32(ifreq.mtu), nil
}

// open opens the specified tun device and returns its file descriptor
func open(name string) (int, error) {
	fd, err := syscall.Open("/dev/net/tun", syscall.O_RDWR, 0)
	if err != nil {
		return -1, err
	}

	var ifreq struct {
		name 	[16]byte
		flags	uint16
		_		[22]byte
	}

	copy(ifreq.name[:], name)
	ifreq.flags = syscall.IFF_TUN | syscall.IFF_NO_PI
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.TUNSETIFF, uintptr(unsafe.Pointer(&ifreq)))
	if errno != 0 {
		syscall.Close(fd)
		return -1, errno
	}

	return fd, nil
}

// New creates a new tun-based endpoint
func New(tunName string) (types.LinkEndpointID, error) {
	mtu, err := getmtu(tunName)
	if err != nil {
		return 0, err
	}

	fd, err := open(tunName)
	if err != nil {
		return 0, err
	}

	err = syscall.SetNonblock(fd, true)
	if err != nil {
		return 0, err
	}

	e := &endpoint{
		fd:		fd,
		mtu:	mtu,
	}

	return stack.RegisterLinkEndpoint(e), nil
}
