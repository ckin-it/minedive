package main

import (
	"context"
	b64 "encoding/base64"
	"flag"
	"fmt"
	"hash/fnv"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/ckin-it/minedive/minedive"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
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

func fnvhash(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

func makeAvoidMap(n string, a string) map[string]bool {
	avoid := make(map[string]bool)
	avoid[n] = true
	for _, s := range strings.Split(a, ",") {
		avoid[s] = true
	}
	return avoid
}

func minediveDispatch(cli *minedive.MinediveClient, m minedive.Cell) {
	if m.Type != "ping" {
		log.Println("New msg:", m.Type, cli.RemoteAddr)
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
		if len(b64pk) == 32 {
			copy(cli.PublicKey[:], b64pk[:32])
		} else {
			log.Printf("error gettid: wrong key len[%d]\n", len(b64pk))
			return
		}
		cli.Name = minedive.GetRandomName(fnvhash(cli.ID), "stan")
		attrs := strings.Split(m.D1, ",")
		for _, attr := range attrs {
			switch attr {
			case "e":
				fmt.Printf("New Exit node: %s\n", cli.Name)
				cli.Exit = true
				s.AddExit(cli)
			case "g":
				fmt.Printf("New Guard node: %s\n", cli.Name)
				cli.Guard = true
				s.AddGuard(cli)
			}
		}
		m.D0 = cli.Name
		cli.TKID = minedive.GetRandomName(fnvhash(cli.ID), "ken")
		m.D1 = cli.TKID
		m.Type = "tid"
		wsjson.Write(ctx, cli.Ws, m)
	case "gettssid":
		cli.Name = minedive.GetRandomName(fnvhash(cli.ID), "stan")
		m.Type = "tssid"
		m.D0 = cli.Name
		wsjson.Write(ctx, cli.Ws, m)
	case "gettksid":
		m.Type = "tksid"
		m.D0 = minedive.GetRandomName(fnvhash(cli.ID), "ken")
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
	case "pubk":
		k, err := b64.StdEncoding.DecodeString(m.D0)
		if err != nil {
			log.Println(err)
			return
		}
		copy(cli.PubK[:], k)
	case "getk":
		oc, err := s.GetClientByName(m.D0)
		if err != nil {
			log.Println(err)
		} else {
			var out minedive.Cell
			out.Type = "k"
			out.D0 = oc.Name
			out.D1 = b64.StdEncoding.EncodeToString(oc.PubK[:])
			wsjson.Write(ctx, cli.Ws, out)
		}
	case "getkey":
		s.SendKey(cli, &m)
	case "getalias":
		cli.GetAlias(m.D0) //XXX probably not true
	case "getpeers":
		s.SendPeer(cli)
	case "getexit":
		avoid := makeAvoidMap(cli.Name, m.D0)
		fmt.Println(avoid)
		nc, err := s.GetExit(avoid)
		if err != nil {
			return
		}
		if nc != nil {
			m.Type = "exit"
			m.D0 = nc.Name
			wsjson.Write(ctx, cli.Ws, m)
		}
	case "getguard":
		fmt.Println("getguard from", cli.Name)
		avoid := makeAvoidMap(cli.Name, m.D0)
		fmt.Println(avoid)
		nc, err := s.GetGuard(avoid)
		if err != nil {
			return
		}
		if nc != nil {
			m.Type = "guard"
			m.D0 = nc.Name
			wsjson.Write(ctx, cli.Ws, m)
		}
	case "getbridge":
		fmt.Println("getguard from", cli.Name)
		avoid := makeAvoidMap(cli.Name, m.D0)
		fmt.Println(avoid)
		nc, err := s.GetGuard(avoid)
		if err != nil {
			return
		}
		if nc != nil {
			m.Type = "bridge"
			m.D0 = nc.Name
			wsjson.Write(ctx, cli.Ws, m)
		}
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
	mux := mux.NewRouter()
	mux.HandleFunc("/ws", s.MinediveAccept)
	//mux.PathPrefix("").Handler(http.FileServer(http.Dir("static/")))
	mux.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/search.html")
	})
	loggedRouter := handlers.CustomLoggingHandler(os.Stdout, mux, ProxyFormatter)

	hs := &http.Server{
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		ReadHeaderTimeout: 20 * time.Second,
		Addr:              portString,
		Handler:           loggedRouter,
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
	plainHTTP := flag.Bool("plain-http", true, "Explicit fallback on plain HTTP")
	port := flag.Int("port", 6501, "Listen port")
	flag.Parse()

	runMined(*certDir, *plainHTTP, *port)
}
