package minedive

import (
	"context"
	crand "crypto/rand"
	b64 "encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	json "encoding/json"

	"golang.org/x/crypto/nacl/secretbox"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

type MinediveServer struct {
	clients       []*MinediveClient
	clientsMutex  *sync.Mutex
	nextID        uint64
	idMutex       *sync.Mutex
	ServeMux      http.ServeMux
	Dispatch      func(*MinediveClient, Cell)
	exits         map[string]*MinediveClient
	exitsRWMutex  *sync.RWMutex
	guards        map[string]*MinediveClient
	guardsRWMutex *sync.RWMutex
}

//MinediveClient is the view the has of a connected client
type MinediveClient struct {
	ID         string
	Name       string
	TKID       string
	SecretKey  [32]byte //internally used???
	PublicKey  [32]byte //internally used???
	PubK       [32]byte
	Nonce      [24]byte
	RemoteAddr string
	Ws         *websocket.Conn
	Exit       bool
	Guard      bool
}

//GetAlias return the name a peer is seen behind another peer
func (gw *MinediveClient) GetAlias(username string) (string, error) {
	var alias string
	var err error
	enc := secretbox.Seal(gw.Nonce[:], []byte(username), &gw.Nonce, &gw.SecretKey)
	alias = b64.StdEncoding.EncodeToString(enc)
	return alias, err
}

func (s *MinediveServer) InitMinediveServer() {
	s.clientsMutex = &sync.Mutex{}
	s.idMutex = &sync.Mutex{}
	s.exitsRWMutex = &sync.RWMutex{}
	s.guardsRWMutex = &sync.RWMutex{}
	s.guards = make(map[string]*MinediveClient)
	s.exits = make(map[string]*MinediveClient)
	//s.ServeMux.HandleFunc("/ws", s.minediveAccept)
	log.Println("MinediveServer initialized")
}

func (s *MinediveServer) AddExit(cli *MinediveClient) {
	s.exitsRWMutex.Lock()
	s.exits[cli.Name] = cli
	s.exitsRWMutex.Unlock()
}

func (s *MinediveServer) DelExit(cliName string) {
	s.exitsRWMutex.Lock()
	delete(s.exits, cliName)
	s.exitsRWMutex.Unlock()
}

//XXX recheck for skipped
func (s *MinediveServer) GetExit(avoid map[string]bool) (cli *MinediveClient, err error) {
	cli = nil
	s.exitsRWMutex.RLock()
	skip := rand.Intn(len(s.exits))
	i := 0
	for cn := range s.exits {
		if i > skip {
			_, ok := avoid[cn]
			if ok == false {
				cli = s.exits[cn]
				break
			}
		}
		i++
	}
	i = 0
	for cn := range s.exits {
		if i <= skip {
			_, ok := avoid[cn]
			if ok == false {
				cli = s.exits[cn]
				break
			}
		}
		i++
	}
	s.exitsRWMutex.RUnlock()
	if cli == nil {
		return nil, errors.New("no exits available")
	}
	return cli, nil
}

func (s *MinediveServer) AddGuard(cli *MinediveClient) {
	s.guardsRWMutex.Lock()
	s.guards[cli.Name] = cli
	s.guardsRWMutex.Unlock()
}

func (s *MinediveServer) DelGuard(cliName string) {
	s.guardsRWMutex.Lock()
	delete(s.guards, cliName)
	s.guardsRWMutex.Unlock()
}

func (s *MinediveServer) GetExits() []string {
	s.exitsRWMutex.RLock()
	o := make([]string, len(s.exits))
	i := 0
	for k := range s.exits {
		o[i] = k
		i++
	}
	s.exitsRWMutex.RUnlock()
	return o
}

func (s *MinediveServer) GetGuards() []string {
	s.guardsRWMutex.RLock()
	o := make([]string, len(s.guards))
	i := 0
	for k := range s.guards {
		o[i] = k
		i++
	}
	s.guardsRWMutex.RUnlock()
	return o
}

func (s *MinediveServer) GetGuard(avoid map[string]bool) (cli *MinediveClient, err error) {
	cli = nil
	s.guardsRWMutex.RLock()
	skip := rand.Intn(len(s.guards))
	i := 0
	for cn := range s.guards {
		if i > skip {
			_, ok := avoid[cn]
			if ok == false {
				cli = s.guards[cn]
				break
			}
		}
		i++
	}
	i = 0
	for cn := range s.guards {
		if i <= skip {
			_, ok := avoid[cn]
			if ok == false {
				cli = s.guards[cn]
				break
			}
		}
		i++
	}
	s.guardsRWMutex.RUnlock()
	if cli == nil {
		return nil, errors.New("no guards available")
	}
	return cli, nil
}

func (s *MinediveServer) MinediveAccept(w http.ResponseWriter, r *http.Request) {
	var remoteAddr string
	host := r.Header.Get("X-Real-IP")
	if host != "" {
		port := strings.Split(r.RemoteAddr, ":")
		if len(port) == 2 {
			remoteAddr = fmt.Sprintf("%s:%s", host, port[1])
		} else {
			remoteAddr = fmt.Sprintf("%s(%s)", host, r.RemoteAddr)
		}
	} else {
		remoteAddr = r.RemoteAddr
	}
	log.Printf("minediveAccept invoked from %s\n", remoteAddr)
	opts := websocket.AcceptOptions{}
	opts.InsecureSkipVerify = true
	opts.Subprotocols = append(opts.Subprotocols, "json")
	//opts.OriginPatters
	//log.Println("subproto", opts.Subprotocols)
	ws, err := websocket.Accept(w, r, &opts)
	if err != nil {
		log.Println(err)
		return
	}
	defer ws.Close(websocket.StatusGoingAway, "")

	var cli MinediveClient
	s.idMutex.Lock()
	cli.ID = fmt.Sprintf("%d", s.nextID)
	s.nextID++
	s.idMutex.Unlock()
	cli.Ws = ws
	cli.RemoteAddr = remoteAddr
	if _, err := io.ReadFull(crand.Reader, cli.SecretKey[:]); err != nil {
		log.Println(err)
		websocket.CloseStatus(err)
	}
	s.clientsMutex.Lock()
	s.clients = append(s.clients, &cli)
	s.clientsMutex.Unlock()
	for {
		var jmsg Cell
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		err := wsjson.Read(ctx, ws, &jmsg)
		if err != nil {
			status := websocket.CloseStatus(err)
			//log.Println("err", err, "status", status)
			if status == -1 {
				//log.Println("READ ERROR", err)
			}
			s.DeleteClientByName(cli.Name)
			cli.Ws.Close(websocket.StatusAbnormalClosure, "") //not really needed but...
			log.Printf("%s %s disconnected (%d)\n", cli.ID, cli.Name, status)
			cancel()
			return
		}
		s.Dispatch(&cli, jmsg)
		cancel()
	}
}

func (s *MinediveServer) DeleteClientByName(name string) error {
	var c *MinediveClient
	s.clientsMutex.Lock()
	len := len(s.clients)
	for n := range s.clients {
		c = s.clients[n]
		if c.Name == name {
			s.clients[n] = s.clients[len-1]
			s.clients[len-1] = nil
			s.clients = s.clients[:len-1]
			s.clientsMutex.Unlock()
			if c.Exit {
				s.DelExit(c.Name)
			}
			if c.Guard {
				s.DelGuard(c.Name)
			}
			return nil
		}
	}
	s.clientsMutex.Unlock()
	return errors.New("Client not found")
}

func (s *MinediveServer) GetClientByName(name string) (*MinediveClient, error) {
	var c *MinediveClient
	s.clientsMutex.Lock()
	for n := range s.clients {
		c = s.clients[n]
		if c.Name == name {
			s.clientsMutex.Unlock()
			return c, nil
		}
	}
	s.clientsMutex.Unlock()
	return nil, errors.New("Client not found")
}

func (s *MinediveServer) dumpClients() {
	s.clientsMutex.Lock()
	if len(s.clients) == 0 {
		log.Println("dump clients: empty")
	}
	for n := range s.clients {
		log.Println("dump clients", n, s.clients[n].Name)
	}
	s.clientsMutex.Unlock()
}

func (s *MinediveServer) GetOtherPeer(cli *MinediveClient) (*MinediveClient, error) {
	s.clientsMutex.Lock()
	if len(s.clients) > 1 {
		i := rand.Intn(len(s.clients))
		c := s.clients[i]
		s.clientsMutex.Unlock()
		if c == cli {
			return cli, errors.New("getOtherPeer: same peer")
		}
		return c, nil
	}
	s.clientsMutex.Unlock()
	return cli, errors.New("getOtherPeer: no peers")
}

func jb64(j interface{}) (str string, err error) {
	t, err := json.Marshal(j)
	if err != nil {
		return "", err
	}
	return b64.StdEncoding.EncodeToString(t), nil
}

func (s *MinediveServer) SendPeer(cli *MinediveClient) {
	var c2 *MinediveClient
	var m1, m2 Cell
	var p1, p2 Cell
	var err error
	c2, err = s.GetOtherPeer(cli)
	if err != nil {
		m1.Type = "userlist"
		wsjson.Write(context.Background(), cli.Ws, m1)
		return
	}
	log.Println("other peer found", c2.Name)
	p1.Type = "user"
	p1.D0 = cli.Name
	p1.D1, err = c2.GetAlias(cli.Name)
	if cli.Exit {
		p1.D2 = "e"
	}
	if err != nil {
		log.Println(err)
	}
	p2.D0 = c2.Name
	p2.D1, err = cli.GetAlias(c2.Name)
	if c2.Exit {
		p2.D2 = "e"
	}
	if err != nil {
		log.Println(err)
	}
	m1.Type = "userlist"
	log.Println(p2)
	m1.D1, err = jb64(p2)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println(m1.D1)
	log.Println(m1)
	m1.D0 = "0"
	wsjson.Write(context.Background(), cli.Ws, m1)
	log.Println("sent", p2.D0, "to", cli.Name)
	m2.Type = "userlist"
	m2.D1, err = jb64(p1)
	if err != nil {
		log.Println(err)
		return
	}
	m2.D0 = "1"
	wsjson.Write(context.Background(), c2.Ws, m2)
	log.Println("sent", p1.D0, "to", c2.Name)
}

func (s *MinediveServer) DecryptAlias(alias string, gwName string) (string, error) {
	var encrypted, decrypted []byte
	var decryptNonce [24]byte
	gw, err := s.GetClientByName(gwName)
	if err != nil {
		log.Println(err)
		return "", err
	}
	encrypted, err = b64.StdEncoding.DecodeString(alias)
	copy(decryptNonce[:], encrypted[:24])
	decrypted, ok := secretbox.Open(nil, encrypted[24:], &decryptNonce, &gw.SecretKey)
	if ok != true {
		return "", errors.New("decryption failed")
	}
	a, err := s.GetClientByName(string(decrypted))
	if err != nil {
		log.Println(err)
		return "nil", err
	}
	return b64.StdEncoding.EncodeToString(a.PublicKey[:]), nil
}

func (s *MinediveServer) FwdToTarget(m *Cell) {
	s.clientsMutex.Lock()
	var c *MinediveClient
	for n := range s.clients {
		c = s.clients[n]
		if c.Name == m.D1 {
			wsjson.Write(context.Background(), c.Ws, m)
		}
	}
	s.clientsMutex.Unlock()
}

func (s *MinediveServer) SendKey(c *MinediveClient, req *Cell) {
	var m Cell
	if c.Name == req.D1 {
		log.Println("Client is his own GW")
		return
	}
	aliasKey, err := s.DecryptAlias(req.D0, req.D1)
	if err != nil {
		log.Println(err)
		return
	}
	m.Type = "key"
	m.D0 = req.D0
	m.D1 = aliasKey
	log.Println("Sending Message: ", m)
	wsjson.Write(context.Background(), c.Ws, m)
}
