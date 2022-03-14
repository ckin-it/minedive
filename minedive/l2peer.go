package minedive

type L2Peer struct {
	Name       string
	GW         *L1Peer
	Inonce     [24]byte
	Ononce     [24]byte
	MsgIgnored int
	State      string
	K          [32]byte
	PK         [32]byte
}
