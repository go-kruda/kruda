//go:build linux

package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	ioringOffSQRing = 0
	ioringOffCQRing = 0x8000000
	ioringOffSQEs   = 0x10000000

	ioringEnterGetEvents       = 1
	ioringEnterSQWakeup        = 1 << 1
	ioringCQEFMore             = 1 << 1
	ioringRecvsendPollFirst    = 1
	ioringAcceptMultishot      = 1
	ioringSetupSQPoll          = 1 << 1
	ioringSetupCoopTaskrun     = 1 << 8
	ioringSetupSingleIssuer    = 1 << 12
	ioringSetupDeferTaskrun    = 1 << 13
	ioringOpAccept             = 13
	ioringOpSend               = 26
	ioringOpRecv               = 27
	defaultResponse            = "HTTP/1.1 200 OK\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Length: 13\r\nConnection: keep-alive\r\n\r\nHello, World!"
	opAccept                op = 1
	opRecv                  op = 2
	opSend                  op = 3
)

type op uint64

type ioSqringOffsets struct {
	head        uint32
	tail        uint32
	ringMask    uint32
	ringEntries uint32
	flags       uint32
	dropped     uint32
	array       uint32
	resv1       uint32
	userAddr    uint64
}

type ioCqringOffsets struct {
	head        uint32
	tail        uint32
	ringMask    uint32
	ringEntries uint32
	overflow    uint32
	cqes        uint32
	flags       uint32
	resv1       uint32
	userAddr    uint64
}

type ioUringParams struct {
	sqEntries    uint32
	cqEntries    uint32
	flags        uint32
	sqThreadCPU  uint32
	sqThreadIdle uint32
	features     uint32
	wqFD         uint32
	resv         [3]uint32
	sqOff        ioSqringOffsets
	cqOff        ioCqringOffsets
}

type ioUringSqe struct {
	opcode      uint8
	flags       uint8
	ioprio      uint16
	fd          int32
	off         uint64
	addr        uint64
	len         uint32
	rwFlags     uint32
	userData    uint64
	bufIndex    uint16
	personality uint16
	spliceFdIn  int32
	addr3       uint64
	pad2        [1]uint64
}

type ioUringCqe struct {
	userData uint64
	res      int32
	flags    uint32
}

type ring struct {
	fd            int
	params        ioUringParams
	sqRing        []byte
	cqRing        []byte
	sqes          []byte
	pendingSubmit uint32
}

type ringOptions struct {
	flags        uint32
	sqThreadIdle uint32
}

type conn struct {
	fd     int
	read   []byte
	resp   []byte
	sent   int
	closed bool
}

type stats struct {
	requests uint64
	accepts  uint64
	errors   uint64
}

type server struct {
	id       int
	r        *ring
	listenFD int
	conns    map[uint64]*conn
	nextID   uint64
	response []byte
	options  serverOptions
	stats    *stats
}

type serverOptions struct {
	multishotAccept bool
	submitBatch     uint32
}

