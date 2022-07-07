package main

import (
	b64 "encoding/base64"
	"encoding/json"
	"flag"
	"log"
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
	flag.StringVar(&bootstrap, "bootstrap", "ws://localhost:6501/ws", "bootstrap server")
	flag.BoolVar(&selfsigned, "selfsigned", false, "accept self signed certificates")
}

func fakeSearch(m *minedive.Client, peer string, q string, lang string, key string, nonce string) {
	log.Println(peer, q, lang)
	l2, ok := m.GetL2PeerIfExists(peer)
	if !ok {
		log.Println(peer, "do not exists")
		return
	}

	msg := minedive.L2Msg{
		Type:  "resp",
		Query: q,
		Text:  []string{"https://example.com", "http://example.com/", "https://example.com/", "https://nlnet.nl/", "https://quitelongexampleurlusefulfortesting.com/sdjioasjg/asjpdaihj90arfau0094/shdga9ohfgaufhahoigdfa/"},
	}
	b, err := json.Marshal(msg)
	if err != nil {
		log.Println("HandleL2 Resp Marshal:", err)
		return
	}
	_ = b
	err = m.SendL2(l2, b)
	if err != nil {
		log.Println("HandleL2 Resp SendL2:", err)
		return
	}
}

func main() {
	flag.Parse()
	m := make([]*minedive.Client, n)
	for i := 0; i < n; i++ {
		m[i] = minedive.DialMiddle(bootstrap)
		m[i].Searcher = fakeSearch
		go m[i].KeepAlive(20 * time.Second)
		go func(j int) {
			var cell minedive.Cell
			cell.Type = "pubk"
			cell.D0 = b64.StdEncoding.EncodeToString(m[j].PK[:])
			minedive.JSONSuccessSend(m[j], cell)
		}(i)
	}
	select {}
}
