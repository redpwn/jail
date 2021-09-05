package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/redpwn/pow"
	"golang.org/x/sys/unix"
	"inet.af/netaddr"
)

type server struct {
	cfg        *jailConfig
	errCh      chan<- error
	countMu    sync.Mutex
	countPerIp map[netaddr.IP]uint32
	countTotal uint32
}

func (s *server) connInc(ip netaddr.IP) bool {
	s.countMu.Lock()
	defer s.countMu.Unlock()
	if (s.cfg.Conns > 0 && s.countTotal >= s.cfg.Conns) || (s.cfg.ConnsPerIp > 0 && s.countPerIp[ip] >= s.cfg.ConnsPerIp) {
		return false
	}
	s.countPerIp[ip]++
	s.countTotal++
	return true
}

func (s *server) connDec(ip netaddr.IP) {
	s.countMu.Lock()
	defer s.countMu.Unlock()
	s.countPerIp[ip]--
	if s.countPerIp[ip] <= 0 {
		delete(s.countPerIp, ip)
	}
	s.countTotal--
}

// readBuf reads the internal buffer from bufio.Reader
func readBuf(r *bufio.Reader) []byte {
	b := make([]byte, r.Buffered())
	r.Read(b)
	return b
}

func runCopy(dst io.Writer, src io.Reader, addr *net.TCPAddr, ch chan<- struct{}) {
	if _, err := io.Copy(dst, src); err != nil && !errors.Is(err, net.ErrClosed) {
		log.Println(fmt.Errorf("connection %s: copy: %w", addr, err))
	}
	ch <- struct{}{}
}

func (s *server) runConn(inConn net.Conn) {
	defer inConn.Close()
	addr := inConn.RemoteAddr().(*net.TCPAddr)
	log.Printf("connection %s: connect", addr)
	defer log.Printf("connection %s: close", addr)
	ip, ok := netaddr.FromStdIP(addr.IP)
	if !ok {
		return
	}

	if !s.connInc(ip) {
		log.Printf("connection %s: limit reached", addr)
		return
	}
	defer s.connDec(ip)

	chall := pow.GenerateChallenge(s.cfg.Pow)
	fmt.Fprintf(inConn, "proof of work: curl -sSfL https://pwn.red/pow | sh -s %s\nsolution: ", chall)
	r := bufio.NewReader(io.LimitReader(inConn, 1024)) // prevent DoS
	proof, err := r.ReadString('\n')
	if err != nil {
		return
	}
	if good, err := chall.Check(strings.TrimSpace(proof)); err != nil || !good {
		log.Printf("connection %s: bad pow", addr)
		inConn.Write([]byte("incorrect proof of work\n"))
		return
	}

	log.Printf("connection %s: forwarding", addr)
	outConn, err := net.Dial("tcp", fmt.Sprintf(":%d", s.cfg.Port+1))
	if err != nil {
		s.errCh <- err
		return
	}
	defer outConn.Close()
	outConn.Write(readBuf(r))
	eofCh := make(chan struct{})
	go runCopy(inConn, outConn, addr, eofCh)
	go runCopy(outConn, inConn, addr, eofCh)
	<-eofCh
}

func startServer(cfg *jailConfig, errCh chan<- error) {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		errCh <- err
		return
	}
	log.Printf("listening on %d", cfg.Port)
	defer l.Close()
	s := &server{
		cfg:        cfg,
		errCh:      errCh,
		countPerIp: make(map[netaddr.IP]uint32),
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go s.runConn(conn)
	}
}

func runServer(cfg *jailConfig) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := dropPrivs(cfg); err != nil {
		return err
	}
	if err := unix.Exec("/jail/run", []string{"run", "server"}, os.Environ()); err != nil {
		return fmt.Errorf("exec run: %w", err)
	}
	return nil
}