func main() {
	addr := flag.String("addr", "127.0.0.1", "listen address")
	port := flag.Int("port", 4555, "listen port")
	entries := flag.Uint("entries", 4096, "io_uring queue entries")
	readSize := flag.Int("read-size", 4096, "per-connection read buffer size")
	workers := flag.Int("workers", 4, "SO_REUSEPORT worker count")
	multishotAccept := flag.Bool("multishot-accept", false, "enable IORING_ACCEPT_MULTISHOT")
	submitBatch := flag.Uint("submit-batch", 0, "flush pending submissions after this many queued SQEs; 0 submits when waiting for completions")
	sqPoll := flag.Bool("sqpoll", false, "enable IORING_SETUP_SQPOLL")
	sqPollIdle := flag.Uint("sqpoll-idle-ms", 1000, "SQPOLL thread idle time in milliseconds")
	singleIssuer := flag.Bool("single-issuer", false, "enable IORING_SETUP_SINGLE_ISSUER")
	coopTaskrun := flag.Bool("coop-taskrun", false, "enable IORING_SETUP_COOP_TASKRUN")
	deferTaskrun := flag.Bool("defer-taskrun", false, "enable IORING_SETUP_DEFER_TASKRUN")
	flag.Parse()

	if *entries == 0 || *readSize <= 0 || *workers <= 0 {
		fmt.Fprintln(os.Stderr, "entries, read-size, and workers must be positive")
		os.Exit(2)
	}
	if *submitBatch > *entries {
		fmt.Fprintln(os.Stderr, "submit-batch must be less than or equal to entries")
		os.Exit(2)
	}

	runtime.GOMAXPROCS(*workers)
	opts := ringOptions{}
	serverOpts := serverOptions{multishotAccept: *multishotAccept, submitBatch: uint32(*submitBatch)}
	if *sqPoll {
		opts.flags |= ioringSetupSQPoll
		opts.sqThreadIdle = uint32(*sqPollIdle)
	}
	if *singleIssuer {
		opts.flags |= ioringSetupSingleIssuer
	}
	if *coopTaskrun {
		opts.flags |= ioringSetupCoopTaskrun
	}
	if *deferTaskrun {
		opts.flags |= ioringSetupDeferTaskrun
	}
	st := &stats{}
	servers := make([]*server, 0, *workers)
	for i := 0; i < *workers; i++ {
		r, err := setupRing(uint32(*entries), opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "worker %d io_uring_setup: %v\n", i, err)
			os.Exit(1)
		}
		defer r.close()

		lfd, err := listenTCP(*addr, *port)
		if err != nil {
			fmt.Fprintf(os.Stderr, "worker %d listen: %v\n", i, err)
			os.Exit(1)
		}
		defer unix.Close(lfd)

		s := &server{
			id:       i,
			r:        r,
			listenFD: lfd,
			conns:    make(map[uint64]*conn, 4096),
			response: []byte(defaultResponse),
			options:  serverOpts,
			stats:    st,
		}
		s.submitAccept()
		servers = append(servers, s)
	}

	for i := 1; i < len(servers); i++ {
		go servers[i].loop(*readSize)
	}

	fmt.Printf("uring_http_probe=ready addr=%s:%d workers=%d entries=%d read_size=%d setup_flags=0x%x sqpoll_idle_ms=%d multishot_accept=%t submit_batch=%d\n",
		*addr, *port, *workers, servers[0].r.params.sqEntries, *readSize, opts.flags, opts.sqThreadIdle, serverOpts.multishotAccept, serverOpts.submitBatch)
	servers[0].loop(*readSize)
}

func listenTCP(addr string, port int) (int, error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return -1, err
	}
	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err != nil {
		_ = unix.Close(fd)
		return -1, err
	}
	_ = unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)

	ip := net.ParseIP(addr).To4()
	if ip == nil {
		_ = unix.Close(fd)
		return -1, fmt.Errorf("addr must be an IPv4 address")
	}
	sa := &unix.SockaddrInet4{Port: port}
	copy(sa.Addr[:], ip)
	if err := unix.Bind(fd, sa); err != nil {
		_ = unix.Close(fd)
		return -1, err
	}
	if err := unix.Listen(fd, 4096); err != nil {
		_ = unix.Close(fd)
		return -1, err
	}
	return fd, nil
}

func setupRing(entries uint32, opts ringOptions) (*ring, error) {
	p := ioUringParams{flags: opts.flags, sqThreadIdle: opts.sqThreadIdle}
	fd, _, errno := syscall.RawSyscall(
		uintptr(unix.SYS_IO_URING_SETUP),
		uintptr(entries),
		uintptr(unsafe.Pointer(&p)),
		0,
	)
	if errno != 0 {
		return nil, errno
	}

	r := &ring{fd: int(fd), params: p}
	sqRingSize := int(p.sqOff.array) + int(p.sqEntries)*4
	cqRingSize := int(p.cqOff.cqes) + int(p.cqEntries)*int(unsafe.Sizeof(ioUringCqe{}))
	sqesSize := int(p.sqEntries) * int(unsafe.Sizeof(ioUringSqe{}))

	var err error
	r.sqRing, err = unix.Mmap(r.fd, ioringOffSQRing, sqRingSize, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED|unix.MAP_POPULATE)
	if err != nil {
		_ = unix.Close(r.fd)
		return nil, fmt.Errorf("mmap sq ring: %w", err)
	}
	r.cqRing, err = unix.Mmap(r.fd, ioringOffCQRing, cqRingSize, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED|unix.MAP_POPULATE)
	if err != nil {
		_ = unix.Munmap(r.sqRing)
		_ = unix.Close(r.fd)
		return nil, fmt.Errorf("mmap cq ring: %w", err)
	}
	r.sqes, err = unix.Mmap(r.fd, ioringOffSQEs, sqesSize, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED|unix.MAP_POPULATE)
	if err != nil {
		_ = unix.Munmap(r.cqRing)
		_ = unix.Munmap(r.sqRing)
		_ = unix.Close(r.fd)
		return nil, fmt.Errorf("mmap sqes: %w", err)
	}
	return r, nil
}

