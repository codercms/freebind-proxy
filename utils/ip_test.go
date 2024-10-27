package utils

import (
	"encoding/binary"
	"math/rand/v2"
	"net/netip"
	"testing"
	"time"
)

var seed [32]byte
var chaChaRandReader *rand.ChaCha8
var defaultRandReader *rand.Rand

func init() {
	var t = time.Now().UnixNano()

	binary.LittleEndian.PutUint64(seed[:], uint64(t))
	binary.LittleEndian.PutUint64(seed[8:], uint64(t))
	binary.LittleEndian.PutUint64(seed[16:], uint64(t))
	binary.LittleEndian.PutUint64(seed[24:], uint64(t))

	chaChaRandReader = rand.NewChaCha8(seed)
	defaultRandReader = rand.New(chaChaRandReader)
}

func mustParsePrefix(s string) *netip.Prefix {
	prefix, err := netip.ParsePrefix(s)
	if err != nil {
		panic("Bad prefix " + err.Error())
	}

	return &prefix
}

func TestGetRandomIpFromPrefix(t *testing.T) {
	tests := []struct {
		name  string
		ipNet *netip.Prefix
	}{
		{
			"ipv6 test /128",
			mustParsePrefix("::1/128"),
		},
		{
			"ipv6 test /64",
			mustParsePrefix("2001:0db8:85a3:0000:0000:8a2e:0370:7334/64"),
		},
		{
			"ipv6 test /48",
			mustParsePrefix("2001:0db8:85a3:0000:0000:8a2e:0370:7334/48"),
		},
		{
			"ipv6 test /32",
			mustParsePrefix("2001:db8::/32"),
		},
		{
			"ipv4 test /24",
			mustParsePrefix("65.128.0.120/24"),
		},
		{
			"ipv4 test /32",
			mustParsePrefix("65.128.0.120/32"),
		},
		{
			"ipv4 test /24 (local)",
			mustParsePrefix("192.168.1.0/24"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < 1000; i++ {
				got := GetRandomIpFromPrefix(defaultRandReader, *tt.ipNet)

				if !tt.ipNet.Contains(got) {
					t.Errorf("GetRandomIpFromRange() got = %v which is out of subnet space %s", got.String(), tt.ipNet.String())
				}

				t.Logf("Got %s", got.String())
			}
		})
	}
}
