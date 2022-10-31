package minedive

import (
	"context"
	crand "crypto/rand"
	b64 "encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/nacl/secretbox"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

type Client struct {
	url                      string
	c                        *websocket.Conn
	L1Peers                  map[string]*L1Peer
	L2Peers                  map[string]*L2Peer
	Searcher                 func(*Client, string, string, string, string, string) //l2, q, lang, key, nonce
	Responder                func(*Client, string)                                 //
	pMu                      *sync.RWMutex
	pL2Mu                    *sync.RWMutex
	tid                      string
	reauth                   string
	Verbose                  bool
	PrivK                    [32]byte
	PK                       [32]byte
	Browser                  bool
	kMu                      *sync.RWMutex
	Keys                     map[string][32]byte
	Circuits                 []*Circuit
	Routes                   map[string]*L1Peer
	routeMu                  *sync.RWMutex
	opts                     *DialOptions
	Notification             chan string
	StateNotification        chan MinediveState
	State                    MinediveState
	peerConnectionNoticeChan chan peerConnEvent
}

type peerConnEvent struct {
	state webrtc.PeerConnectionState
	peer  string
}

func (cli *Client) stateHandler() {
	for {
		select {
		case n := <-cli.StateNotification:
			cli.State = n
		}
	}
}

func (cli *Client) CreateOffer(target string, alias string, exit bool) {
	cli.newL1Peer(target, alias, true, exit)
}

func (cli *Client) AcceptOffer(target string, sdp string) {
	var p *L1Peer
	var ok bool
	//p := cli.newL1Peer(target, alias, false)
	p, ok = cli.GetPeer(target)
	if !ok {
		log.Println(cli.tid, "Peer", target, "not found when accepting offer")
		p = cli.newL1Peer(target, "XXX", false, false)
	}

	//log.Println("XXX ACCEPTING OFFER")
	//log.Println(sdp)
	desc := webrtc.SessionDescription{}
	desc.SDP = sdp
	desc.Type = webrtc.SDPTypeOffer
	p.pc.SetRemoteDescription(desc)

	answer, err := p.pc.CreateAnswer(nil)
	if err != nil {
		log.Println("AcceptOffer CreateAnswer failed", err)
	}
	err = p.pc.SetLocalDescription(answer) //XXX
	if err != nil {
		log.Println("AcceptOffer SetLocalDescription failed", err)
	}
	<-p.gatherComplete
}

func (cli *Client) AcceptAnswer(target, sdp string) {
	//log.Println("accepting answer")
	p, ok := cli.GetPeer(target)
	if ok != true {
		//log.Println("peer", target, "NOT FOUND")
		return
	}
	p.SDP = sdp
	if p.pc.SignalingState() == webrtc.SignalingStateHaveLocalOffer {
		desc := webrtc.SessionDescription{}
		desc.SDP = sdp
		desc.Type = webrtc.SDPTypeAnswer
		err := p.pc.SetRemoteDescription(desc)
		if err != nil {
			//log.Fatal(cli.tid, target, err)
			log.Println("XXX", cli.tid, target, err)
		}
		<-p.dataChanOpen
	} else {
		log.Println("AcceptAnswer SKIPPED BECAUSE", cli.tid, target, p.pc.SignalingState())
	}
	return
}

func (cli *Client) WebSocketJSONDial() (*websocket.Conn, error) {
	opts := websocket.DialOptions{}
	opts.Subprotocols = append(opts.Subprotocols, "json")
	c, _, err := websocket.Dial(context.Background(), cli.url, &opts)
	return c, err
}

func (cli *Client) Redial() (err error) {
	cli.c.Close(websocket.StatusAbnormalClosure, "")
	cli.c, err = cli.WebSocketJSONDial()
	if err == nil {
		log.Printf("REDIAL: Connected[%s]\n", cli.opts.D1)
	}
	d0 := b64.StdEncoding.EncodeToString(cli.PK[:32])
	in := Cell{Type: "gettid", D0: d0, D1: cli.opts.D1}
	JSONSuccessSend(cli, in)

	return err
}

func newClient(ctx context.Context, url string) (*Client, error) {
	//transport := http.Transport{}
	//httpClient := http.Client{Transport: &transport}
	cli := &Client{
		url: url,
	}
	cli.Notification = make(chan string)
	c, err := cli.WebSocketJSONDial()
	if err != nil {
		return nil, err
	}

	cli.c = c
	cli.L1Peers = make(map[string]*L1Peer)
	cli.L2Peers = make(map[string]*L2Peer)
	cli.Keys = make(map[string][32]byte)
	cli.pMu = &sync.RWMutex{}
	cli.pL2Mu = &sync.RWMutex{}
	cli.kMu = &sync.RWMutex{}
	cli.Routes = make(map[string]*L1Peer) //XXX mutex?
	cli.routeMu = &sync.RWMutex{}
	cli.State = MinediveStateNew
	go cli.stateHandler()

	return cli, nil
}

func (c *Client) DeletePeer(name string) {
	c.pMu.Lock()
	_, ok := c.L1Peers[name]
	if ok {
		delete(c.L1Peers, name)
	}
	c.pMu.Unlock()
	fmt.Println("peer", name, "deleted")
}

func (c *Client) GetNPeers() int {
	c.pMu.RLock()
	n := len(c.L1Peers)
	c.pMu.RUnlock()
	return n
}

func (c *Client) SendL2(l2 *L2Peer, b []byte) error {
	enc := secretbox.Seal(l2.Ononce[:24], b, &l2.Ononce, &l2.K)
	//fmt.Println(c.tid, "L2 enc with", hex.EncodeToString(l2.K[:32]), "Ononce:", l2.Ononce[23])
	IncNonce(l2.Ononce[:], len(l2.Ononce))
	m := FwdMsg{
		From: c.tid,
		Type: "fwd",
		To:   l2.Name,
		Msg:  b64.StdEncoding.EncodeToString(enc),
	}
	b, err := json.Marshal(m)
	if err != nil {
		log.Println("SendL2 marshal", err)
		return err
	}
	err = l2.GW.dc.SendText(string(b))
	if err != nil {
		log.Println("SendL2 send", err)
	}
	return err
}

func (c *Client) SearchL2(q string, lang string) {
	m := L2Msg{
		Type:  "search",
		Query: q,
		Lang:  lang,
	}
	b, err := json.Marshal(m)
	if err != nil {
		log.Println("search", err)
		return
	}
	if c.Circuits[0].StateOK() {
		c.Circuits[0].Send(string(b))
	}
}

func (c *Client) GetNL2Peers() int {
	c.pL2Mu.RLock()
	n := len(c.L2Peers)
	c.pL2Mu.RUnlock()
	return n
}

func (c *Client) PeerIsPresent(name string) bool {
	c.pMu.RLock()
	_, ok := c.L1Peers[name]
	c.pMu.RUnlock()
	return ok
}

func (c *Client) GetPeer(name string) (*L1Peer, bool) {
	c.pMu.RLock()
	p, ok := c.L1Peers[name]
	c.pMu.RUnlock()
	return p, ok
}

func (c *Client) GetOtherPeers(name string) []string {
	out := []string{}
	c.pMu.RLock()
	for k, v := range c.L1Peers {
		if k != name {
			//fmt.Println(c.tid, "send to", name, "peer", v.Alias, "which is", v.Name)
			out = append(out, v.Alias)
		}
	}
	c.pMu.RUnlock()
	return out
}

func (c *Client) GetL2PeerIfExists(name string) (*L2Peer, bool) {
	c.pL2Mu.RLock()
	defer c.pL2Mu.RUnlock()
	for k, v := range c.L2Peers {
		if k == name {
			return v, true
		}
	}
	return nil, false
}

func (c *Client) DeleteL2Peer(name string) {
	c.pL2Mu.Lock()
	_, ok := c.L2Peers[name]
	if ok {
		delete(c.L2Peers, name)
	}
	c.pL2Mu.Unlock()
}

func (c *Client) AddL2Peer(l2 *L2Peer) {
	fmt.Printf("[ADD L2 PEER][%s] %s\n", c.tid, l2.Name)
	c.pL2Mu.Lock()
	c.L2Peers[l2.Name] = l2
	c.pL2Mu.Unlock()
}

func (c *Client) AskL2() {
	m := minL1Msg{
		From: c.tid,
		Type: "getl2",
	}
	b, err := json.Marshal(m)
	if err != nil {
		log.Println("AskL2 Marshal err:", err)
	}
	c.pMu.RLock()
	for _, l1 := range c.L1Peers {
		l1.dc.SendText(string(b))
	}
	c.pMu.RUnlock()
}

func (c *Client) GetL2Peer(name string, gw *L1Peer, askKey bool) (*L2Peer, bool) {
	var l2 *L2Peer
	l2, ok := c.GetL2PeerIfExists(name)
	if ok {
		if l2.GW == nil {
			l2.GW = gw
		}
		return l2, true
	}
	//fmt.Println(c.tid, "L2", name, "not present yet")
	l2 = &L2Peer{
		Name:       name,
		PK:         [32]byte{},
		GW:         gw,
		State:      "from-msg",
		MsgIgnored: 1,
	}
	c.AddL2Peer(l2)
	if askKey {
		in := Cell{
			Type: "getkey",
			D0:   l2.Name,
			D1:   l2.GW.Name,
		}
		JSONSuccessSend(c, in)
	}
	return l2, true
}

func (c *Client) DecodeL2Msg(l2 *L2Peer, emsg string) ([]byte, bool) {
	encrypted, err := b64.StdEncoding.DecodeString(emsg)
	if err != nil {
		log.Println(c.tid, "unable to DecodeL2Msg base64")
		return nil, false
	}
	var nonce [24]byte
	copy(nonce[:], encrypted[:24])
	decrypted, ok := secretbox.Open(nil, encrypted[24:], &nonce, &l2.K)
	if !ok {
		fmt.Println(c.tid, "DecodeL2Msg Failed (Nonce", nonce[23], l2.MsgIgnored, ")", hex.EncodeToString(l2.K[:32]))
		return nil, false
	}
	//fmt.Println("[DECODED]", c.tid, string(decrypted))
	return decrypted, true
}

func isZero(a []byte, n int) bool {
	return false
}

func (c *Client) HandleL2Msg(gw *L1Peer, data []byte) {
	m := FwdMsg{}
	json.Unmarshal(data, &m)
	l2, ok := c.GetL2Peer(m.Ori, gw, true)
	if !ok {
		fmt.Printf("[MISSING L2 PEER][%s] %s\n", c.tid, m.Ori)
		return
	} else {
		fmt.Printf("[PRESENT L2 PEER][%s] %s\n", c.tid, m.Ori)
	}
	//fmt.Printf("[%s] KEY [%b]\n", c.tid, l2.K)
	failure := 0
RETRY:
	//XXX check if PK? K?
	b, ok := c.DecodeL2Msg(l2, m.Msg)
	if !ok {
		failure++
		if failure < 3 {
			fmt.Printf("[%s] wait for key\n", c.tid)
			time.Sleep(3 * time.Second)
			goto RETRY
		}
		return
	}
	dm := L2Msg{}
	json.Unmarshal(b, &dm)
	switch dm.Type {
	case "fwd":
		fmt.Println("fwd", dm)
		//m := L2Msg{}
	case "search":
		fmt.Println("search", dm)
		log.Println(c.tid, "search as browser?", c.Browser)
		m := L2Msg{
			Type:  "resp",
			Query: dm.Query,
			//Text:  []string{dm.Query, "prova1", "prova2", "prova3"},
		}
		if c.Searcher != nil {
			c.Searcher(c, l2.Name, dm.Query, dm.Lang, "", "")
		}
		b, err := json.Marshal(m)
		if err != nil {
			log.Println(c.tid, "HandleL2 Resp Marshal:", err)
			return
		}
		_ = b
		//err = c.SendL2(l2, b)
		if err != nil {
			log.Println(c.tid, "HandleL2 Resp SendL2:", err)
			return
		}
	case "resp":
		if c.Responder != nil {
			c.Responder(c, string(b))
		} else {
			log.Println("Responder not implemented")
		}
		log.Println(dm)
		for _, s := range dm.Text {
			fmt.Println("XXX", s)
		}
	}
}

func (c *Client) GetPeerByAlias(alias string) (*L1Peer, bool) {
	c.pMu.RLock()
	defer c.pMu.RUnlock()
	for _, v := range c.L1Peers {
		if v.Alias == alias {
			//fmt.Println("[OK] GET PEERS BY ALIAS", c.tid, v.Name, v.Alias)
			return v, true
		}
		//fmt.Println("[NOT] GET PEERS BY ALIAS", c.tid, v.Name, v.Alias)
	}
	return nil, false
}

func (c *Client) ListL1Peers() {
	if c != nil {
		c.pMu.RLock()
		for k := range c.L1Peers {
			fmt.Println(k)
		}
		c.pMu.RUnlock()
	}
}

func (c *Client) ListL2Peers() {
	if c != nil {
		c.pL2Mu.RLock()
		for k := range c.L2Peers {
			fmt.Println(k)
		}
		c.pL2Mu.RUnlock()
	}
}

func (c *Client) AddPeer(p *L1Peer) bool {
	name := p.Name
	c.pMu.Lock()
	_, ok := c.L1Peers[name]
	if ok == false {
		c.L1Peers[name] = p
	}
	c.pMu.Unlock()
	//fmt.Println("peer", name, "added")
	return !ok
}

func JSONSuccessSend(cl *Client, in Cell) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	err := wsjson.Write(ctx, cl.c, in)
	if err != nil {
		log.Println("JSON Send failed", err)
	}
	cancel()
}

