package main

import (
	"context"
	b64 "encoding/base64"
	"errors"
	"flag"
	"fmt"
	"hash/fnv"
	"log"
	"net/http"
	"runtime"
	"time"

	"github.com/ckin-it/minedive/minedive"
	"nhooyr.io/websocket/wsjson"
)

var s minedive.MinediveServer

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

func printMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	fmt.Printf("Alloc = %v MiB", bToMb(m.Alloc))
	fmt.Printf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
	fmt.Printf("\tSys = %v MiB", bToMb(m.Sys))
	fmt.Printf("\tNumGC = %v\n", m.NumGC)
}

func incNonce(a []byte, dyn int) error {
	l := len(a)
	if l < dyn {
		dyn = l
	}
	for i := 1; i <= dyn; i++ {
		if a[l-i] < 0xff {
			a[l-i]++
			return nil
		}
		a[l-i] = 0
	}
	return errors.New("incNonce: nonce expired")
}

func fnvhash(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

func minediveDispatch(cli *minedive.MinediveClient, m minedive.Cell) {
	if m.Type != "ping" {
		log.Println("New msg:", m.Type)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	switch m.Type {
	case "gettid":
		b64pk, err := b64.StdEncoding.DecodeString(m.D0)
		if err != nil {
			log.Println("error gettid", err)
			return
		}
		copy(cli.PublicKey[:], b64pk[:32])
		cli.Name = s.GetRandomName(fnvhash(cli.ID), "stan")
		m.D0 = cli.Name
		cli.TKID = s.GetRandomName(fnvhash(cli.ID), "ken")
		m.D1 = cli.TKID
		m.Type = "tid"
		wsjson.Write(ctx, cli.Ws, m)
	case "gettssid":
		cli.Name = s.GetRandomName(fnvhash(cli.ID), "stan")
		m.Type = "tssid"
		m.D0 = cli.Name
		wsjson.Write(ctx, cli.Ws, m)
	case "gettksid":
		m.Type = "tksid"
		m.D0 = s.GetRandomName(fnvhash(cli.ID), "ken")
		cli.TKID = m.D0
		wsjson.Write(ctx, cli.Ws, m)
	case "ping":
		m.Type = "pong"
		wsjson.Write(ctx, cli.Ws, m)
	case "pub":
		//
		//wsjson.Write(ctx, cli.Ws, m)
	case "refuse":
		wsjson.Write(ctx, cli.Ws, m)
	case "message":
		log.Println("message not used")
	case "getkey":
		s.SendKey(cli, &m)
	case "getalias":
		cli.GetAlias(m.D0) //XXX probably not true
	case "getpeers":
		s.SendPeer(cli)
	case "offer":
		s.FwdToTarget(&m)
	case "answer":
		s.FwdToTarget(&m)
	default:
		log.Println(m.Type)
	}
}

func runMined(certDir string, plainHTTP bool, port int) {
	s.InitMinediveServer()
	s.Dispatch = minediveDispatch
	portString := fmt.Sprintf(":%d", port)
	hs := &http.Server{
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		ReadHeaderTimeout: 20 * time.Second,
		Addr:              portString,
		Handler:           &s.ServeMux,
	}
	var err error
	if plainHTTP == true {
		err = hs.ListenAndServe()
	} else {
		err = hs.ListenAndServeTLS(certDir+"cert.pem", certDir+"privkey.pem")
	}
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}

func main() {
	certDir := flag.String("d", "", "Certificate and Key directory")
	plainHTTP := flag.Bool("plain-http", false, "Explic fallback on plain HTTP")
	port := flag.Int("port", 6501, "Listen port")
	flag.Parse()

	runMined(*certDir, *plainHTTP, *port)
}
