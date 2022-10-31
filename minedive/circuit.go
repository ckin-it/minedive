package minedive

import (
	crand "crypto/rand"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/pion/webrtc/v3"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/nacl/secretbox"
)

const (
	GetPeerTimeout      = 30
	GetPeerFailureMax   = 3
	GetKeyTimeout       = 30
	NotificationTimeout = 30
)

type Node struct {
	ID    string
	OKey  [32]byte // own key
	TKey  [32]byte // temporary key
	LKey  [32]byte // long term key
	Nonce [24]byte
	State int
	Peer  *L1Peer
}

type Circuit struct {
	CircuitID    string
	DC           *webrtc.DataChannel
	state        string
	Guard        Node
	Bridge       Node
	Exit         Node
	PubK         [32]byte
	PrivK        [32]byte
	m            *Client
	Notification chan string
}

func (c *Circuit) handleNotification() {
	for {
		select {
		case n := <-c.Notification:
			log.Printf("XXX [HN] circuit %s (%s) %s\n", c.CircuitID, c.state, n)
			switch n {
			case "gotexit":
				go c.SetupStep1()
			case "gotguard":
				go c.SetupStep2()
			case "gotbridge":
				c.SetupStepGetKeys()
			case "gotkey":
				ok := c.CheckKeys()
				if ok {
					if c.state == "guard-connected" {
						go c.ExtendCircuit()
						c.state = "extending"
					} else {
						c.state = "gotallkeys"
					}
				}
			case "guard-connected":
				log.Printf("[HN] circuit %s (%s) GUARD CONNECTED\n", c.CircuitID, c.state)
				p, ok := c.m.GetPeer(c.Guard.ID)
				if ok {
					c.DC = p.dc
					if c.state == "gotallkeys" {
						go c.ExtendCircuit()
						c.state = "extending"
					} else {
						c.state = "guard-connected"
					}
				} else {
					log.Println("[HN] unexplicable peer not available but connected")
				}
			case "quit":
				log.Printf("[HN] circuit %s (%s) EXITING\n", c.CircuitID, c.state)
				return
			case "TEST-OK":
				c.state = "OK"
				log.Printf("[HN] circuit %s (%s) THIS COMPLEX HANDLER COULD EXIT AND LET PLACE TO A FAILURE HANDLER\n", c.CircuitID, c.state)
			default:
				log.Println("[HN]", n)
			}
		case <-time.After(NotificationTimeout * time.Second):
			switch c.state {
			case "OK":
				log.Printf("[HN] circuit %s (%s) KEEP-ALIVE\n", c.CircuitID, c.state)
				//XXX nothing to do
			default:
				c.Send("{\"type\":\"test\"}")
				log.Printf("[HN] circuit %s TIMEOUT: handle state (%s)\n", c.CircuitID, c.state)
			}
		}
	}
}

//needed: nonce, e.Key
func (m *Client) ReplyCircuit(msg string, eKey string, eNonce string) {
	o := encL1Msg{}
	var k [32]byte
	var peerKey [32]byte
	var nonce [24]byte
	//var err error
	p, ok := m.Routes[eKey]
	if !ok {
		log.Println("MISSING ROUTE FOR", eKey)
		return
	}

	o.Type = "encrep"
	o.Key = b64.StdEncoding.EncodeToString(m.PK[:])
	log.Printf("Reply with Key[%s]\n", eKey)
	o.TPK = eKey
	tnonce, err := b64.StdEncoding.DecodeString(eNonce)
	if err != nil {
		log.Println(err)
		return
	}
	IncNonce(tnonce, len(tnonce))
	copy(nonce[:], tnonce[:24])
	o.Nonce = b64.StdEncoding.EncodeToString(nonce[:])
	tpeerKey, err := b64.StdEncoding.DecodeString(eKey)
	if err != nil {
		log.Println(err)
		return
	}
	//XXX moving to go1.17 zerocopy with (*[32]byte)(peerKey)
	copy(peerKey[:], tpeerKey[:32])
	box.Precompute(&k, &peerKey, &m.PrivK)
	encmsg := secretbox.Seal(nil, []byte(msg), &nonce, &k)
	o.CT = b64.StdEncoding.EncodeToString(encmsg[:])
	b, err := json.Marshal(o)
	if err != nil {
		log.Println(err)
		return
	}
	err = p.dc.SendText(string(b))
	if err != nil {
		log.Printf("ReplyCircuit %s failed\n", eKey)
	}
}

