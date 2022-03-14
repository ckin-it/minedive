package minedive

type minL1Msg struct {
	From string `json:"from"`
	Type string `json:"type"`
}

type pingL1Msg struct {
	From string `json:"from"`
	Text string `json:"text"`
	Type string `json:"type"`
}

type L2L1Msg struct {
	From string   `json:"from"`
	Type string   `json:"type"`
	I    int      `json:"i"`
	L2   []string `json:"l2"`
}

type FwdMsg struct {
	From string `json:"from"`
	Type string `json:"type"`
	To   string `json:"to"`
	Msg  string `json:"msg"`
	Ori  string `json:"ori,omitempty"`
}

type L2Msg struct {
	Type  string   `json:"type"`
	Lang  string   `json:"l,omitempty"`
	Query string   `json:"q,omitempty"`
	Text  []string `json:"text,omitempty"`
}
