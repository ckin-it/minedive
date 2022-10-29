package main

import (
	"bufio"
	b64 "encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	_ "net/http/pprof"
	"os"
	"strings"
	"time"

	"github.com/ckin-it/minedive/minedive"
)

var lineChan chan string
var bootstrap string
var mineClient *minedive.Client
var avoid []string

func init() {
	lineChan = make(chan string)
	flag.StringVar(&bootstrap, "bootstrap", "ws://localhost:6501/ws", "bootstrapserver")
}

//cli, encMsg.Key, m.Query, m.Lang, encMsg.Key, encMsg.Nonce
func s(m *minedive.Client, peer string, q string, lang string, key string, nonce string) {
	msg := minedive.L2Msg{
		Type:  "respv2",
		Query: q,
		Text:  []string{"I AM NOT A BROWSER", "https://example.com", "http://example.com/", "https://example.com/", "https://nlnet.nl/", "https://quitelongexampleurlusefulfortesting.com/sdjioasjg/asjpdaihj90arfau0094/shdga9ohfgaufhahoigdfa/"},
	}
	b, err := json.Marshal(msg)
	if err != nil {
		log.Println("HandleL2 Resp Marshal:", err)
		return
	}
	_ = b
	m.ReplyCircuit(string(b), key, nonce)
}

func r(c *minedive.Client, a string) {
	var res minedive.L2Msg
	json.Unmarshal([]byte(a), &res)
	for _, v := range res.Text {
		fmt.Println(">", v)
	}
}

func cmdDispatcher(c chan<- string) {
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			c <- fmt.Sprintf("err %s", err)
		} else {
			c <- line
		}
	}
}

func cmdLoop(c <-chan string) {
	for {
		fmt.Print("> ")
		select {
		case line := <-c:
			line = strings.Trim(line, " \n")
			cmd := strings.Split(line, " ")
			if len(cmd[0]) > 0 {
				switch cmd[0] {
				case "err":
					fmt.Println(cmd[1:])
				case "avoid":
					avoid = append(avoid, cmd[1])
				case "init":
					mineClient = minedive.Dial(bootstrap)
					mineClient.Verbose = true
					for mineClient.GetNPeers() < 2 {
						mineClient.SingleCmd("getpeers")
						time.Sleep(3 * time.Second)
						if len(avoid) > 0 {
							for _, p := range avoid {
								mineClient.DeletePeer(p)
							}
						}
					}
					mineClient.Verbose = false
				case "search":
					lang := "en_US"
					if mineClient == nil {
						return
					}
					if len(cmd) > 2 {
						lang = cmd[2]
					}
					mineClient.SearchL2(cmd[1], lang)
				case "verbose":
					if mineClient != nil {
						mineClient.Verbose = true
					}
				case "silence":
					if mineClient != nil {
						mineClient.Verbose = false
					}
				case "join":
					mineClient = minedive.Dial(bootstrap)
					fmt.Println(b64.StdEncoding.EncodeToString(mineClient.PK[:]))
					mineClient.Searcher = s
					var cell minedive.Cell
					cell.Type = "pubk"
					cell.D0 = b64.StdEncoding.EncodeToString(mineClient.PK[:])
					minedive.JSONSuccessSend(mineClient, cell)
					go mineClient.KeepAlive(20 * time.Second)
				case "wsping":
					mineClient.SingleCmd("ping")
				case "cmd":
					if cmd[1] != "" {
						mineClient.SingleCmd(cmd[1])
					}
				case "getk":
					if cmd[1] != "" {
						var cell minedive.Cell
						cell.Type = "getk"
						cell.D0 = cmd[1]
						minedive.JSONSuccessSend(mineClient, cell)
					}
				case "getspd":
					if len(cmd) > 1 {
						mineClient = minedive.Dial(bootstrap)
						mineClient.Responder = r
						fmt.Println(b64.StdEncoding.EncodeToString(mineClient.PK[:]))
						p := mineClient.NewL1Peer(cmd[1], "", true, false)
						time.Sleep(3 * time.Second)
						fmt.Println(p.SDP)
					} else {
						fmt.Println("no")
					}
				case "wsraw":
					if cmd[1] != "" {
						mineClient.JCell(cmd[1])
					}
				case "circuit":
					fmt.Println("circuit invoked with cmd:", len(cmd))
					if len(cmd) == 4 {
						var err error
						circuit, err := mineClient.NewCircuit()
						if err != nil {
							log.Println(err)
						} else {
							fmt.Println("invoking setup circuit")
							circuit.SetupCircuit(cmd[1], cmd[2], cmd[3])
						}
					}
				case "testcirc":
					mineClient = minedive.Dial(bootstrap)
					mineClient.Responder = r
					fmt.Println(b64.StdEncoding.EncodeToString(mineClient.PK[:]))
					_, err := mineClient.NewCircuit()
					if err != nil {
						log.Println(err)
					}
					time.Sleep(1 * time.Second)
					mineClient.Circuits[0].Send("{\"type\":\"test\"}")
					go mineClient.KeepAlive(20 * time.Second)

				case "testcirc2":
					mineClient.Circuits[0].Send("{\"type\":\"test\"}")
				case "searchv2":
					if len(cmd) == 2 {
						msg := fmt.Sprintf("{\"type\":\"search\",\"q\":\"%s\",\"lang\":\"it\"}", cmd[1])
						mineClient.Circuits[0].Send(msg)
					}
				case "raw":
					if cmd[1] != "" && cmd[2] != "" {
						p, ok := mineClient.GetPeer(cmd[1])
						if ok {
							p.Msg(cmd[2])
						}
					}
				case "list":
					mineClient.ListL1Peers()
				case "getpeers":
					mineClient.SingleCmd("getpeers")
				case "quit":
					os.Exit(1)
				default:
				}
			}
		}
	}
}

func main() {
	flag.Parse()
	go func() {
		//log.Println(http.ListenAndServe("localhost:6600", nil))
	}()
	go cmdDispatcher(lineChan)
	cmdLoop(lineChan)
}
