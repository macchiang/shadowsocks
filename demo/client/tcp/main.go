package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

type Addr []byte

const (
	AtypIPv4       = 1
	AtypDomainName = 3
	AtypIPv6       = 4
)

var local = flag.String("local", "127.0.0.1:1080", "please enter local proxy ip")
var target = flag.String("target", "192.168.1.105:8806", "please enter target server ip")
var server = flag.String("server", "192.168.0.104:8805", "please enter tcp tunnel server ip")

func main() {
	flag.Parse()
	tgt := ParseAddr(*target)
	l, err := net.Listen("tcp", *local)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", *local, err)
		return
	}

	log.Println("listening TCP on ", *local)
	if err != nil {
		fmt.Println(err, err.Error())
		os.Exit(0)
	}

	for {
		c, err := l.Accept()
		if err != nil {
			continue
		}

		go func() {

			rc, err := net.Dial("tcp", *server)
			if err != nil {
				log.Fatal("can't connect " + *server)
				c.Write([]byte("can't connect " + *server))
				c.Close()

			}

			if _, err = rc.Write(tgt); err != nil {
				log.Fatal("failed to send target address: ", err.Error()+"\n")
				return
			}

			log.Println("proxy " + c.RemoteAddr().String() + "<->" + *server + "<->" + *target)
			if err = relay(rc, c); err != nil {
				log.Fatal(err.Error() + "\n")
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
		_, err1 = io.Copy(right, left)
		right.SetReadDeadline(time.Now().Add(wait)) // unblock read on right
	}()
	_, err = io.Copy(left, right)
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

// 将string类型的IP地址转换成[]byte类型
func ParseAddr(s string) Addr {
	var addr Addr
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			addr = make([]byte, 1+net.IPv4len+2)
			addr[0] = AtypIPv4
			copy(addr[1:], ip4)
		} else {
			addr = make([]byte, 1+net.IPv6len+2)
			addr[0] = AtypIPv6
			copy(addr[1:], ip)
		}
	} else {
		if len(host) > 255 {
			return nil
		}
		addr = make([]byte, 1+1+len(host)+2)
		addr[0] = AtypDomainName
		addr[1] = byte(len(host))
		copy(addr[2:], host)
	}

	portnum, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return nil
	}

	addr[len(addr)-2], addr[len(addr)-1] = byte(portnum>>8), byte(portnum)

	return addr
}
