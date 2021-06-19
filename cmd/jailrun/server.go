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

var connCount = struct {
	mu    sync.Mutex
	perIp map[netaddr.IP]uint32
	total uint32
}{perIp: make(map[netaddr.IP]uint32)}

func connInc(cfg *jailConfig, ip netaddr.IP) bool {
	connCount.mu.Lock()
	defer connCount.mu.Unlock()
	if (cfg.Conns > 0 && connCount.total >= cfg.Conns) || (cfg.ConnsPerIp > 0 && connCount.perIp[ip] >= cfg.ConnsPerIp) {
		return false
	}
	connCount.perIp[ip]--
	connCount.total--
	return true
}

func connDec(ip netaddr.IP) {
	connCount.mu.Lock()
	defer connCount.mu.Unlock()
	connCount.perIp[ip]--
	connCount.total--
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

func runConn(cfg *jailConfig, c net.Conn, errCh chan<- error) {
	defer c.Close()
	addr := c.RemoteAddr().(*net.TCPAddr)
	log.Printf("connection %s: connect", addr)
	defer log.Printf("connection %s: close", addr)
	ip, ok := netaddr.FromStdIP(addr.IP)
	if !ok {
		return
	}
	if !connInc(cfg, ip) {
		log.Printf("connection %s: limit reached", addr)
		return
	}
	defer connDec(ip)
	chal := pow.GenerateChallenge(cfg.Pow)
	r := bufio.NewReader(io.LimitReader(c, 1024)) // prevent denial of service
	c.Write([]byte(fmt.Sprintf("proof of work: curl -sSfL https://pwn.red/pow | sh -s %s\nsolution: ", chal)))
	s, err := r.ReadString('\n')
	if err != nil {
		return
	}
	if correct, err := chal.Check(strings.TrimSpace(s)); err != nil || !correct {
		log.Printf("connection %s: bad pow", addr)
		c.Write([]byte("incorrect proof of work\n"))
		return
	}
	log.Printf("connection %s: forwarding", addr)
	d, err := net.Dial("tcp", fmt.Sprintf(":%d", cfg.Port+1))
	if err != nil {
		errCh <- err
		return
	}
	defer d.Close()
	d.Write(readBuf(r))
	eofCh := make(chan struct{})
	go runCopy(c, d, addr, eofCh)
	go runCopy(d, c, addr, eofCh)
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
	for {
		c, err := l.Accept()
		if err != nil {
			log.Println(err)
		}
		go runConn(cfg, c, errCh)
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
