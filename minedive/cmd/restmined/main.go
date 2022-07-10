package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

var rdb *redis.Client
var rcmd redis.Cmdable
var port int
var dataSource string

const RESPONSE_LIMIT = 100

func restExit(w http.ResponseWriter, r *http.Request) {
	o := strings.Join(rcmd.SRandMemberN("EXITS", RESPONSE_LIMIT).Val(), "\n")
	w.Write([]byte(o))
}

func restGuard(w http.ResponseWriter, r *http.Request) {
	o := strings.Join(rcmd.SRandMemberN("GUARDS", RESPONSE_LIMIT).Val(), "\n")
	w.Write([]byte(o))
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

	mux := mux.NewRouter()
	mux.HandleFunc("/exit", restExit)
	mux.HandleFunc("/guard", restGuard)
	mux.HandleFunc("/bridge", restGuard)
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
