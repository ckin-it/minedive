//go:build js && wasm
// +build js,wasm

package main

import (
	b64 "encoding/base64"
	"encoding/json"
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
	msg := minedive.L2Msg{}
	//name := args[0]
	key := args[1].String()
	nonce := args[2].String()
	//log.Println("replyL2 INVOKED towards", name.Type(), name.String())
	for i, a := range args {
		log.Println(i, a)
		switch a.Type() {
		case js.TypeObject:
			q := a.Get("q")
			//log.Println(q)
			msg.Query = q.String()
			text := a.Get("text")
			for j := 0; j < text.Length(); j++ {
				//log.Println(text.Index(j))
				ti := text.Index(j)
				msg.Text = append(msg.Text, ti.String())
			}
		default:
			log.Println("not an object", a)
		}
	}
	msg.Type = "respv2"
	b, err := json.Marshal(msg)
	if err != nil {
		log.Println(err)
		return err
	}
	//log.Println("Invoke ReplayCircuit with CircuitID", key)
	m.ReplyCircuit(string(b), key, nonce)
	return nil
	// p, ok := m.GetL2PeerIfExists(name.String())
	// if !ok {
	// 	fmt.Println(name.String(), "do not exist")
	// 	return nil
	// }
	// b, err := json.Marshal(msg)
	// if err != nil {
	// 	log.Println("ReplyL2 WASM:", err)
	// }
	// err = m.SendL2(p, b)
	//return err
}

var jsSearch js.Value
var jsRespond js.Value

func r(c *minedive.Client, a string) {
	//log.Println("jsRespond Invoked with", a)
	jsRespond.Invoke(a)
}

func s(c *minedive.Client, l2 string, q string, lang string, key string, nonce string) {
	//log.Println("search", q, lang)
	jsSearch.Invoke(l2, q, lang, key, nonce)
}

func MinediveNewCircuit(bootstrap string, nL1 int, nL2 int) {
	_, err := m.NewCircuit()
	if err != nil {
		log.Println(err)
	}
}

func ExportedMinediveNewCircuit(this js.Value, args []js.Value) interface{} {
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
	log.Println("MinediveNewCircuit invoked with", bootstrap)
	go MinediveNewCircuit(bootstrap, nL1, nL2)
	return nil
}

func MinediveConnect(bootstrap string, nL1 int, nL2 int) {
	m = minedive.Dial(bootstrap)
	m.Searcher = s
	m.Responder = r
	var cell minedive.Cell
	cell.Type = "pubk"
	cell.D0 = b64.StdEncoding.EncodeToString(m.PK[:])
	minedive.JSONSuccessSend(m, cell)

	_, err := m.NewCircuit()
	if err != nil {
		log.Println(err)
	}
	time.Sleep(1 * time.Second)

	go m.KeepAlive(30 * time.Second)

	// go func() {
	// 	for m.GetNPeers() < nL1 {
	// 		m.SingleCmd("getpeers")
	// 		time.Sleep(3 * time.Second)
	// 	}
	// 	for m.GetNL2Peers() < nL2 {
	// 		m.AskL2()
	// 		time.Sleep(3 * time.Second)
	// 	}
	// }()
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
	argQ := args[0]
	argLang := args[1]
	q := argQ.String()
	lang := argLang.String()
	log.Println("MinediveSearch invoked with", q)
	go m.SearchL2(q, lang)
	return nil
}

func ExportedMinediveGetNL1(this js.Value, args []js.Value) interface{} {
	//return js.ValueOf(m.GetNPeers()) //XXX fix this
	return js.ValueOf(m.GetNPeers())
}
func ExportedMinediveGetNL2(this js.Value, args []js.Value) interface{} {
	return js.ValueOf(m.Circuits[0].State)
}

func main() {
	log.Println("go main invoked")
	js.Global().Set("replyL2", js.FuncOf(ReplyL2))
	js.Global().Set("MinediveConnect", js.FuncOf(ExportedMinediveConnect))
	js.Global().Set("MinediveNewCircuit", js.FuncOf(ExportedMinediveNewCircuit))
	js.Global().Set("MinediveReConnect", js.FuncOf(ExportedMinediveReConnect))
	js.Global().Set("MinediveSearch", js.FuncOf(ExportedMinediveSearch))
	js.Global().Set("MinediveGetNL1", js.FuncOf(ExportedMinediveGetNL1))
	js.Global().Set("MinediveGetNL2", js.FuncOf(ExportedMinediveGetNL2))
	//XXX register multiple
	jsSearch = js.Global().Get("search")
	jsRespond = js.Global().Get("respond")
	<-make(chan bool)
}
