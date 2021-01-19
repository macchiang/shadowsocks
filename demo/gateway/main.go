package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
)

var local = flag.String("local", "", "please enter monitor ip, example: 192.168.0.104:8805")
var target = flag.String("target", "", "please enter target ip, example : 192.168.1.105:8806")

func main() {
	flag.Parse()
	l, err := net.Listen("tcp", *local)
	if err != nil {
		fmt.Println(err, err.Error())
		os.Exit(0)
	}

	for {
		s_conn, err := l.Accept()
		log.Println("接收到 " + s_conn.RemoteAddr().String() + " 发送请求")
		if err != nil {
			continue
		}

		d_tcpAddr, _ := net.ResolveTCPAddr("tcp4", *target)
		d_conn, err := net.DialTCP("tcp", nil, d_tcpAddr)
		log.Println("向 " + *target + " 发送请求")
		if err != nil {
			fmt.Println(err)
			s_conn.Write([]byte("can't connect " + *target))
			s_conn.Close()
			continue
		}
		go io.Copy(s_conn, d_conn)
		go io.Copy(d_conn, s_conn)
	}
}