func (c *Circuit) Send(msg string) {
	o := encL1Msg{}
	var err error

BEGIN:
	k := b64.StdEncoding.EncodeToString(c.PubK[:])

	o.Type = "enc"
	o.Key = k
	o.TPK = b64.StdEncoding.EncodeToString(c.Exit.OKey[:])
	o.Nonce, err = UseNonce(c.Exit.Nonce[:])
	if err != nil {
		//XXX NEW circuit Key (PubK, PrivK) and refresh Nodes TKey
		//Reset Nonce
		//Go back to beginning
		goto BEGIN
	}
	encmsg := secretbox.Seal(nil, []byte(msg), &c.Exit.Nonce, &c.Exit.TKey)
	o.CT = b64.StdEncoding.EncodeToString(encmsg[:])
	b, err := json.Marshal(o)
	if err != nil {
		log.Println(err)
		return
	}

	fm := &FwdMsg{
		Type: "fwd2",
		To:   c.Exit.ID,
		Ori:  b64.StdEncoding.EncodeToString(b),
	}
	b, err = json.Marshal(fm)
	tmpm := string(b)
	o.Type = "enc"
	o.Key = k
	o.TPK = b64.StdEncoding.EncodeToString(c.Bridge.OKey[:])
	o.Nonce, err = UseNonce(c.Bridge.Nonce[:24])
	if err != nil {
		//XXX NEW circuit Key (PubK, PrivK) and refresh Nodes TKey
		//Reset Nonce
		//Go back to beginning
		goto BEGIN
	}
	encmsg = secretbox.Seal(nil, []byte(tmpm), &c.Bridge.Nonce, &c.Bridge.TKey)
	o.CT = b64.StdEncoding.EncodeToString(encmsg[:])
	b, err = json.Marshal(o)

	fm = &FwdMsg{
		Type: "fwd2",
		To:   c.Bridge.ID,
		Ori:  b64.StdEncoding.EncodeToString(b),
	}
	b, err = json.Marshal(fm)
	tmpm = string(b)
	o.Type = "enc"
	o.Key = k
	o.TPK = b64.StdEncoding.EncodeToString(c.Guard.OKey[:])
	o.Nonce, err = UseNonce(c.Guard.Nonce[:24])
	if err != nil {
		//XXX NEW circuit Key (PubK, PrivK) and refresh Nodes TKey
		//Reset Nonce
		//Go back to beginning
		goto BEGIN
	}
	encmsg = secretbox.Seal(nil, []byte(tmpm), &c.Guard.Nonce, &c.Guard.TKey)
	o.CT = b64.StdEncoding.EncodeToString(encmsg[:])
	b, err = json.Marshal(o)

	c.DC.SendText(string(b))
}

func (m *Client) NewCircuit() (*Circuit, error) {
	c := &Circuit{}
	runtime.SetFinalizer(c, func(c *Circuit) { fmt.Println(c.CircuitID, "IS DEAD XXX") })
	c.Notification = make(chan string)
	c.state = "New"
	go c.handleNotification()

	publicKey, privateKey, err := box.GenerateKey(crand.Reader)
	if err != nil {
		fmt.Println("SetupCircuit: unable to generate keys:", err)
		return nil, err
	}
	copy(c.PrivK[:], privateKey[:32])
	copy(c.PubK[:], publicKey[:32])
	c.m = m
	c.CircuitID = b64.StdEncoding.EncodeToString(publicKey[:])
	log.Println("New Circuit:", c.CircuitID)
	//XXX Mutex around circuits? Just one circuit alive?
	//XXX deal with more circuits?
	//XXX m.Circuits = append(m.Circuits, c)
	if len(m.Circuits) > 0 {
		m.Circuits[0] = c
	} else {
		m.Circuits = append(m.Circuits, c)
	}
	var cell Cell
	cell.Type = "getexit"
	JSONSuccessSend(c.m, cell)
	return c, nil
}

