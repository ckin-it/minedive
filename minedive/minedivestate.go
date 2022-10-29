package minedive

type MinediveState int

const (
	MinediveStateNew MinediveState = iota + 1
	MinediveStateConnecting
)

const (
	MinediveStateNewStr        = "new"
	MinediveStateConnectingStr = "connecting"
)

func (t MinediveState) String() string {
	switch t {
	case MinediveStateNew:
		return MinediveStateNewStr
	case MinediveStateConnecting:
		return MinediveStateConnectingStr
	default:
		return ErrUnknownType.Error()
	}
}
