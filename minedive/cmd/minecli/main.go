package main

import (
	"bufio"
	"flag"
	"fmt"
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
	flag.StringVar(&bootstrap, "bootstrap", "wss://localhost:6501/", "bootstrapserver")
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
					for mineClient.GetNL2Peers() < 2 {
						mineClient.AskL2()
						time.Sleep(3 * time.Second)
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
					go mineClient.KeepAlive(5 * time.Second)
				case "wsping":
					mineClient.SingleCmd("ping")
				case "cmd":
					if cmd[1] != "" {
						mineClient.SingleCmd(cmd[1])
					}
				case "list":
					mineClient.ListL1Peers()
					mineClient.ListL2Peers()
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
	go cmdDispatcher(lineChan)
	cmdLoop(lineChan)
}