func (c *Circuit) SetupStep1() {
	var cell Cell
	cell.Type = "getguard"
	cell.D0 = c.Exit.ID
	JSONSuccessSend(c.m, cell)
}

func (c *Circuit) SetupStep2() {
	var cell Cell
	cell.Type = "getbridge"
	cell.D0 = fmt.Sprintf("%s,%s", c.Guard.ID, c.Exit.ID)
	JSONSuccessSend(c.m, cell)
}

func (c *Circuit) SetupStepGetKeys() {
	var cell Cell
	cell.Type = "getk"
	cell.D0 = c.Guard.ID
	JSONSuccessSend(c.m, cell)
	cell.D0 = c.Bridge.ID
	JSONSuccessSend(c.m, cell)
	cell.D0 = c.Exit.ID
	JSONSuccessSend(c.m, cell)
}

func (c *Circuit) CheckKeys() bool {
	c.m.kMu.RLock()
	tk1, ok1 := c.m.Keys[c.Guard.ID]
	tk2, ok2 := c.m.Keys[c.Bridge.ID]
	tk3, ok3 := c.m.Keys[c.Exit.ID]
	c.m.kMu.RUnlock()
	if !ok1 || !ok2 || !ok3 {
		return false
	}
	copy(c.Guard.OKey[:32], tk1[:32])
	copy(c.Bridge.OKey[:32], tk2[:32])
	copy(c.Exit.OKey[:32], tk3[:32])
	box.Precompute(&c.Guard.TKey, &c.Guard.OKey, &c.PrivK)
	box.Precompute(&c.Bridge.TKey, &c.Bridge.OKey, &c.PrivK)
	box.Precompute(&c.Exit.TKey, &c.Exit.OKey, &c.PrivK)
	return true
}

