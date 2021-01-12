package main

import (
	"testing"
)

func TestConfigure(t *testing.T) {
	configure := loadConfigure("/home/wu/go/src/shadowsocks/conf/configure.json")
	t.Log(configure.UDPTimeout.String())
}
