package server

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/netip"
	"os"
	"strings"
	"sync"

	"github.com/redpwn/jail/internal/config"
	"github.com/redpwn/jail/internal/privs"
	"github.com/redpwn/pow"
	"golang.org/x/sys/unix"
)

type proxyServer struct {
	cfg        *config.Config
	errCh      chan<- error
	countMu    sync.Mutex
	countPerIp map[netip.Addr]uint32
	countTotal uint32
}

func (p *proxyServer) connInc(ip netip.Addr) bool {
	p.countMu.Lock()
	defer p.countMu.Unlock()
	if (p.cfg.Conns > 0 && p.countTotal >= p.cfg.Conns) || (p.cfg.ConnsPerIp > 0 && p.countPerIp[ip] >= p.cfg.ConnsPerIp) {
		return false
	}
	p.countPerIp[ip]++
	p.countTotal++
	return true
}

func (p *proxyServer) connDec(ip netip.Addr) {
	p.countMu.Lock()
	defer p.countMu.Unlock()
	p.countPerIp[ip]--
	if p.countPerIp[ip] <= 0 {
		delete(p.countPerIp, ip)
	}
	p.countTotal--
}

// readBuf reads the internal buffer from bufio.Reader
func readBuf(r *bufio.Reader) []byte {
	b := make([]byte, r.Buffered())
	if _, err := r.Read(b); err != nil {
		panic(err)
	}
	return b
}

func runCopy(dst io.Writer, src io.Reader, addr *net.TCPAddr, ch chan<- struct{}) {
	if _, err := io.Copy(dst, src); err != nil && !errors.Is(err, net.ErrClosed) {
		log.Printf("connection %s: copy: %s", addr, err)
	}
	ch <- struct{}{}
}

func (p *proxyServer) runConn(inConn net.Conn) {
	defer inConn.Close()
	addr := inConn.RemoteAddr().(*net.TCPAddr)
	log.Printf("connection %s: connect", addr)
	defer log.Printf("connection %s: close", addr)
	ip, ok := netip.AddrFromSlice(addr.IP)
	if !ok {
		return
	}

	if !p.connInc(ip) {
		log.Printf("connection %s: limit reached", addr)
		return
	}
	defer p.connDec(ip)

	chall := pow.GenerateChallenge(p.cfg.Pow)
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
	port, _ := p.cfg.NsjailListen()
	outConn, err := net.Dial("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		p.errCh <- err
		return
	}
	defer outConn.Close()
	outConn.Write(readBuf(r))
	eofCh := make(chan struct{})
	go runCopy(inConn, outConn, addr, eofCh)
	go runCopy(outConn, inConn, addr, eofCh)
	<-eofCh
}

func startProxy(cfg *config.Config, errCh chan<- error) {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		errCh <- err
		return
	}
	log.Printf("listening on %d", cfg.Port)
	defer l.Close()
	p := &proxyServer{
		cfg:        cfg,
		errCh:      errCh,
		countPerIp: make(map[netip.Addr]uint32),
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go p.runConn(conn)
	}
}

const runPath = "/jail/run"

func execProxy(cfg *config.Config) error {
	if err := privs.DropPrivs(cfg); err != nil {
		return err
	}
	if err := unix.Exec(runPath, []string{runPath, "proxy"}, os.Environ()); err != nil {
		return fmt.Errorf("exec run: %w", err)
	}
	return nil
}