func JSONSuccessExchange(cl *Client, in Cell, out *Cell) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	err := wsjson.Write(ctx, cl.c, in)
	if err != nil {
		log.Println("JSON Exchange failed", err)
		cancel()
		return
	}
	cancel()
	ctx, cancel = context.WithTimeout(context.Background(), time.Second*30)
	err = wsjson.Read(ctx, cl.c, out)
	if err != nil {
		log.Println(err)
		//log.Println(*out)
	}
	cancel()
}

func (cli *Client) userlistHandler(cell *Cell) {
	var user Cell
	//log.Println(cell)
	a, err := b64.StdEncoding.DecodeString(cell.D1)
	if err != nil {
		log.Println("userlistHandler DecodeString failed", err)
		return
	}
	if len(a) <= 0 {
		return
	}
	err = json.Unmarshal(a, &user)
	if err != nil {
		log.Println("userlistHandler Unmarshal failed", err)
		return
	}
	if cli.PeerIsPresent(user.D0) {
		return
	}
	exit := false
	//log.Println("I AM NOT THE INITIATOR")
	if user.D2 == "e" {
		exit = true
	}
	if cell.D0 != "1" {
		cli.newL1Peer(user.D0, user.D1, false, exit)
		return
	}
	//log.Println("CREATE user:", user.D0, "alias:", user.D1)
	go cli.CreateOffer(user.D0, user.D1, exit)
}

