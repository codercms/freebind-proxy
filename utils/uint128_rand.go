package utils

import (
	"encoding/binary"
	"math/rand/v2"
	"time"
)

var seed [32]byte
var chaChaRandReader *rand.ChaCha8
var randReader *rand.Rand

func init() {
	var t = time.Now().UnixNano()

	binary.LittleEndian.PutUint64(seed[:], uint64(t))
	binary.LittleEndian.PutUint64(seed[8:], uint64(t))
	binary.LittleEndian.PutUint64(seed[16:], uint64(t))
	binary.LittleEndian.PutUint64(seed[24:], uint64(t))

	chaChaRandReader = rand.NewChaCha8(seed)
	randReader = rand.New(chaChaRandReader)
}

// generateRandomUint128 generates a random uint128.
func generateRandomUint128() uint128 {
	hi := randReader.Uint64()
	lo := randReader.Uint64()

	return uint128{hi: hi, lo: lo}
}

// generateRandomUint128 generates a random uint128 within specified bounds.
func generateRandomUint128InRange(min, max uint128) uint128 {
	if max.cmp(min) == -1 {
		panic("min cannot be greater than max")
	}

	var lo, hi uint64

	if max.lo-min.lo == 0 {
		lo = min.lo
	} else {
		lo = rand.Uint64N(max.lo-min.lo) + min.lo
	}

	if max.hi-min.hi == 0 {
		hi = min.hi
	} else {
		hi = rand.Uint64N(max.hi-min.hi) + min.hi
	}

	return uint128{hi: hi, lo: lo}
}
