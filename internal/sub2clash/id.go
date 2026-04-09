package sub2clash

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

func NewProfileID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("prf_%d", time.Now().UnixNano())
	}
	return "prf_" + hex.EncodeToString(buf[:])
}
