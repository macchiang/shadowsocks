package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
)

type Addr []byte

const (
	AtypIPv4       = 1
	AtypDomainName = 3
	AtypIPv6       = 4
)

var local = flag.String("local", "127.0.0.1:1080", "please enter local proxy ip")

//var target = flag.String("target", "192.168.1.105:8806", "please enter target server ip")
var server = flag.String("server", "192.168.0.104:8805", "please enter tcp tunnel server ip")

func main() {
	flag.Parse()
	//tgt := ParseAddr(*target)
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

		wg := new(sync.WaitGroup)
		wg.Add(2)

		go func() {

			target, localReq := GetRemoteIP(c)

			tgt := ParseAddr(target)
			rc, err := net.Dial("tcp", *server)
			if err != nil {
				log.Fatal("can't connect " + *server)
				c.Write([]byte("can't connect " + *server))
				c.Close()
			}

			// 发送目的IP地址
			if _, err = rc.Write(tgt); err != nil {
				log.Fatal("failed to send target address: ", err.Error()+"\n")
				return
			}

			// 转发数据
			log.Println("proxy " + c.RemoteAddr().String() + "<->" + *server + "<->" + target)
			go func() {
				defer wg.Done()
				rc.Write(localReq)
				// SecureCopy(localClient, dstServer, auth.Encrypt)
			}()

			go func() {
				defer wg.Done()
				SecureCopy(rc, c)
			}()

		}()

	}
}

func GetRemoteIP(localClient net.Conn) (string, []byte) {
	buff := make([]byte, 1024)

	n, err := localClient.Read(buff)
	if err != nil {
		log.Print(err)
	}
	localReq := buff[:n]
	j := 0
	z := 0
	httpreq := []string{}
	for i := 0; i < n; i++ {
		if buff[i] == 32 {
			httpreq = append(httpreq, string(buff[j:i]))
			j = i + 1
		}
		if buff[i] == 10 {
			z += 1
		}
	}

	dstURI, err := url.ParseRequestURI(httpreq[1])
	if err != nil {
		log.Print(err)
	}
	var dstAddr string
	var dstPort = "80"
	dstAddrPort := strings.Split(dstURI.Host, ":")
	if len(dstAddrPort) == 1 {
		dstAddr = dstAddrPort[0]
	} else if len(dstAddrPort) == 2 {
		dstAddr = dstAddrPort[0]
		dstPort = dstAddrPort[1]
	} else {
		log.Print("URL parse error!")
	}

	return fmt.Sprintf("%s:%s", dstAddr, dstPort), localReq
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
