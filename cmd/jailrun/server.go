package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"sync"

	"golang.org/x/sys/unix"
	"github.com/redpwn/pow"
	"inet.af/netaddr"
)

var connMu sync.Mutex
var connCountPerIp = make(map[netaddr.IP]uint32)
var	connCount uint32

func connInc(cfg *jailConfig, ip netaddr.IP) bool {
	connMu.Lock()
	defer connMu.Unlock()
	if (cfg.Conns > 0 && connCount >= cfg.Conns) || (cfg.ConnsPerIp > 0 && connCountPerIp[ip] >= cfg.ConnsPerIp) {
		return false
	}
	connCountPerIp[ip] += 1
	connCount += 1
	return true
}

func connDec(ip netaddr.IP) {
	connMu.Lock()
	defer connMu.Unlock()
	connCountPerIp[ip] -= 1
	connCount -= 1
}

func runConn(cfg *jailConfig, c net.Conn, errCh chan error) {
	defer c.Close()
	addr := c.RemoteAddr().(*net.TCPAddr)
	log.Printf("connection: %s", addr)
	ip, ok := netaddr.FromStdIP(addr.IP)
	if !ok {
		return
	}
	if !connInc(cfg, ip) {
		log.Printf("connection: %s: limit reached", addr)
		return
	}
	defer connDec(ip)
	chal := pow.GenerateChallenge(cfg.Pow)
	r := bufio.NewReader(io.LimitReader(c, 1024))
	c.Write([]byte(fmt.Sprintf("proof of work: curl -sSfL https://pwn.red/pow | sh -s %s\nsolution: ", chal)))
	s, err := r.ReadString('\n')
	if err != nil {
		return
	}
	correct, err := chal.Check(s)
	if err != nil || !correct {
		log.Printf("connection: %s: bad pow", addr)
		c.Write([]byte("incorrect proof of work\n"))
		return
	}
	log.Printf("connection: %s: forwarding", addr)
	d, err := net.Dial("tcp", fmt.Sprintf(":%d", cfg.Port+1))
	if err != nil {
		errCh <- err
		return
	}
	defer d.Close()
	eofCh := make(chan struct{})
	go func() {
		io.Copy(c, d)
		eofCh <- struct{}{}
	}()
	go func() {
		io.Copy(d, c)
		eofCh <- struct{}{}
	}()
	<-eofCh
}

func startServer(cfg *jailConfig, errCh chan error) {
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
