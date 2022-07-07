package main

import (
	"context"
	b64 "encoding/base64"
	"encoding/json"
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
	"github.com/go-redis/redis"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"nhooyr.io/websocket/wsjson"
)

var s minedive.MinediveServer
var rdb *redis.Client
var rcmd redis.Cmdable
var port int
var dataSource string

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

func printMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	fmt.Printf("Alloc = %v Mb", bToMb(m.Alloc))
	fmt.Printf("\tTotalAlloc = %v Mb", bToMb(m.TotalAlloc))
	fmt.Printf("\tSys = %v Mb", bToMb(m.Sys))
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
		if s != "" {
			avoid[s] = true
		}
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
		copy(cli.PublicKey[:], b64pk[:32])
		rcmd.SAdd("K_"+cli.Name, m.D0)
		cli.Name = minedive.GetRandomName(fnvhash(cli.ID), "stan")
		attrs := strings.Split(m.D1, ",")
		for _, attr := range attrs {
			switch attr {
			case "e":
				fmt.Printf("New Exit node: %s\n", cli.Name)
				cli.Exit = true
				s.AddExit(cli)
				log.Println("Adding member to EXITS")
				rcmd.SAdd("EXITS", cli.Name)
			case "g":
				fmt.Printf("New Guard node: %s\n", cli.Name)
				cli.Guard = true
				s.AddGuard(cli)
				log.Println("Adding member to GUARDS")
				rcmd.SAdd("GUARDS", cli.Name)
			default:
				//XXX community
			}
		}
		m.D0 = cli.Name
		cli.TKID = minedive.GetRandomName(fnvhash(cli.ID), "ken")
		m.D1 = cli.TKID
		m.Type = "tid"
		wsjson.Write(ctx, cli.Ws, m)
	case "ping":
		m.Type = "pong"
		wsjson.Write(ctx, cli.Ws, m)
	case "pubk":
		k, err := b64.StdEncoding.DecodeString(m.D0)
		if err != nil {
			log.Println(err)
			return
		}
		copy(cli.PubK[:], k)
		rcmd.Set("K_"+cli.Name, m.D0, 1*time.Hour)
	case "getk":
		k := rcmd.Get("K_" + m.D0).Val()
		if k != "" {
			var out minedive.Cell
			out.Type = "k"
			out.D0 = m.D0
			out.D1 = k
			wsjson.Write(ctx, cli.Ws, out)
		} else {
			log.Printf("key for %s not found\n", m.D0)
		}
	case "getexit":
		log.Printf("getexit?\n")
		avoid := makeAvoidMap(cli.Name, m.D0)
		r := rcmd.SRandMemberN("EXITS", int64(len(avoid)+1))
		log.Println(r.Result())
		nc := r.Val()
		log.Println("EXITS:", nc)
		log.Println("AVOID EXITS:", avoid)
		if len(avoid) > len(nc) {
			log.Printf("avoid (%d) >= exits (%d)\n", len(avoid), len(nc))
			return
		}
		i := 0
		for i = range nc {
			_, ok := avoid[nc[i]]
			if !ok {
				log.Println(nc[i], "is not in avoid list")
				break
			} else {
				log.Println(nc[i], "is in avoid list")
			}
		}
		m.Type = "exit"
		m.D0 = nc[i]
		wsjson.Write(ctx, cli.Ws, m)
	case "getguard":
		fmt.Println("getguard from", cli.Name)
		avoid := makeAvoidMap(cli.Name, m.D0)
		nc := rcmd.SRandMemberN("GUARDS", int64(len(avoid)+1)).Val()
		if len(avoid) >= len(nc) {
			return
		}
		i := 0
		for i = range nc {
			_, ok := avoid[nc[i]]
			if !ok {
				break
			}
		}
		m.Type = "guard"
		m.D0 = nc[i]
		wsjson.Write(ctx, cli.Ws, m)
	case "getbridge":
		fmt.Println("getbridge from", cli.Name)
		avoid := makeAvoidMap(cli.Name, m.D0)
		nc := rcmd.SRandMemberN("GUARDS", int64(len(avoid)+1)).Val()
		if len(avoid) >= len(nc) {
			return
		}
		i := 0
		for i = range nc {
			_, ok := avoid[nc[i]]
			if !ok {
				break
			}
		}
		m.Type = "bridge"
		m.D0 = nc[i]
		wsjson.Write(ctx, cli.Ws, m)
	case "offer":
		s.FwdToTarget(&m)
	case "answer":
		s.FwdToTarget(&m)
	default:
		log.Println(m.Type)
	}
}

func restExit(w http.ResponseWriter, r *http.Request) {
	o := strings.Join(s.GetExits(), "\n")
	w.Write([]byte(o))
}

func restGuard(w http.ResponseWriter, r *http.Request) {
	o := strings.Join(s.GetGuards(), "\n")
	w.Write([]byte(o))
}

type BridgeResponse struct {
	Hosts   []string `json:"hosts"`
	Purpose string   `json:"purpose"`
}

func IPHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var val []string
	vars := mux.Vars(r)

	w.Header().Add("Content-Type", "application/json")
	val, err = rcmd.SMembers("IP_" + vars["ip"]).Result()
	if err != nil {
		log.Println("ERR", err)
	}
	w.WriteHeader(http.StatusOK)
	br := &BridgeResponse{}
	for _, v := range val {
		switch v[0] {
		case 'H':
			br.Hosts = append(br.Hosts, v[1:])
		}
	}
	json.NewEncoder(w).Encode(br.Hosts)
}

func runMined(port int) {
	dsn := strings.Split(dataSource, "://")
	switch dsn[0] {
	case "redis":
		rdb = redis.NewClient(&redis.Options{
			Addr:     dsn[1],
			Password: "", // no password set
			DB:       0,  // use default DB
		})
		rcmd = rdb
		a := rcmd.Time().String()
		log.Println("redis time:", a)
		rcmd.Del("EXITS")
		rcmd.Del("GUARDS")
		defer rdb.Close()
	default:
		log.Fatalf("%s not implemented\n", dsn[0])
	}

	s.InitMinediveServer()
	s.Dispatch = minediveDispatch
	mux := mux.NewRouter()
	mux.HandleFunc("/ws", s.MinediveAccept)
	mux.HandleFunc("/exit", restExit)
	mux.HandleFunc("/guard", restGuard)
	mux.HandleFunc("/bridge", s.MinediveAccept)
	mux.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/search.html")
	})
	loggedRouter := handlers.CustomLoggingHandler(os.Stdout, mux, ProxyFormatter)

	hs := &http.Server{
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		ReadHeaderTimeout: 20 * time.Second,
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           loggedRouter,
	}
	var err error
	err = hs.ListenAndServe()
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
	rcmd.Del("EXITS")
	rcmd.Del("GUARDS")
	//XXX clean redis or reboot it
}

func init() {
	flag.IntVar(&port, "port", 6501, "Listen port")
	flag.StringVar(&dataSource, "source", "redis://127.0.0.1:6379", "source selection (full DSN, only redis supported)")
}

func main() {
	flag.Parse()
	runMined(port)
}
