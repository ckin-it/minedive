package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/ckin-it/minedive/minedive"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

var s minedive.MinediveServer
var port int

func minediveDispatch(cli *minedive.MinediveClient, m minedive.Cell) {
	switch m.Type {
	case "sub":
		cli.Name = m.D0
	case "offer":
		s.FwdToTarget(&m)
	case "answer":
		s.FwdToTarget(&m)
	default:
		log.Println(m.Type)
	}
}

func runMined(port int) {
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
}

func main() {
	flag.Parse()
	runMined(port)
}
