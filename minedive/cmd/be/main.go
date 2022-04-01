package main

import (
	"log"
	"strconv"
	"syscall/js"
	"time"

	"github.com/ckin-it/minedive/minedive"
)

var m *minedive.Client

func ReplyL2(this js.Value, args []js.Value) interface{} {
	//name := args[0]
	//results := args[1]
	log.Println("replyL2 INVOKED towards", args[0].String)
	log.Println(this)
	for i, a := range args {
		log.Println(i, a)
		switch a.Type() {
		case js.TypeObject:
			q := a.Get("q")
			log.Println(q)
			text := a.Get("text")
			for j := 0; j < text.Length(); j++ {
				log.Println(text.Index(j))
			}
		default:
			log.Println(a)
		}
	}
	return nil
}

var jsSearch js.Value

func s(l2 string, q string, lang string) {
	log.Println("search", q, lang)
	jsSearch.Invoke(l2, q, lang)
}

func MinediveConnect(bootstrap string, nL1 int, nL2 int) {
	m = minedive.Dial(bootstrap)
	m.Searcher = s
	go m.KeepAlive(200 * time.Second)
	go func() {
		for m.GetNPeers() < nL1 {
			m.SingleCmd("getpeers")
			time.Sleep(3 * time.Second)
		}
		for m.GetNL2Peers() < nL2 {
			m.AskL2()
			time.Sleep(3 * time.Second)
		}
	}()
	select {}
}

func ExportedMinediveConnect(this js.Value, args []js.Value) interface{} {
	a := args[0]
	b := args[1]
	c := args[2]
	bootstrap := a.String()
	nL1, err := strconv.Atoi(string(b.String()))
	if err != nil {
		nL1 = 1
	}
	nL2, err := strconv.Atoi(string(c.String()))
	if err != nil {
		nL2 = 1
	}
	log.Println("MinediveConnect invoked with", bootstrap)
	go MinediveConnect(bootstrap, nL1, nL2)
	return nil
}

func ExportedMinediveReConnect(this js.Value, args []js.Value) interface{} {
	a := args[0]
	b := args[1]
	bootstrap := a.String()
	n, err := strconv.Atoi(string(b.String()))
	if err != nil {
		n = 1
	}
	_ = n
	log.Println("MinediveReConnect invoked with", bootstrap, "[not implemented yet]")
	//go MinediveConnect(bootstrap, n)
	return nil
}

func ExportedMinediveSearch(this js.Value, args []js.Value) interface{} {
	a := args[0]
	b := args[1]
	q := a.String()
	lang := b.String()
	log.Println("MinediveSearch invoked with", q)
	go m.SearchL2(q, lang)
	return nil
}

func ExportedMinediveGetNL1(this js.Value, args []js.Value) interface{} {
	return js.ValueOf(1) //XXX fix this
}
func ExportedMinediveGetNL2(this js.Value, args []js.Value) interface{} {
	return js.ValueOf(m.GetNL2Peers())
}

func main() {
	log.Println("go main invoked")
	js.Global().Set("replyL2", js.FuncOf(ReplyL2))
	js.Global().Set("MinediveConnect", js.FuncOf(ExportedMinediveConnect))
	js.Global().Set("MinediveReConnect", js.FuncOf(ExportedMinediveReConnect))
	js.Global().Set("MinediveSearch", js.FuncOf(ExportedMinediveSearch))
	js.Global().Set("MinediveGetNL1", js.FuncOf(ExportedMinediveGetNL1))
	js.Global().Set("MinediveGetNL2", js.FuncOf(ExportedMinediveGetNL2))
	jsSearch = js.Global().Get("search")
	<-make(chan bool)
}
