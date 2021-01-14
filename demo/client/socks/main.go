package main

import (
	"encoding/binary"
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
var server = flag.String("server", "192.168.0.104:8805", "please enter tcp tunnel server ip")

func main() {
	flag.Parse()
	l, err := net.Listen("tcp", *local)
	if err != nil {
		fmt.Printf("Listen failed: %v\n", err)
		return
	}

	for {
		c, err := l.Accept()
		if err != nil {
			fmt.Printf("Accept failed: %v", err)
			continue
		}

		go func() {

			if err := socks5Auth(c); err != nil {
				fmt.Println("auth error:", err)
				c.Close()
				return
			}

			targetIP, err := getTargetIP(c)
			if err != nil {
				log.Fatal("don't get target ip ,err is " + err.Error())
				return
			}

			tgt := ParseAddr(targetIP)

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

			log.Println("proxy " + c.RemoteAddr().String() + "<->" + *server + "<->" + targetIP)
			if err = relay(rc, c); err != nil {
				log.Fatal(err.Error() + "\n")
			}
		}()
	}
}

func socks5Auth(client net.Conn) (err error) {
	buf := make([]byte, 256)

	// 读取 VER 和 NMETHODS
	n, err := io.ReadFull(client, buf[:2])
	if n != 2 {
		return errors.New("reading header: " + err.Error())
	}

	ver, nMethods := int(buf[0]), int(buf[1])
	if ver != 5 {
		return errors.New("invalid version")
	}

	// 读取 METHODS 列表
	n, err = io.ReadFull(client, buf[:nMethods])
	if n != nMethods {
		return errors.New("reading methods: " + err.Error())
	}

	n, err = client.Write([]byte{0x05, 0x00})
	if n != 2 || err != nil {
		return errors.New("write rsp err: " + err.Error())
	}

	return nil
}

func getTargetIP(client net.Conn) (string, error) {
	buf := make([]byte, 256)

	n, err := io.ReadFull(client, buf[:4])
	if n != 4 {
		return "", errors.New("read header: " + err.Error())
	}

	ver, cmd, _, atyp := buf[0], buf[1], buf[2], buf[3]
	if ver != 5 || cmd != 1 {
		return "", errors.New("invalid ver/cmd")
	}

	addr := ""
	switch atyp {
	case 1:
		n, err = io.ReadFull(client, buf[:4])
		if n != 4 {
			return "", errors.New("invalid IPv4: " + err.Error())
		}
		addr = fmt.Sprintf("%d.%d.%d.%d", buf[0], buf[1], buf[2], buf[3])
	case 3:
		n, err = io.ReadFull(client, buf[:1])
		if n != 1 {
			return "", errors.New("invalid hostname: " + err.Error())
		}
		addrLen := int(buf[0])
		n, err = io.ReadFull(client, buf[:addrLen])
		if n != addrLen {
			return "", errors.New("invalid hostname: " + err.Error())
		}
		addr = string(buf[:addrLen])
	case 4:
		return "", errors.New("IPv6: no supported yet")
	default:
		return "", errors.New("invalid atyp")
	}

	n, err = io.ReadFull(client, buf[:2])
	if n != 2 {
		return "", errors.New("read port: " + err.Error())
	}
	port := binary.BigEndian.Uint16(buf[:2])

	destAddrPort := fmt.Sprintf("%s:%d", addr, port)

	return destAddrPort, nil
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
