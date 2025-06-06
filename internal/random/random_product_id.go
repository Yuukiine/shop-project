package random

import (
	"math/rand"
	"time"
)

func RandomProductID(number int) int {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	return rand.Intn(number)
}
