package minedive

import "fmt"

type Cell struct {
	Type string `json:"type"`
	D0   string `json:"d0"`
	D1   string `json:"d1"`
	D2   string `json:"d2"`
	D3   string `json:"d3"`
}

func (a *Cell) String() string {
	return fmt.Sprintf("%v (%v %v %v %v)", a.Type, a.D0, a.D1, a.D2, a.D3)
}

// type p1Users struct {
// 	Name  string `json:"name"`
// 	Alias string `json:"alias"`
// }

// type dataMsg struct {
// 	Type string `json:"type"`
// 	Data string `json:"data"`
// }

// type userlistMsg struct {
// 	Type    string    `json:"type"`
// 	Contact int       `json:"contact"`
// 	Users   []p1Users `json:"users"`
// }

// type idMsg struct {
// 	Type string `json:"type"`
// 	ID   uint64 `json:"id"`
// }

// type usernameMsg struct {
// 	Type string `json:"type"`
// 	ID   uint64 `json:"id"`
// 	Name string `json:"name"`
// 	PK   string `json:"pk"`
// }

// type webrtcMsg struct {
// 	Type   string `json:"type"`
// 	Name   string `json:"name"`
// 	Target string `json:"target"`
// 	SDP    string `json:"sdp"`
// }

// type keyReq struct {
// 	Type  string `json:"type"`
// 	Alias string `json:"alias"`
// 	GW    string `json:"gw"`
// }

// type keyMsg struct {
// 	Type  string `json:"type"`
// 	Alias string `json:"alias"`
// 	Key   string `json:"key"`
// }
