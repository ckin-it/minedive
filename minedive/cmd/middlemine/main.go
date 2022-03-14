package main

import (
	"flag"
	"time"

	"github.com/ckin-it/minedive/minedive"
)

var (
	bootstrap  string
	selfsigned bool
	n          int
)

func init() {
	flag.IntVar(&n, "n", 4, "")
	flag.StringVar(&bootstrap, "bootstrap", "wss://localhost:6501/", "bootstrap server")
	flag.BoolVar(&selfsigned, "selfsigned", false, "accept self signed certificates")
}

func main() {
	flag.Parse()
	m := make([]*minedive.Client, n)
	for i := 0; i < n; i++ {
		m[i] = minedive.Dial(bootstrap)
		go m[i].KeepAlive(200 * time.Second)
		go func(j int) {
			for m[j].GetNPeers() < 1 {
				m[j].SingleCmd("getpeers")
				time.Sleep(3 * time.Second)
			}
		}(i)
	}
	select {}
}
