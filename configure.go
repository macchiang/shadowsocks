package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

type Configure struct {
	Client     string   `json:"client"`
	Server     string   `json:"server"`
	Cipher     string   `json:"cipher"`
	Key        string   `json:"key"`
	Password   string   `json:"password"`
	Keygen     int      `json:"keygen"`
	Socks      string   `json:"socks"`
	RedirTCP   string   `json:"redir_tcp"`
	RedirTCP6  string   `json:"redir_tcp_6"`
	TCPTun     string   `json:"tcp_tun"`
	UDPTun     string   `json:"udp_tun"`
	UDPSocks   bool     `json:"udp_socks"`
	UDP        bool     `json:"udp"`
	TCP        bool     `json:"tcp"`
	Plugin     string   `json:"plugin"`
	PluginOpts string   `json:"plugin_opts"`
	Verbose    bool     `json:"verbose"`
	UDPTimeout Duration `json:"udp_timeout"`
	TCPCork    bool     `json:"tcp_cork"`
}

var (
	config Configure
)

func loadConfigure(filePath string) *Configure {

	var (
		content []byte
		err     error
	)

	if !exists(filePath) {
		logf(filePath + " is not exits")
		return nil
	}

	if content, err = ioutil.ReadFile(filePath); err != nil {
		logf("Error reading configuration file, err is " + err.Error())
		return nil
	}

	if err := json.Unmarshal(content, &config); err != nil {
		logf("Error decoding json file, err is :" + err.Error())
		return nil
	}

	return &config
}

func exists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}