func (cli *Client) ws_loop() {
	var aaa Cell
	var ctx context.Context
	var cancel context.CancelFunc

	for {
	RETRY:
		ctx, cancel = context.WithTimeout(context.Background(), time.Second*300)
		err := wsjson.Read(ctx, cli.c, &aaa)
		//log.Println(aaa)
		if err == nil {
			cancel()
			switch aaa.Type {
			case "userlist":
				go cli.userlistHandler(&aaa)
			case "pong":
			//log.Println("pong")
			case "guard": //receiving a guard peer?
				log.Println("Guard:", aaa.D0)
				go cli.newL1Peer(aaa.D0, "", true, false)
				for _, c := range cli.Circuits {
					if c.Guard.ID == "" {
						c.Guard.ID = aaa.D0
						c.Notification <- "gotguard"
					}
				}
			case "bridge":
				log.Println("Bridge:", aaa.D0)
				for _, c := range cli.Circuits {
					if c.Bridge.ID == "" {
						c.Bridge.ID = aaa.D0
						c.Notification <- "gotbridge"
					}
				}
			case "exit":
				log.Println("Exit:", aaa.D0)
				for _, c := range cli.Circuits {
					if c.Exit.ID == "" {
						c.Exit.ID = aaa.D0
						c.Notification <- "gotexit"
					}
				}
			case "offer":
				fmt.Printf("Offer from %s received\n", aaa.D0)
				//D0 target D1 D2 sdp
				go cli.AcceptOffer(aaa.D0, aaa.D2)
			case "answer":
				fmt.Printf("Answer from %s received\n", aaa.D0)
				go cli.AcceptAnswer(aaa.D0, aaa.D2)
			case "k":
				k, err := b64.StdEncoding.DecodeString(aaa.D1)
				if err == nil {
					var k32 [32]byte
					copy(k32[:], k)
					cli.kMu.Lock()
					cli.Keys[aaa.D0] = k32
					cli.kMu.Unlock()
					for _, c := range cli.Circuits {
						if c.Guard.ID == aaa.D0 || c.Exit.ID == aaa.D0 || c.Bridge.ID == aaa.D0 {
							c.Notification <- "gotkey"
						}
					}
				}
			case "key":
				l2, _ := cli.GetL2Peer(aaa.D0, nil, false)
				b, err := b64.StdEncoding.DecodeString(aaa.D1)
				if err != nil {
					log.Println("Impossible to decode key for", aaa.D0)
				}
				copy(l2.PK[:], b)
				box.Precompute(&l2.K, &l2.PK, &cli.PrivK)
				for l2.MsgIgnored > 0 {
					IncNonce(l2.Ononce[:], len(l2.Ononce))
					l2.MsgIgnored--
				}
			default:
				fmt.Println(aaa.Type, "WS msg not implemented")
			}
		} else {
			cancel()
			cs := websocket.CloseStatus(err)
			switch cs {
			case websocket.StatusGoingAway:
				log.Println("WS Status (GoingAway) REDIAL AGAIN")
				time.Sleep(10 * time.Second)
				err := cli.Redial()
				if err != nil {
					log.Println(err)
				}
				time.Sleep(10 * time.Second)
				goto RETRY
			case websocket.StatusAbnormalClosure:
				log.Println("WS Status (AbnormalClosure) REDIAL AGAIN")
				time.Sleep(10 * time.Second)
				err := cli.Redial()
				if err != nil {
					log.Println(err)
				}
				time.Sleep(10 * time.Second)
				goto RETRY
			case -1:
				log.Println("WS Status (-1) REDIAL AGAIN")
				time.Sleep(10 * time.Second)
				err := cli.Redial()
				if err != nil {
					log.Println(err)
				}
				time.Sleep(10 * time.Second)
				goto RETRY
			default:
				log.Printf("ERR on WS READ[%v] %v\n", cs, err)
				goto RETRY
			}
		}
		cancel()
	}
}

