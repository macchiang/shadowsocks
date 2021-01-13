package main

import (
	"testing"
)

func TestConfigure(t *testing.T) {
	configure := loadConfigure("/home/wu/go/src/shadowsocks/conf/client.json")
	t.Log(configure.UDPTimeout.String())
}
