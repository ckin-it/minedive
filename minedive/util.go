package minedive

import (
	b64 "encoding/base64"
	"errors"
)

func IncNonce(a []byte, dyn int) error {
	l := len(a)
	if l < dyn {
		dyn = l
	}
	for i := 1; i <= dyn; i++ {
		if a[l-i] < 0xff {
			a[l-i]++
			return nil
		}
		a[l-i] = 0
	}
	return errors.New("IncNonce: nonce expired")
}

func UseNonce(a []byte) (string, error) {
	err := IncNonce(a, len(a))
	return b64.StdEncoding.EncodeToString(a), err
}
