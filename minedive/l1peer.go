package minedive

import (
	"bytes"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/pion/webrtc/v3"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/nacl/secretbox"
)

var rtcConfig = webrtc.Configuration{
	ICEServers: []webrtc.ICEServer{
		{URLs: []string{"stun:stun.l.google.com:19302"}},
		//{URLs: []string{"stun:meet-jit-si-turnrelay.jitsi.net:443"}},
		// {
		// 	URLs:       []string{"turn:openrelay.metered.ca:80"},
		// 	Username:   "openrelayproject",
		// 	Credential: "openrelayproject",
		// },
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
	ConnStateCh    chan webrtc.PeerConnectionState
	ConnState      webrtc.PeerConnectionState
	Exit           bool
	K              [32]byte
	SDP            string
}

func (p *L1Peer) Msg(msg string) error {
	err := p.dc.SendText(msg)
	return err
}

func L1peerPing(cli *Client, p *L1Peer) {
	a := pingL1Msg{Type: "ping", From: cli.tid}
	for {
		now := time.Now()
		a.Text = fmt.Sprintf("%d", now.UnixNano())
		b, err := json.Marshal(a)
		if err != nil {
			log.Println("L1peerPing marshal failed:", err)
			return
		}
		err = p.dc.SendText(string(b))
		if err != nil {
			log.Println("SendText", err)
			return
		}
		time.Sleep(30 * time.Second)
	}
}

func (cli *Client) EncL1Msg(p *L1Peer, msg []byte) {
	m := encL1Msg{}
	m.Type = "enc"
	//m.CT = encrypt(msg)
	b, err := json.Marshal(m)
	if err != nil {
		log.Println(err)
		return
	}
	//box.Precompute(&l2.K, &l2.PK, &cli.PrivK)
	p.dc.SendText(string(b))
	return
}

func (cli *Client) DecL1Msg(p *L1Peer, msg webrtc.DataChannelMessage) {
	m := encL1Msg{}
	json.Unmarshal(msg.Data, &m)
	switch m.Type {
	case "encrep":
		cli.routeMu.RLock()
		tgt, ok := cli.Routes[m.TPK]
		cli.routeMu.RUnlock()
		log.Printf("%s LOOKING FOR ROUTE %s (%v)\n", cli.tid, m.TPK, ok)
		if ok {
			tgt.dc.SendText(string(msg.Data))
			return
		}
		log.Println("Response received!")
		var circuit *Circuit
		for _, c := range cli.Circuits {
			if c.CircuitID == m.TPK {
				log.Println("Right circuit found!")
				circuit = c
				break
			}
		}
		if circuit != nil {
			m2 := &webrtc.DataChannelMessage{}
			nonce, err := b64.StdEncoding.DecodeString(m.Nonce)
			if err != nil {
				log.Println(err)
				return
			}
			var nonce2 [24]byte
			copy(nonce2[:], nonce[:24])
			ct, err := b64.StdEncoding.DecodeString(m.CT)
			if err != nil {
				log.Println(err)
				return
			}
			decrypted, ok := secretbox.Open(nil, ct, &nonce2, &circuit.Exit.TKey)
			if !ok {
				log.Println("Decryption failed")
				return
			}
			m2.Data = decrypted
			cli.handleL1Msg(p, *m2, nil)
		}
	case "enc":
		_, ok := cli.Routes[m.Key]
		if !ok {
			log.Println(cli.tid, "ADDING ROUTE FOR", m.Key, "TO", p.Name)
			cli.routeMu.Lock()
			cli.Routes[m.Key] = p
			cli.routeMu.Unlock()
		}
		//log.Println(m)
		//log.Println("Decoding message! 2")
		tpk, err := b64.StdEncoding.DecodeString(m.TPK)
		if err != nil {
			log.Println(err)
			return
		}
		if bytes.Compare(tpk, cli.PK[:]) != 0 {
			log.Printf("target key [%s] not found, discarded", m.TPK)
			return
		}
		nonce, err := b64.StdEncoding.DecodeString(m.Nonce)
		if err != nil {
			log.Println(err)
			return
		}
		var nonce2 [24]byte
		copy(nonce2[:], nonce[:24])
		ct, err := b64.StdEncoding.DecodeString(m.CT)
		if err != nil {
			log.Println(err)
			return
		}
		var k [32]byte
		tmpk, err := b64.StdEncoding.DecodeString(m.Key)
		copy(k[:], tmpk[:32])
		log.Println("circuit key:", b64.StdEncoding.EncodeToString(k[:]))
		box.Precompute(&k, &k, &cli.PrivK)
		decrypted, ok := secretbox.Open(nil, ct, &nonce2, &k)
		if !ok {
			log.Println("Decryption failed")
			return
		}
		m2 := &webrtc.DataChannelMessage{}
		//XXX m2.IsString = false
		m2.Data = decrypted
		log.Println("Message decoded, going to handle")
		cli.handleL1Msg(p, *m2, &m)
	default:
		cli.handleL1Msg(p, msg, nil)
	}
}

func (cli *Client) handleL1Msg(p *L1Peer, msg webrtc.DataChannelMessage, encMsg *encL1Msg) {
	m := minL1Msg{}
	json.Unmarshal(msg.Data, &m)
	//fmt.Printf("MSG [%s] FROM [%s]\n", m.Type, p.Name)
	switch m.Type {
	case "search":
		if encMsg == nil {
			log.Println("direct search, discarded")
			return
		}
		m := L2Msg{}
		json.Unmarshal(msg.Data, &m)

		log.Println(cli.tid, "search as browser?", cli.Browser)
		if cli.Searcher != nil {
			cli.Searcher(cli, encMsg.Key, m.Query, m.Lang, encMsg.Key, encMsg.Nonce)
		}
		b, err := json.Marshal(m)
		if err != nil {
			log.Println(cli.tid, "HandleL2 Resp Marshal:", err)
			return
		}
		_ = b
		//XXX err = c.SendL2(l2, b) dealt inside the search?
		if err != nil {
			log.Println(cli.tid, "HandleL2 Resp SendL2:", err)
			return
		}
	case "respv2":
		m := L2Msg{}
		json.Unmarshal(msg.Data, &m)
		if cli.Responder != nil {
			log.Println("RESPONDER TRIGGERED")
			cli.Responder(cli, string(msg.Data))
		} else {
			log.Println("Responder not implemented")
		}
	case "connect":
		log.Println("CONNECT RECEIVED")
		m := FwdMsg{}
		json.Unmarshal(msg.Data, &m)
		_, ok := cli.GetPeer(m.To)
		if !ok {
			go cli.newL1Peer(m.To, "", true, false)
			return
		}
		log.Printf("connection to %s available, connect discarded\n", m.To)
	case "ping":
		pingMsg := pingL1Msg{}
		json.Unmarshal(msg.Data, &pingMsg)
		pingMsg.Type = "pong"
		pingMsg.From = cli.tid
		b, err := json.Marshal(pingMsg)
		if err != nil {
			log.Println("marshal failed, message discarded", err)
			return
		}
		err = p.dc.SendText(string(b))
		if err != nil {
			log.Println("send failed, message discarded", err)
			log.Println("ready state?", p.dc.ReadyState().String())
			return
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
		if err != nil {
			log.Println("getl2 marshal failed", err)
			return
		}
		err = p.dc.SendText(string(b))
		if err != nil {
			log.Println("getl2 send failed", err)
			return
		}
	case "test":
		log.Println(cli.tid, "TEST MSG RECEIVED")
		cli.ReplyCircuit("{\"type\":\"test2\"}", encMsg.Key, encMsg.Nonce)
	case "test2":
		log.Println(cli.tid, "REPLY TEST MSG RECEIVED")
		//XXX this could be blocking cli.Circuits[0].StateNotification <- "OK"
		cli.Circuits[0].Notification <- "TEST-OK" //XXX fix to map to a specific Circuit
		log.Println(cli.tid, "REPLY TEST MSG PROPAGATED")
		//cli.Circuits[0].State = "OK"
	case "fwd2":
		m := FwdMsg{}
		json.Unmarshal(msg.Data, &m)
		tgt, ok := cli.GetPeer(m.To)
		if ok {
			b, err := b64.StdEncoding.DecodeString(m.Ori)
			if err != nil {
				log.Println(err)
				return
			}
			//log.Printf("Peer %s should get %s\n", m.To, b)
			tgt.dc.SendText(string(b))
		} else {
			log.Printf("Peer %s not found\n", m.To)
		}
		return
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
		if err != nil {
			log.Println("fwd marshal failed", err)
			return
		}
		err = next.dc.SendText(string(b))
		if err != nil {
			log.Println("fwd send failed", err)
			return
		}
	case "l2":
		log.Println("L2 msg received, NEVER RIGHT?")
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
}

// Exporting newL1Peer
func (cli *Client) NewL1Peer(name string, alias string, initiator bool, exit bool) (p *L1Peer) {
	return cli.newL1Peer(name, alias, initiator, exit)
}

// newL1Peer is the creator
func (cli *Client) newL1Peer(name string, alias string, initiator bool, exit bool) (p *L1Peer) {
	log.Println("NEW PEER", name)
	var dcLabel string
	iceFinished := false
	p = new(L1Peer)
	p.Name = name
	p.Alias = alias
	p.Exit = exit
	//p.rtcConfig = rtcConfig
	pc, err := webrtc.NewPeerConnection(rtcConfig)
	if err != nil {
		log.Println("newL1Peer NewPeerConnection failed", err)
		return
	}
	p.gatherComplete = make(chan struct{})
	p.dataChanOpen = make(chan struct{})
	p.ConnStateCh = make(chan webrtc.PeerConnectionState)

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
					//log.Println("responder ice finished, closing gatherChannel", cli.tid, name)
					sdp := pc.LocalDescription().SDP
					in := Cell{D0: cli.tid, D1: name, D2: sdp}
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
		//XXX THIS WILL NOTIFY the client
		ev := peerConnEvent{}
		ev.state = webrtc.PeerConnectionStateConnected
		ev.peer = p.Name
		for _, c := range cli.Circuits {
			if c.Guard.ID == ev.peer {
				c.Notification <- "guard-connected"
			}
		}

		dc.OnClose(func() {
			fmt.Println("DC closed with", p.Name)
			delete(cli.L1Peers, p.Name)
		})
		// Register text message handling
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			//fmt.Printf("Message from DataChannel '%s': '%s'\n", dc.Label(), string(msg.Data))
			//cli.handleL1Msg(p, msg)
			cli.DecL1Msg(p, msg)
		})
		go L1peerPing(cli, p)
	})

	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		//fmt.Printf("%s PC State has changed: %s\n", p.Name, s.String())
		//XXX more granular notification
		//ev := peerConnEvent{}
		//ev.state = s
		//ev.peer = p.Name
		//if s != webrtc.PeerConnectionStateConnected {
		//	cli.peerConnectionNoticeChan <- ev
		//}
		//p.ConnState = s

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
			log.Println("NewL1Peer SetLocalDescription failed", err)
		}
		<-p.gatherComplete //XXX benchmark this
		cell := Cell{}
		cell.Type = offer.Type.String()
		cell.D0 = cli.tid                   //me, from
		cell.D1 = name                      //you, target
		cell.D2 = pc.LocalDescription().SDP //XXX instead of initial offer
		JSONSuccessSend(cli, cell)
	}
	p.pc = pc
	cli.AddPeer(p)
	//XXX cli.Notification <- "new-peer" why notify client when addPeer has been invoked by it?

	return p
}
