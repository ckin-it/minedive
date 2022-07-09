package main

import (
	"context"
	b64 "encoding/base64"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
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

func minediveDispatch(cli *minedive.MinediveClient, m minedive.Cell) {
	if m.Type != "ping" {
		log.Println("New msg:", m.Type, cli.RemoteAddr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	switch m.Type {
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
	default:
		log.Println(m.Type)
	}
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
		defer rdb.Close()
	default:
		log.Fatalf("%s not implemented\n", dsn[0])
	}

	s.InitMinediveServer()
	s.Dispatch = minediveDispatch
	mux := mux.NewRouter()
	mux.HandleFunc("/ws/k", s.MinediveAccept)
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
}

func init() {
	flag.IntVar(&port, "port", 6503, "Listen port")
	flag.StringVar(&dataSource, "source", "redis://127.0.0.1:6379", "source selection (full DSN, only redis supported)")
}

func main() {
	flag.Parse()
	runMined(port)
}
