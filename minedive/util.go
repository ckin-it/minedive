package minedive

import "errors"

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
	return errors.New("incNonce: nonce expired")
}