func (cli *Client) SinglePing() {
	var in Cell
	cli.SingleCmd("ping")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	err := wsjson.Write(ctx, cli.c, in)
	if err != nil {
		log.Println("Client Ping write failed", err)
		//XXX fix peer
	}
	cancel()
}

func (cli *Client) ping() {
	var in Cell
	//activePeers = 4
	for {
		if cli.GetNPeers() > 3 {
			cli.SingleCmd("ping")
		} else {
			cli.SingleCmd("getpeers")
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		err := wsjson.Write(ctx, cli.c, in)
		if err != nil {
			log.Println("Client Ping write failed", err)
			//XXX fix peer
		}
		cancel()
		time.Sleep(10 * time.Second)
	}
}

func (cli *Client) KeepAlive(interval time.Duration) {
	for {
		cli.SingleCmd("ping")
		time.Sleep(interval)
	}
}

func (cli *Client) JCell(cell string) {
	var in Cell
	err := json.Unmarshal([]byte(cell), &in)
	if err != nil {
		log.Println("JCell Unmarshall failed", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	err = wsjson.Write(ctx, cli.c, in)
	cancel()
	if err != nil {
		log.Println("JCell Write failed", err)
		return
	}
}

func (cli *Client) SingleCmd(cmd string) {
	var in Cell
	in.Type = cmd
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	err := wsjson.Write(ctx, cli.c, in)
	if err != nil {
		log.Println("SingleCmd Write failed", err)
	}
	cancel()
}

func Dial(addr string) *Client {
	senderPublicKey, senderPrivateKey, err := box.GenerateKey(crand.Reader)
	if err != nil {
		fmt.Println("Dial: unable to generate keys")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	cli, err := newClient(ctx, addr)
	cancel()
	cli.Browser = true
	if err != nil {
		log.Fatal(err)
	}
	copy(cli.PrivK[:], senderPrivateKey[:32])
	copy(cli.PK[:], senderPublicKey[:32])
	d0 := b64.StdEncoding.EncodeToString(cli.PK[:32])
	cli.opts = &DialOptions{
		D0: d0,
		D1: "e,g",
	}

	var out Cell
	in := Cell{Type: "gettid", D0: d0, D1: cli.opts.D1}
	JSONSuccessExchange(cli, in, &out)
	//log.Println(out)
	cli.tid = out.D0
	fmt.Println("tid:", cli.tid)
	//tkid := out.D1

	go cli.ws_loop()

	return cli
}

func DefaultDial(addr string) *Client {
	opts := &DialOptions{
		D1: "e,g",
	}
	return DialOpts(addr, opts)
}

type DialOptions struct {
	D0              string
	D1              string
	PublicKey       *[32]byte
	PrivateKey      *[32]byte
	ClientIsBrowser bool
}

func (cli *Client) RedialOpts(add string, opts *DialOptions) {

}

func DialOpts(addr string, opts *DialOptions) *Client {
	var senderPublicKey *[32]byte
	var senderPrivateKey *[32]byte
	var err error
	var d0 string

	if opts.PublicKey == nil || opts.PrivateKey == nil {
		senderPublicKey, senderPrivateKey, err = box.GenerateKey(crand.Reader)
		if err != nil {
			//XXX wait and try again actually
			log.Panic("Dial: unable to generate keys", err)
		}
	} else {
		senderPublicKey = opts.PublicKey
		senderPrivateKey = opts.PrivateKey
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	cli, err := newClient(ctx, addr)
	cli.opts = opts
	cli.Browser = true
	if err != nil {
		log.Fatal(err)
	}
	copy(cli.PrivK[:], senderPrivateKey[:32])
	copy(cli.PK[:], senderPublicKey[:32])
	if opts.D0 == "" {
		d0 = b64.StdEncoding.EncodeToString(cli.PK[:32])
	} else {
		d0 = opts.D0
	}

	cancel()

	var out Cell
	in := Cell{Type: "gettid", D0: d0, D1: opts.D1}
	JSONSuccessExchange(cli, in, &out)
	cli.tid = out.D0
	fmt.Println("tid:", cli.tid)

	go cli.ws_loop()

	return cli
}

func DialMiddle(addr string) *Client {
	senderPublicKey, senderPrivateKey, err := box.GenerateKey(crand.Reader)
	if err != nil {
		fmt.Println("Dial: unable to generate keys")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	cli, err := newClient(ctx, addr)
	cancel()
	cli.Browser = false
	if err != nil {
		log.Fatal(err)
	}
	copy(cli.PrivK[:], senderPrivateKey[:32])
	copy(cli.PK[:], senderPublicKey[:32])
	d0 := b64.StdEncoding.EncodeToString(cli.PK[:32])
	cli.opts = &DialOptions{
		D0: d0,
		D1: "g,b",
	}

	var out Cell
	in := Cell{Type: "gettid", D0: d0, D1: cli.opts.D1}
	JSONSuccessExchange(cli, in, &out)
	//log.Println(out)
	cli.tid = out.D0
	fmt.Println("tid:", cli.tid)
	//tkid := out.D1

	go cli.ws_loop()

	return cli
}