func (s *server) loop(readSize int) {
	for {
		cqe, err := s.r.waitCQE()
		if err != nil {
			fmt.Fprintf(os.Stderr, "wait cqe: %v\n", err)
			os.Exit(1)
		}
		kind, id := decodeUserData(cqe.userData)
		switch kind {
		case opAccept:
			s.handleAccept(cqe.res, cqe.flags, readSize)
		case opRecv:
			s.handleRecv(id, cqe.res)
		case opSend:
			s.handleSend(id, cqe.res)
		default:
			atomic.AddUint64(&s.stats.errors, 1)
		}
		if s.options.submitBatch > 0 && s.r.pendingSubmit >= s.options.submitBatch {
			if err := s.r.submitPending(0); err != nil {
				fmt.Fprintf(os.Stderr, "submit pending: %v\n", err)
				os.Exit(1)
			}
		}
	}
}

func (s *server) handleAccept(res int32, flags uint32, readSize int) {
	if !s.options.multishotAccept || flags&ioringCQEFMore == 0 {
		s.submitAccept()
	}
	if res < 0 {
		if res != -int32(unix.EAGAIN) && res != -int32(unix.EINTR) {
			atomic.AddUint64(&s.stats.errors, 1)
		}
		return
	}
	fd := int(res)
	_ = unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_NODELAY, 1)
	_ = unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_QUICKACK, 1)

	s.nextID++
	id := s.nextID
	s.conns[id] = &conn{
		fd:   fd,
		read: make([]byte, readSize),
		resp: s.response,
	}
	atomic.AddUint64(&s.stats.accepts, 1)
	s.submitRecv(id)
}

func (s *server) handleRecv(id uint64, res int32) {
	c := s.conns[id]
	if c == nil || c.closed {
		return
	}
	if res <= 0 {
		if res != 0 && res != -int32(unix.ECONNRESET) && res != -int32(unix.EAGAIN) {
			atomic.AddUint64(&s.stats.errors, 1)
		}
		s.closeConn(id)
		return
	}
	atomic.AddUint64(&s.stats.requests, 1)
	c.sent = 0
	s.submitSend(id)
}

func (s *server) handleSend(id uint64, res int32) {
	c := s.conns[id]
	if c == nil || c.closed {
		return
	}
	if res < 0 {
		if res != -int32(unix.ECONNRESET) && res != -int32(unix.EPIPE) && res != -int32(unix.EAGAIN) {
			atomic.AddUint64(&s.stats.errors, 1)
		}
		s.closeConn(id)
		return
	}
	c.sent += int(res)
	if c.sent < len(c.resp) {
		s.submitSend(id)
		return
	}
	s.submitRecv(id)
}

func (s *server) submitAccept() {
	sqe := s.r.nextSQE()
	sqe.opcode = ioringOpAccept
	sqe.fd = int32(s.listenFD)
	sqe.rwFlags = unix.SOCK_NONBLOCK | unix.SOCK_CLOEXEC
	if s.options.multishotAccept {
		sqe.ioprio = ioringAcceptMultishot
	}
	sqe.userData = encodeUserData(opAccept, 0)
}

func (s *server) submitRecv(id uint64) {
	c := s.conns[id]
	if c == nil || c.closed {
		return
	}
	sqe := s.r.nextSQE()
	sqe.opcode = ioringOpRecv
	sqe.ioprio = ioringRecvsendPollFirst
	sqe.fd = int32(c.fd)
	sqe.addr = uint64(uintptr(unsafe.Pointer(&c.read[0])))
	sqe.len = uint32(len(c.read))
	sqe.userData = encodeUserData(opRecv, id)
}

func (s *server) submitSend(id uint64) {
	c := s.conns[id]
	if c == nil || c.closed || c.sent >= len(c.resp) {
		return
	}
	buf := c.resp[c.sent:]
	sqe := s.r.nextSQE()
	sqe.opcode = ioringOpSend
	sqe.fd = int32(c.fd)
	sqe.addr = uint64(uintptr(unsafe.Pointer(&buf[0])))
	sqe.len = uint32(len(buf))
	sqe.userData = encodeUserData(opSend, id)
}

