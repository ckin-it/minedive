package minedive

import (
	"context"
	crand "crypto/rand"
	b64 "encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/nacl/secretbox"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

type Client struct {
	url     string
	c       *websocket.Conn
	L1Peers map[string]*L1Peer
	L2Peers map[string]*L2Peer
	pMu     *sync.RWMutex
	pL2Mu   *sync.RWMutex
	tid     string
	reauth  string
	Verbose bool
	PrivK   [32]byte
	PK      [32]byte
}

func (cli *Client) CreateOffer(target string, alias string) {
	cli.newL1Peer(target, alias, true)
}

func (cli *Client) AcceptOffer(target string, sdp string) {
	var p *L1Peer
	var ok bool
	//p := cli.newL1Peer(target, alias, false)
	p, ok = cli.GetPeer(target)
	if !ok {
		log.Println(cli.tid, "Peer", target, "not found when accepting offer")
		p = cli.newL1Peer(target, "XXX", false)
	}

	//log.Println("XXX ACCEPTING OFFER")
	//log.Println(sdp)
	desc := webrtc.SessionDescription{}
	desc.SDP = sdp
	desc.Type = webrtc.SDPTypeOffer
	p.pc.SetRemoteDescription(desc)

	answer, err := p.pc.CreateAnswer(nil)
	assertSuccess(err)
	err = p.pc.SetLocalDescription(answer)
	assertSuccess(err)
	<-p.gatherComplete
}

func (cli *Client) AcceptAnswer(target, sdp string) {
	//log.Println("accepting answer")
	p, ok := cli.GetPeer(target)
	if ok != true {
		//log.Println("peer", target, "NOT FOUND")
		return
	}
	if p.pc.SignalingState() == webrtc.SignalingStateHaveLocalOffer {
		desc := webrtc.SessionDescription{}
		desc.SDP = sdp
		desc.Type = webrtc.SDPTypeAnswer
		err := p.pc.SetRemoteDescription(desc)
		if err != nil {
			log.Fatal(cli.tid, target, err)
		}
		<-p.dataChanOpen
	} else {
		log.Println("AcceptAnswer SKIPPED BECAUSE", cli.tid, target, p.pc.SignalingState())
	}
	return
}

func newClient(ctx context.Context, url string) (*Client, error) {
	transport := http.Transport{}
	httpClient := http.Client{Transport: &transport}
	opts := websocket.DialOptions{HTTPClient: &httpClient}
	opts.Subprotocols = append(opts.Subprotocols, "json")
	c, _, err := websocket.Dial(ctx, url, &opts)
	if err != nil {
		return nil, err
	}

	cli := &Client{
		url: url,
		c:   c,
	}
	cli.L1Peers = make(map[string]*L1Peer)
	cli.L2Peers = make(map[string]*L2Peer)
	cli.pMu = &sync.RWMutex{}
	cli.pL2Mu = &sync.RWMutex{}

	return cli, nil
}

func (c *Client) DeletePeer(name string) {
	c.pMu.Lock()
	delete(c.L1Peers, name)
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

func (c *Client) SearchL2(q string) {
	m := L2Msg{
		Type:  "search",
		Query: q,
		Lang:  "en-US",
	}
	b, err := json.Marshal(m)
	if err != nil {
		log.Println("SearchL2", err)
	}
	c.pL2Mu.RLock()
	for _, l2 := range c.L2Peers {
		if l2.GW.dc.ReadyState() == webrtc.DataChannelStateOpen {
			c.SendL2(l2, b)
		} else {
			fmt.Println(c.tid, "GW readyState is:", l2.GW.dc.ReadyState().String())
		}
	}
	c.pL2Mu.RUnlock()

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

func (c *Client) AddL2Peer(l2 *L2Peer) {
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
	fmt.Println(c.tid, "L2", name, "not present yet")
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

	return l2, false
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

func (c *Client) HandleL2Msg(gw *L1Peer, data []byte) {
	m := FwdMsg{}
	json.Unmarshal(data, &m)
	l2, ok := c.GetL2Peer(m.Ori, gw, true)
	if !ok {
		fmt.Println("[IGNORED]", c.tid, "L2 msg from", m.Ori)
		return
	}
	b, ok := c.DecodeL2Msg(l2, m.Msg)
	if !ok {
		return
	}
	dm := L2Msg{}
	json.Unmarshal(b, &dm)
	switch dm.Type {
	case "search":
		fmt.Println("search", dm)
		m := L2Msg{
			Type:  "resp",
			Query: dm.Query,
			//Text:  []string{dm.Query, "prova1", "prova2", "prova3"},
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
		for _, s := range dm.Text {
			fmt.Println(s)
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

func assertSuccess(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func JSONSuccessSend(cl *Client, in Cell) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	err := wsjson.Write(ctx, cl.c, in)
	assertSuccess(err)
	cancel()
}

func JSONSuccessExchange(cl *Client, in Cell, out *Cell) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	err := wsjson.Write(ctx, cl.c, in)
	assertSuccess(err)
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
	assertSuccess(err) //XXX debug
	if len(a) <= 0 {
		return
	}
	err = json.Unmarshal(a, &user)
	assertSuccess(err) //XXX debug
	if cli.PeerIsPresent(user.D0) {
		return
	}
	if cell.D0 != "1" {
		//log.Println("I AM NOT THE INITIATOR")
		cli.newL1Peer(user.D0, user.D1, false)
		return
	}
	//log.Println("CREATE user:", user.D0, "alias:", user.D1)
	go cli.CreateOffer(user.D0, user.D1)
}

func (cli *Client) ws_loop() {
	var aaa Cell
	var ctx context.Context
	var cancel context.CancelFunc

	for {
		ctx, cancel = context.WithTimeout(context.Background(), time.Second*300)
		err := wsjson.Read(ctx, cli.c, &aaa)
		//log.Println(aaa)
		if err == nil {
			switch aaa.Type {
			case "userlist":
				go cli.userlistHandler(&aaa)
			case "pong":
				//log.Println("pong")
			case "offer":
				//D0 target D1 D2 sdp
				go cli.AcceptOffer(aaa.D0, aaa.D2)
				//log.Println(p.pc)
				//log.Println(p.dc)
				//log.Println("p.dc.ReadyState:", p.dc.ReadyState().String())
			case "answer":
				go cli.AcceptAnswer(aaa.D0, aaa.D2)
				//log.Println(p.pc)
				//log.Println(p.dc)
				//log.Println("p.dc.ReadyState:", p.dc.ReadyState().String())
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
		}
		cancel()
	}
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
		assertSuccess(err)
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

func (cli *Client) SingleCmd(cmd string) {
	var in Cell
	in.Type = cmd
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	err := wsjson.Write(ctx, cli.c, in)
	assertSuccess(err)
	cancel()
}

func Dial(addr string) *Client {
	senderPublicKey, senderPrivateKey, err := box.GenerateKey(crand.Reader)
	if err != nil {
		fmt.Println("Dial: unable to generate keys")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	cli, err := newClient(ctx, addr)
	if err != nil {
		log.Fatal(err)
	}
	copy(cli.PrivK[:], senderPrivateKey[:32])
	copy(cli.PK[:], senderPublicKey[:32])
	d0 := b64.StdEncoding.EncodeToString(cli.PK[:32])

	cancel()

	var out Cell
	in := Cell{Type: "gettid", D0: d0, D1: "mid-only"}
	JSONSuccessExchange(cli, in, &out)
	//log.Println(out)
	cli.tid = out.D0
	fmt.Println("tid:", cli.tid)
	//tkid := out.D1

	go cli.ws_loop()

	return cli
}
