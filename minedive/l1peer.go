package minedive

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/pion/webrtc/v3"
)

var rtcConfig = webrtc.Configuration{
	ICEServers: []webrtc.ICEServer{
		{
			URLs: []string{"stun:stun.l.google.com:19302"},
		},
	},
}

type L1Peer struct {
	Name           string
	Alias          string
	pc             *webrtc.PeerConnection
	dc             *webrtc.DataChannel
	rtcConfig      webrtc.Configuration
	c              *Client
	gatherComplete chan struct{}
	dataChanOpen   chan struct{}
}

func L1peerPing(cli *Client, p *L1Peer) {
	a := pingL1Msg{Type: "ping", From: cli.tid}
	for {
		now := time.Now()
		a.Text = fmt.Sprintf("%d", now.UnixNano())
		b, err := json.Marshal(a)
		assertSuccess(err)
		err = p.dc.SendText(string(b))
		if err != nil {
			log.Println("SendText", err)
			return
		}
		time.Sleep(30 * time.Second)
	}
}

// newL1Peer is the creator
func (cli *Client) newL1Peer(name string, alias string, initiator bool) (p *L1Peer) {
	var dcLabel string
	iceFinished := false
	p = new(L1Peer)
	p.Name = name
	p.Alias = alias
	//p.rtcConfig = rtcConfig
	pc, err := webrtc.NewPeerConnection(rtcConfig)
	assertSuccess(err)
	p.gatherComplete = make(chan struct{})
	p.dataChanOpen = make(chan struct{})

	//_true := true
	_true := false
	opts := webrtc.DataChannelInit{
		Negotiated: &_true,
	}
	if initiator {
		dcLabel = cli.tid + p.Name
	} else {
		dcLabel = p.Name + cli.tid
	}

	//pc.OnICEGatheringStateChange(func(state webrtc.ICEGathererState) {
	//log.Println(state)
	//})

	pc.OnICECandidate(func(ice *webrtc.ICECandidate) {
		log.Println("ICE", ice)
		//nil candidate means we collected all candidates
		if ice == nil {
			if initiator {
				if !iceFinished {
					iceFinished = true
					close(p.gatherComplete)
				}
			} else {
				if !iceFinished {
					iceFinished = true
					log.Println("responder ice finished, closing channel", cli.tid, name)
					sdp := pc.LocalDescription().SDP
					in := Cell{D0: cli.tid, D1: name, D2: sdp}
					//log.Println("XXX ANSWER:", sdp)
					in.Type = "answer"
					JSONSuccessSend(cli, in)
					close(p.gatherComplete)
				}
			}
		}
	})

	dc, err := pc.CreateDataChannel(dcLabel, &opts)
	if err != nil {
		log.Println(err)
	}
	p.dc = dc
	// Register channel opening handling
	dc.OnOpen(func() {
		//log.Println("Data channel ", dc.Label(), dc.ID(), "open. DO SOMETHING")
		dc.OnClose(func() {
			fmt.Println("DC closed with", p.Name)
			delete(cli.L1Peers, p.Name)
		})
		// Register text message handling
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			//fmt.Printf("Message from DataChannel '%s': '%s'\n", dc.Label(), string(msg.Data))
			m := minL1Msg{}
			json.Unmarshal(msg.Data, &m)
			switch m.Type {
			case "ping":
				pingMsg := pingL1Msg{}
				json.Unmarshal(msg.Data, &pingMsg)
				pingMsg.Type = "pong"
				pingMsg.From = cli.tid
				b, err := json.Marshal(pingMsg)
				assertSuccess(err)
				err = p.dc.SendText(string(b))
				if err != nil {
					log.Println("onMessage Ping:", err)
				}
			case "pong":
				m := pingL1Msg{}
				json.Unmarshal(msg.Data, &m)
				//XXX store the result in the peer
				if cli.Verbose {
					now := time.Now()
					thenInt, _ := strconv.ParseInt(m.Text, 10, 64)
					then := time.Unix(0, thenInt)
					fmt.Println("ping", m.From, now.Sub(then))
				}
			case "getl2":
				//fmt.Println(cli.tid, "receiving getl2 from", m.From)
				r := cli.GetOtherPeers(m.From)
				if len(r) == 0 {
					//fmt.Println(cli.tid, "not sending l2 to", m.From)
					return
				}
				//fmt.Println(cli.tid, "sending (", len(r), ") l2 to", m.From)
				//fmt.Println(cli.tid, "sending ", r)
				m := L2L1Msg{
					Type: "l2",
					From: cli.tid,
					L2:   r,
					I:    0,
				}
				//XXX should I and could I trigger ask key?
				b, err := json.Marshal(m)
				assertSuccess(err)
				//fmt.Println("L2 msg sending:", m)
				err = p.dc.SendText(string(b))
				assertSuccess(err) //XXX debug
			case "fwd":
				m := FwdMsg{}
				json.Unmarshal(msg.Data, &m)
				if m.To == cli.tid {
					cli.HandleL2Msg(p, msg.Data)
					return
				}
				next, ok := cli.GetPeerByAlias(m.To)
				if !ok {
					fmt.Println("next not found")
					return
				}
				//fmt.Println(cli.tid, "next found:", next.Name, "ori:", p.Name)
				m.From = cli.tid
				m.Ori = p.Alias
				m.To = next.Name
				b, err := json.Marshal(m)
				assertSuccess(err)
				next.dc.SendText(string(b))
			case "l2":
				var m L2L1Msg
				var askKey bool
				err := json.Unmarshal(msg.Data, &m)
				if err != nil {
					fmt.Println("L2 msg unmarshal err:", err, string(msg.Data))
				}
				if m.I == 1 {
					askKey = true
				}
				for _, s := range m.L2 {
					_ = askKey
					cli.GetL2Peer(s, p, true) //XXX ignored askKey
				}
			default:
				fmt.Println("msg", m.Type, "not implemented")
			}
		})
		go L1peerPing(cli, p)
	})

	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		//fmt.Printf("%s PC State has changed: %s\n", p.Name, s.String())

		if s == webrtc.PeerConnectionStateFailed {
			// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
			// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
			// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
			cli.DeletePeer(p.Name)
			fmt.Println("Peer Connection has gone to failed exiting")
			//os.Exit(0)
		}
	})

	pc.OnSignalingStateChange(func(ss webrtc.SignalingState) {
		//log.Println("signaling state:", cli.tid, p.Name, ss)
	})
	pc.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		//log.Println(p.Name, "ICE Connection State has changed:", connectionState.String())
	})

	// Register data channel creation handling
	pc.OnDataChannel(func(d *webrtc.DataChannel) {
		//log.Println("New DataChannel:", d.Label(), d.ID())
		close(p.dataChanOpen)
		p.dc = d
	})

	if initiator {
		//XXX transform offer in Cell
		offer, err := pc.CreateOffer(nil)
		if err != nil {
			log.Fatal(err)
		}
		err = pc.SetLocalDescription(offer) //XXX THIS STARTS ICE
		if err != nil {
			log.Fatal(err)
		}
		<-p.gatherComplete
		cell := Cell{}
		cell.Type = offer.Type.String()
		cell.D0 = cli.tid //me, from
		cell.D1 = name    //you, target
		cell.D2 = offer.SDP
		JSONSuccessSend(cli, cell)
		assertSuccess(err)
	}
	p.pc = pc
	cli.AddPeer(p)

	return p
}