func (c *Circuit) SetupCircuit(guard string, bridge string, exit string) {
	o := encL1Msg{}
	var err error
	var cell Cell
	cell.Type = "getk"
	cell.D0 = guard
	JSONSuccessSend(c.m, cell)
	cell.D0 = bridge
	JSONSuccessSend(c.m, cell)
	cell.D0 = exit
	JSONSuccessSend(c.m, cell)

	// 	//LOOP this on failed
	// 	failed := 0
	// GETKEYS:
	// 	time.Sleep(1 * time.Second)
	// 	c.m.kMu.RLock()
	// 	tk, ok1 := c.m.Keys[guard]
	// 	copy(c.Guard.OKey[:32], tk[:32])
	// 	tk, ok2 := c.m.Keys[bridge]
	// 	copy(c.Bridge.OKey[:32], tk[:32])
	// 	tk, ok3 := c.m.Keys[exit]
	// 	copy(c.Exit.OKey[:32], tk[:32])
	// 	c.m.kMu.RUnlock()
	// 	if !ok1 || !ok2 || !ok3 {
	// 		if failed < 30 {
	// 			failed++
	// 			log.Println("Try again get keys")
	// 			goto GETKEYS
	// 		}
	// 		return
	// 	}
	box.Precompute(&c.Guard.TKey, &c.Guard.OKey, &c.PrivK)
	box.Precompute(&c.Bridge.TKey, &c.Bridge.OKey, &c.PrivK)
	box.Precompute(&c.Exit.TKey, &c.Exit.OKey, &c.PrivK)

	guardPeer, ok := c.m.GetPeer(guard)
	if !ok {
		//XXX should connect to
		log.Println("guard not found")
		return
	}
	c.DC = guardPeer.dc
	c.Guard.ID = guard
	m1 := fmt.Sprintf("{\"type\":\"connect\",\"to\":\"%s\"}", bridge)
	o.Type = "enc"
	o.Key = b64.StdEncoding.EncodeToString(c.PubK[:])
	log.Println("circuit key:", o.Key)
	o.TPK = b64.StdEncoding.EncodeToString(c.Guard.OKey[:])
	o.Nonce, err = UseNonce(c.Guard.Nonce[:])
	encmsg := secretbox.Seal(nil, []byte(m1), &c.Guard.Nonce, &c.Guard.TKey)
	o.CT = b64.StdEncoding.EncodeToString(encmsg[:])
	b, err := json.Marshal(o)
	if err != nil {
		log.Println("setupCircuit, Marshal:", err)
		return
	}
	c.DC.SendText(string(b))

	time.Sleep(10 * time.Second)
	c.Bridge.ID = bridge
	m1 = fmt.Sprintf("{\"type\":\"connect\",\"to\":\"%s\"}", exit)
	o.Type = "enc"
	o.Key = b64.StdEncoding.EncodeToString(c.PubK[:])
	o.TPK = b64.StdEncoding.EncodeToString(c.Bridge.OKey[:])
	o.Nonce, err = UseNonce(c.Bridge.Nonce[:])
	encmsg = secretbox.Seal(nil, []byte(m1), &c.Bridge.Nonce, &c.Bridge.TKey)
	o.CT = b64.StdEncoding.EncodeToString(encmsg[:])
	b, err = json.Marshal(o)
	fm := &FwdMsg{
		Type: "fwd2",
		To:   bridge,
		Ori:  b64.StdEncoding.EncodeToString(b),
	}
	b, err = json.Marshal(fm)
	m1 = string(b)
	o.Type = "enc"
	o.Key = b64.StdEncoding.EncodeToString(c.PubK[:])
	o.TPK = b64.StdEncoding.EncodeToString(c.Guard.OKey[:])
	o.Nonce, err = UseNonce(c.Guard.Nonce[:24])
	encmsg = secretbox.Seal(nil, []byte(m1), &c.Guard.Nonce, &c.Guard.TKey)
	o.CT = b64.StdEncoding.EncodeToString(encmsg[:])
	b, err = json.Marshal(o)
	if err != nil {
		log.Println("setupCircuit, Marshal:", err)
		return
	}
	c.DC.SendText(string(b))
	c.Exit.ID = exit
	//guardPeer.Msg(string(b))

	failedWait := 0
	failedSend := 0
	time.Sleep(1 * time.Second) //XXX
SENDTEST:
	c.Send("{\"type\":\"test\"}")
	time.Sleep(1 * time.Second)
WAITTEST:
	switch c.state {
	case "OK":
		log.Println("OK CIRCUIT")
		return
	default:
		log.Printf("Circuit %s State [%s]", c.CircuitID, c.state)
	}
	if failedWait < 2 {
		failedWait++
		time.Sleep(5 * time.Second)
		goto WAITTEST
	}
	if failedSend < 2 {
		log.Println("RESEND")
		failedSend++
		failedWait = 0
		goto SENDTEST
	}

}

