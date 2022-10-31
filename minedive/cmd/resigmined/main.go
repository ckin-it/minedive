package main

import (
	b64 "encoding/base64"
	"encoding/json"
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
)

var s minedive.MinediveServer
var port int
var rdb *redis.Client
var rcmd redis.Cmdable
var dataSource string
var queueName string

func signalingDispatch() {
	for {
		r := rcmd.BRPop(300*time.Millisecond, "Q_"+queueName)
		for _, a := range r.Val() {
			var m minedive.Cell
			err := json.Unmarshal([]byte(a), &m)
			if err != nil {
				log.Println(err)
			} else {
				s.FwdToTarget(&m)
			}
		}
	}
}

func minediveDispatch(cli *minedive.MinediveClient, m minedive.Cell) {
	switch m.Type {
	case "sub":
		cli.Name = m.D0
	case "offer":
		if m.D3 == queueName || m.D3 == "" {
			s.FwdToTarget(&m)
		} else {
			b, err := json.Marshal(m)
			if err != nil {
				log.Println("remarshal failed")
			}
			rcmd.LPush("Q_"+queueName, b64.StdEncoding.EncodeToString(b))
		}
	case "answer":
		if m.D3 == queueName || m.D3 == "" {
			s.FwdToTarget(&m)
		} else {
			b, err := json.Marshal(m)
			if err != nil {
				log.Println("remarshal failed")
			}
			rcmd.LPush("Q_"+queueName, b)
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
	mux.HandleFunc("/ws", s.MinediveAccept)
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
	flag.IntVar(&port, "port", 6501, "Listen port")
	flag.StringVar(&dataSource, "source", "redis://127.0.0.1:6379", "source selection (full DSN, only redis supported)")
	flag.StringVar(&queueName, "q", "ws://localhost:6501/ws", "queue name")
}

func main() {
	flag.Parse()
	runMined(port)
}
