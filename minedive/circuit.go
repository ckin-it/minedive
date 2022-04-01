package minedive

import "github.com/pion/webrtc/v3"

type Circuit struct {
	DC       *webrtc.DataChannel
	KeyStack [][32]byte
	State    int
	Length   int
}

func Build(c *Circuit) {

}
