package main

import (
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

type Addr []byte

// String serializes SOCKS address a to string form.
func (a Addr) String() string {
	var host, port string

	switch a[0] { // address type
	case AtypDomainName:
		host = string(a[2 : 2+int(a[1])])
		port = strconv.Itoa((int(a[2+int(a[1])]) << 8) | int(a[2+int(a[1])+1]))
	case AtypIPv4:
		host = net.IP(a[1 : 1+net.IPv4len]).String()
		port = strconv.Itoa((int(a[1+net.IPv4len]) << 8) | int(a[1+net.IPv4len+1]))
	case AtypIPv6:
		host = net.IP(a[1 : 1+net.IPv6len]).String()
		port = strconv.Itoa((int(a[1+net.IPv6len]) << 8) | int(a[1+net.IPv6len+1]))
	}

	return net.JoinHostPort(host, port)
}

const (
	AtypIPv4       = 1
	AtypDomainName = 3
	AtypIPv6       = 4
	// MaxAddrLen is the maximum size of SOCKS address in bytes.
	MaxAddrLen = 1 + 1 + 255 + 2
)

var local = flag.String("local", "192.168.0.104:8805", "please enter local ip")

func main() {
	flag.Parse()

	l, err := net.Listen("tcp", *local)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", *local, err)
		return
	}

	log.Println("listening TCP on ", *local)
	for {
		c, err := l.Accept()
		if err != nil {
			log.Fatalf("failed to accept: %v", err)
		}

		go func() {
			defer c.Close()

			tgt, err := ReadAddr(c)
			if err != nil {
				log.Fatalf("failed to get target address from %v: %v", c.RemoteAddr(), err)
				// drain c to avoid leaking server behavioral features
				// see https://www.ndss-symposium.org/ndss-paper/detecting-probe-resistant-proxies/
				return
			}
			log.Println("target ip :" + tgt.String() + "\n")
			rc, err := net.Dial("tcp", tgt.String())
			if err != nil {
				log.Fatalf("failed to connect to target: %v", err)
				return
			}
			defer rc.Close()

			log.Println("proxy " + c.RemoteAddr().String() + " <---> " + tgt.String())
			if err = relay(c, rc); err != nil {
				log.Fatalf("relay error: %v", err)
			}
		}()
	}
}

// 数据流的转发
func relay(left, right net.Conn) error {
	var err, err1 error
	var wg sync.WaitGroup
	var wait = 5 * time.Second
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err1 = SecureCopy(right, left)
		right.SetReadDeadline(time.Now().Add(wait)) // unblock read on right
	}()
	_, err = SecureCopy(left, right)
	left.SetReadDeadline(time.Now().Add(wait)) // unblock read on left
	wg.Wait()
	if err1 != nil && !errors.Is(err1, os.ErrDeadlineExceeded) { // requires Go 1.15+
		return err1
	}
	if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
		return err
	}
	return nil
}

// ReadAddr reads just enough bytes from r to get a valid Addr.
func ReadAddr(r io.Reader) (Addr, error) {
	return readAddr(r, make([]byte, MaxAddrLen))
}

func readAddr(r io.Reader, b []byte) (Addr, error) {
	if len(b) < MaxAddrLen {
		return nil, io.ErrShortBuffer
	}
	_, err := io.ReadFull(r, b[:1]) // read 1st byte for address type
	if err != nil {
		return nil, err
	}

	switch b[0] {
	case AtypDomainName:
		_, err = io.ReadFull(r, b[1:2]) // read 2nd byte for domain length
		if err != nil {
			return nil, err
		}
		_, err = io.ReadFull(r, b[2:2+int(b[1])+2])
		return b[:1+1+int(b[1])+2], err
	case AtypIPv4:
		_, err = io.ReadFull(r, b[1:1+net.IPv4len+2])
		return b[:1+net.IPv4len+2], err
	case AtypIPv6:
		_, err = io.ReadFull(r, b[1:1+net.IPv6len+2])
		return b[:1+net.IPv6len+2], err
	}

	return nil, err
}

func SecureCopy(src io.ReadWriteCloser, dst io.ReadWriteCloser) (written int64, err error) {
	size := 1024
	buf := make([]byte, size)
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}