func (c *Circuit) ExtendCircuit() {
	var o encL1Msg
	var err error

	log.Printf("EXTEND CIRCUIT TO BRIDGE[%s]\n", c.Bridge.ID)
	m1 := fmt.Sprintf("{\"type\":\"connect\",\"to\":\"%s\"}", c.Bridge.ID)
	o.Type = "enc"
	o.Key = b64.StdEncoding.EncodeToString(c.PubK[:])
	log.Println("circuit key:", o.Key)
	o.TPK = b64.StdEncoding.EncodeToString(c.Guard.OKey[:])
	o.Nonce, err = UseNonce(c.Guard.Nonce[:])
	encmsg := secretbox.Seal(nil, []byte(m1), &c.Guard.Nonce, &c.Guard.TKey)
	o.CT = b64.StdEncoding.EncodeToString(encmsg[:])
	b, err := json.Marshal(o)
	if err != nil {
		log.Println("setupCircuit, Marshal:", err)
		return
	}
	c.DC.SendText(string(b))

	time.Sleep(10 * time.Second)
	log.Printf("EXTEND CIRCUIT TO EXIT[%s]\n", c.Exit.ID)
	m1 = fmt.Sprintf("{\"type\":\"connect\",\"to\":\"%s\"}", c.Exit.ID)
	o.Type = "enc"
	o.Key = b64.StdEncoding.EncodeToString(c.PubK[:])
	o.TPK = b64.StdEncoding.EncodeToString(c.Bridge.OKey[:])
	o.Nonce, err = UseNonce(c.Bridge.Nonce[:])
	encmsg = secretbox.Seal(nil, []byte(m1), &c.Bridge.Nonce, &c.Bridge.TKey)
	o.CT = b64.StdEncoding.EncodeToString(encmsg[:])
	b, err = json.Marshal(o)
	fm := &FwdMsg{
		Type: "fwd2",
		To:   c.Bridge.ID,
		Ori:  b64.StdEncoding.EncodeToString(b),
	}
	b, err = json.Marshal(fm)
	m1 = string(b)
	o.Type = "enc"
	o.Key = b64.StdEncoding.EncodeToString(c.PubK[:])
	o.TPK = b64.StdEncoding.EncodeToString(c.Guard.OKey[:])
	o.Nonce, err = UseNonce(c.Guard.Nonce[:24])
	encmsg = secretbox.Seal(nil, []byte(m1), &c.Guard.Nonce, &c.Guard.TKey)
	o.CT = b64.StdEncoding.EncodeToString(encmsg[:])
	b, err = json.Marshal(o)
	if err != nil {
		log.Println("setupCircuit, Marshal:", err)
		return
	}
	c.DC.SendText(string(b))
	//guardPeer.Msg(string(b))

	failedWait := 0
	failedSend := 0
	time.Sleep(1 * time.Second) //XXX
SENDTEST:
	c.Send("{\"type\":\"test\"}")
	time.Sleep(1 * time.Second)
WAITTEST:
	switch c.state {
	case "OK":
		log.Println("OK CIRCUIT")
		return
	default:
		log.Printf("Circuit %s State [%s]", c.CircuitID, c.state)
	}
	if failedWait < 2 {
		failedWait++
		time.Sleep(5 * time.Second)
		goto WAITTEST
	}
	if failedSend < 2 {
		log.Println("RESEND")
		failedSend++
		failedWait = 0
		goto SENDTEST
	}
}

func (m *Client) BuildCircuitZ() (c *Circuit) {
	var en, gu, br *L1Peer
	tries := 0
	//pick an exit (from bridge next suggested or L1s?)
FIND_EXIT:
	for _, v := range m.L1Peers {
		if v.Exit {
			en = v
			break
		}
	}
	if en.Name == "" {
		if tries < 30 {
			tries++
			time.Sleep(3 * time.Second)
			goto FIND_EXIT
		}
	}
	c.Exit.Peer = en
	c.Exit.ID = en.Name

	//pick a bridge (!exit) from guard next suggested or L1s
	for _, v := range m.L1Peers {
		if v.Name != en.Name {
			br = v
			break
		}
	}
	c.Bridge.Peer = br
	c.Bridge.ID = br.Name

	//pick a guard (!exit !bridge) from established L1s
	for _, v := range m.L1Peers {
		if v.Name != en.Name && v.Name != br.Name {
			gu = v
			break
		}
	}
	c.Guard.Peer = gu
	c.Guard.ID = gu.Name
	return c
}

func (c *Circuit) State() string {
	return c.state
}

func (c *Circuit) StateOK() bool {
	if c.state == "OK" {
		return true
	}
	return false
}