func (s *server) closeConn(id uint64) {
	c := s.conns[id]
	if c == nil || c.closed {
		return
	}
	c.closed = true
	_ = unix.Close(c.fd)
	delete(s.conns, id)
}

func (r *ring) nextSQE() *ioUringSqe {
	sqHead := r.u32(r.sqRing, r.params.sqOff.head)
	sqTail := r.u32(r.sqRing, r.params.sqOff.tail)
	sqMask := r.u32(r.sqRing, r.params.sqOff.ringMask)
	sqEntries := r.u32(r.sqRing, r.params.sqOff.ringEntries)

	head := atomic.LoadUint32(sqHead)
	tail := atomic.LoadUint32(sqTail)
	if tail-head >= atomic.LoadUint32(sqEntries) {
		panic("io_uring submission queue full")
	}

	idx := tail & atomic.LoadUint32(sqMask)
	sqe := r.sqe(idx)
	*sqe = ioUringSqe{}
	*r.u32(r.sqRing, r.params.sqOff.array+idx*4) = idx
	atomic.StoreUint32(sqTail, tail+1)
	r.pendingSubmit++
	return sqe
}

func (r *ring) waitCQE() (ioUringCqe, error) {
	for {
		if cqe, ok := r.popCQE(); ok {
			return cqe, nil
		}
		if err := r.submitPending(1); err != nil {
			if err == syscall.EINTR {
				continue
			}
			return ioUringCqe{}, err
		}
	}
}

func (r *ring) submitPending(minComplete uint32) error {
	if r.pendingSubmit == 0 && minComplete == 0 {
		return nil
	}
	toSubmit := r.pendingSubmit
	r.pendingSubmit = 0
	flags := uintptr(0)
	if minComplete > 0 {
		flags |= ioringEnterGetEvents
	}
	if r.params.flags&ioringSetupSQPoll != 0 {
		flags |= ioringEnterSQWakeup
	}
	_, _, errno := syscall.RawSyscall6(
		uintptr(unix.SYS_IO_URING_ENTER),
		uintptr(r.fd),
		uintptr(toSubmit),
		uintptr(minComplete),
		flags,
		0,
		0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}

func (r *ring) popCQE() (ioUringCqe, bool) {
	cqHead := r.u32(r.cqRing, r.params.cqOff.head)
	cqTail := r.u32(r.cqRing, r.params.cqOff.tail)
	cqMask := r.u32(r.cqRing, r.params.cqOff.ringMask)

	head := atomic.LoadUint32(cqHead)
	tail := atomic.LoadUint32(cqTail)
	if head == tail {
		return ioUringCqe{}, false
	}
	idx := head & atomic.LoadUint32(cqMask)
	cqe := *r.cqe(idx)
	atomic.StoreUint32(cqHead, head+1)
	return cqe, true
}

func (r *ring) u32(buf []byte, off uint32) *uint32 {
	return (*uint32)(unsafe.Pointer(&buf[off]))
}

func (r *ring) sqe(idx uint32) *ioUringSqe {
	size := unsafe.Sizeof(ioUringSqe{})
	return (*ioUringSqe)(unsafe.Pointer(&r.sqes[uintptr(idx)*size]))
}

func (r *ring) cqe(idx uint32) *ioUringCqe {
	size := unsafe.Sizeof(ioUringCqe{})
	return (*ioUringCqe)(unsafe.Pointer(&r.cqRing[uintptr(r.params.cqOff.cqes)+uintptr(idx)*size]))
}

func (r *ring) close() {
	if r.sqes != nil {
		_ = unix.Munmap(r.sqes)
	}
	if r.cqRing != nil {
		_ = unix.Munmap(r.cqRing)
	}
	if r.sqRing != nil {
		_ = unix.Munmap(r.sqRing)
	}
	if r.fd >= 0 {
		_ = unix.Close(r.fd)
	}
}

func encodeUserData(kind op, id uint64) uint64 {
	return uint64(kind)<<56 | (id & 0x00ff_ffff_ffff_ffff)
}

func decodeUserData(v uint64) (op, uint64) {
	return op(v >> 56), v & 0x00ff_ffff_ffff_ffff
}
