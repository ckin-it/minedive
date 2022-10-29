package minedive

type CircuitState int

const (
	CircuitStateNew CircuitState = iota + 1
	CircuitStateKeys
)

const (
	CircuitStateNewStr  = "new"
	CircuitStateKeysStr = "got all keys"
)

func (t CircuitState) String() string {
	switch t {
	case CircuitStateNew:
		return CircuitStateNewStr
	case CircuitStateKeys:
		return CircuitStateKeysStr
	default:
		return ErrUnknownType.Error()
	}
}
