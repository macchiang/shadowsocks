package main

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"github.com/shadowsocks/go-shadowsocks2/core"
	"github.com/shadowsocks/go-shadowsocks2/socks"
	"io"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	var flags *Configure

	var filePath = flag.String("path", "/etc/shadowsocks/conf.configure.json", "shadowsocks configure file path")
	flag.Parse()

	if flags = loadConfigure(*filePath); flags == nil {
		return
	}

	if flags.Keygen > 0 {
		key := make([]byte, flags.Keygen)
		io.ReadFull(rand.Reader, key)
		fmt.Println(base64.URLEncoding.EncodeToString(key))
		return
	}

	if flags.Client == "" && flags.Server == "" {
		flag.Usage()
		return
	}

	var key []byte
	if flags.Key != "" {
		k, err := base64.URLEncoding.DecodeString(flags.Key)
		if err != nil {
			log.Fatal(err)
		}
		key = k
	}

	if flags.Client != "" { // client mode
		addr := flags.Client
		cipher := flags.Cipher
		password := flags.Password
		var err error

		if strings.HasPrefix(addr, "ss://") {
			addr, cipher, password, err = parseURL(addr)
			if err != nil {
				log.Fatal(err)
			}
		}

		udpAddr := addr

		ciph, err := core.PickCipher(cipher, key, password)
		if err != nil {
			log.Fatal(err)
		}

		if flags.Plugin != "" {
			addr, err = startPlugin(flags.Plugin, flags.PluginOpts, addr, false)
			if err != nil {
				log.Fatal(err)
			}
		}

		if flags.UDPTun != "" {
			for _, tun := range strings.Split(flags.UDPTun, ",") {
				p := strings.Split(tun, "=")
				go udpLocal(p[0], udpAddr, p[1], ciph.PacketConn)
			}
		}

		if flags.TCPTun != "" {
			for _, tun := range strings.Split(flags.TCPTun, ",") {
				p := strings.Split(tun, "=")
				go tcpTun(p[0], addr, p[1], ciph.StreamConn)
			}
		}

		if flags.Socks != "" {
			socks.UDPEnabled = flags.UDPSocks
			go socksLocal(flags.Socks, addr, ciph.StreamConn)
			if flags.UDPSocks {
				go udpSocksLocal(flags.Socks, udpAddr, ciph.PacketConn)
			}
		}

		if flags.RedirTCP != "" {
			go redirLocal(flags.RedirTCP, addr, ciph.StreamConn)
		}

		if flags.RedirTCP6 != "" {
			go redir6Local(flags.RedirTCP6, addr, ciph.StreamConn)
		}
	}

	if flags.Server != "" { // server mode
		addr := flags.Server
		cipher := flags.Cipher
		password := flags.Password
		var err error

		if strings.HasPrefix(addr, "ss://") {
			addr, cipher, password, err = parseURL(addr)
			if err != nil {
				log.Fatal(err)
			}
		}

		udpAddr := addr

		if flags.Plugin != "" {
			addr, err = startPlugin(flags.Plugin, flags.PluginOpts, addr, true)
			if err != nil {
				log.Fatal(err)
			}
		}

		ciph, err := core.PickCipher(cipher, key, password)
		if err != nil {
			log.Fatal(err)
		}

		if flags.UDP {
			go udpRemote(udpAddr, ciph.PacketConn)
		}
		if flags.TCP {
			go tcpRemote(addr, ciph.StreamConn)
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	killPlugin()
}

func parseURL(s string) (addr, cipher, password string, err error) {
	u, err := url.Parse(s)
	if err != nil {
		return
	}

	addr = u.Host
	if u.User != nil {
		cipher = u.User.Username()
		password, _ = u.User.Password()
	}
	return
}
