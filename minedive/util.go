package minedive

import (
	b64 "encoding/base64"
	"encoding/binary"
	"errors"
	"math/rand"
	"time"
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

func GetRandomName(id uint64, sseed string) string {
	bid := make([]byte, 8)
	binary.LittleEndian.PutUint64(bid, id)
	token := make([]byte, 24)
	rand.Seed(time.Now().UnixNano())
	rand.Read(token)
	for i, v := range sseed {
		token[i] ^= byte(v)
	}
	//token = append(token, bid...)
	r := b64.StdEncoding.EncodeToString(token)
	return r
}
