package tundev

import (
	"log"
	"syscall"
	"unsafe"

	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/stack"
	"github.com/YaoZengzeng/yustack/buffer"
	"github.com/YaoZengzeng/yustack/header"
)

// Placed here to avoid breakage caused by coverage
// instrumentation. Any, even unrelated, changes to this file should ensure
// that coverage still work
func blockingPoll(fds unsafe.Pointer, nfds int, timeout int64) (n int, err syscall.Errno)

// BufConfig defines the shape of the vectorized view used to read packets from the Nic
var BufConfig = []int{128, 256, 512, 1024}

type endpoint struct {
	// fd is the file descriptor used to send and receive packets
	fd int

	// mtu (maximum transmission unit) is the maximum size of a packets
	mtu uint32

	// The sized buffer of views
	vv 		*buffer.VectorisedView
	// Buffer used for system call
	iovecs 	[]syscall.Iovec
	// Buffer used to store raw data
	views	[]buffer.View
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

// WritePacket writes outbound packets to the file descriptor. If it is not writable
// right now, drop the packet
func (e *endpoint) WritePacket(r *types.Route, hdr *buffer.Prependable, payload buffer.View, protocol types.NetworkProtocolNumber) error {
	return nonBlockingWrite2(e.fd, hdr.UsedBytes(), payload)
}

// Attach launches the goroutine that reads packets from the file descriptor and
// dispatches them via the provided dispatcher
func (e *endpoint) Attach(dispatcher types.NetworkDispatcher) {
	go e.dispatchLoop(dispatcher)
}

// dispatchLoop reads packets from the file descriptor in a loop and dispatches
// them to the network stack
func (e *endpoint) dispatchLoop(d types.NetworkDispatcher) error {
	for {
		ok, err := e.dispatch(d)
		if err != nil || !ok {
			return nil
		}
	}
}

// dispatch reads one packet from the file descriptor and dispatches it
func (e *endpoint) dispatch(d types.NetworkDispatcher) (bool, error) {
	e.allocateViews(BufConfig)

	n, err := blockingReadv(e.fd, e.iovecs)
	if err != nil {
		return false, err
	}

	if n <= 0 {
		return false, nil
	}

	used := e.capViews(n, BufConfig)
	e.vv.SetViews(e.views[:used])
	e.vv.SetSize(n)

	// We don't get any indication of what the packet is, so try to guess
	// if it's an IPv4 packet
	var p types.NetworkProtocolNumber
	switch header.IPVersion(e.views[0]) {
	case header.IPv4Version:
		p = header.IPv4ProtocolNumber
		log.Printf("Network protocol is %x\n", p)
	default:
		log.Printf("Unknown network protocol, dropped\n")
		return true, nil
	}

	d.DeliverNetworkPacket(e, "", p, e.vv)

	// Prepare e.views for another packet: release used views
	for i := 0; i < used; i++ {
		e.views[i] = nil
	}

	return true, nil
}

func (e *endpoint) allocateViews(bufConfig []int) {
	for i, _ := range e.views {
		b := buffer.NewView(bufConfig[i])
		e.views[i] = b
		e.iovecs[i] = syscall.Iovec{
			Base:	&b[0],
			Len:	uint64(len(b)),
		}
	}
}

// blockingReadv reads from a file descriptor that is set up as non-blocking and
// stores the data in a list of iovecs buffers. If no data is available, it will
// block in a poll() syscall until the file descriptor becomes readable.
func blockingReadv(fd int, iovecs []syscall.Iovec) (int, error) {
	for {
		n, _, e := syscall.RawSyscall(syscall.SYS_READV, uintptr(fd), uintptr(unsafe.Pointer(&iovecs[0])), uintptr(len(iovecs)))
		if e == 0 {
			return int(n), nil
		}

		event := struct {
			fd 		uint32
			events	int16
			revents	int16
		}{
			fd:	uint32(fd),
			events:	1,		// POLLIN
		}

		_, e = blockingPoll(unsafe.Pointer(&event), 1, -1)
		if e != 0 && e != syscall.EINTR {
			return 0, TranslateErrno(e)
		}
	}
}

// NonBlockingWrite writes the given buffer to a file descriptor. It fails if
// partial data is written
func NonBlockingWrite(fd int, buf []byte) error {
	var ptr unsafe.Pointer
	if len(buf) > 0 {
		ptr = unsafe.Pointer(&buf[0])
	}

	_, _, e := syscall.RawSyscall(syscall.SYS_WRITE, uintptr(fd), uintptr(ptr), uintptr(len(buf)))
	if e != 0 {
		return TranslateErrno(e)
	}

	return nil
}

// NonBlockingWrite2 writes up to two byte slices to a file descriptor in a
// single syscall. It fails if partial data is written
func nonBlockingWrite2(fd int, b1, b2 []byte) error {
	// If there is no second buffer, issue a regular write
	if len(b2) == 0 {
		return NonBlockingWrite(fd, b1)
	}

	// We have tow buffers. Build the iovec that represents them and issue
	// a writev syscall
	iovec := [...]syscall.Iovec{
		{
			Base:	(*byte)(unsafe.Pointer(&b1[0])),
			Len:	uint64(len(b1)),
		},
		{
			Base:	(*byte)(unsafe.Pointer(&b2[0])),
			Len:	uint64(len(b2)),
		},
	}

	_, _, e := syscall.RawSyscall(syscall.SYS_WRITEV, uintptr(fd), uintptr(unsafe.Pointer(&iovec[0])), 2)
	if e != 0 {
		return TranslateErrno(e)
	}

	return nil
}

func (e *endpoint) capViews(n int, buffers []int) int {
	c := 0
	for i, s := range buffers {
		c += s
		if c >= n {
			e.views[i].CapLength(s - (c - n))
			return i + 1
		}
	}
	return len(buffers)
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
		views:	make([]buffer.View, len(BufConfig)),
		iovecs:	make([]syscall.Iovec, len(BufConfig)),
	}
	vv := buffer.NewVectorisedView(e.views, 0)
	e.vv = &vv

	return stack.RegisterLinkEndpoint(e), nil
}
