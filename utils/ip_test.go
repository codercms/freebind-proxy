package utils

import (
	"net"
	"net/netip"
	"reflect"
	"testing"
)

func mustParseCIDR(s string) *net.IPNet {
	_, cidr, err := net.ParseCIDR(s)
	if err != nil {
		panic("Bad CIDR " + err.Error())
	}

	return cidr
}

func mustParseIP(s string) net.IP {
	ip, err := netip.ParseAddr(s)
	if err != nil {
		panic("bad ip " + err.Error())
	}

	return ip.AsSlice()
}

func TestGetLowerUpperIPs(t *testing.T) {
	tests := []struct {
		name     string
		ipNet    *net.IPNet
		wantLow  net.IP
		wantHigh net.IP
	}{
		{
			"ipv6 test /64",
			mustParseCIDR("2001:0db8:85a3:0000:0000:8a2e:0370:7334/64"),
			mustParseIP("2001:0db8:85a3:0000:0000:0000:0000:0000"),
			mustParseIP("2001:0db8:85a3:0000:ffff:ffff:ffff:ffff"),
		},
		{
			"ipv6 test /48",
			mustParseCIDR("2001:0db8:85a3:0000:0000:8a2e:0370:7334/48"),
			mustParseIP("2001:0db8:85a3:0000:0000:0000:0000:0000"),
			mustParseIP("2001:0db8:85a3:ffff:ffff:ffff:ffff:ffff"),
		},
		{
			"ipv6 test /32",
			mustParseCIDR("2001:db8::/32"),
			mustParseIP("2001:db8::"),
			mustParseIP("2001:db8:ffff:ffff:ffff:ffff:ffff:ffff"),
		},
		{
			"ipv4 test /24",
			mustParseCIDR("65.128.0.120/24"),
			mustParseIP("65.128.0.0"),
			mustParseIP("65.128.0.255"),
		},
		{
			"ipv4 test /32",
			mustParseCIDR("65.128.0.120/32"),
			mustParseIP("65.128.0.120"),
			mustParseIP("65.128.0.120"),
		},
		{
			"ipv4 test /24 (local)",
			mustParseCIDR("192.168.1.0/24"),
			mustParseIP("192.168.1.0"),
			mustParseIP("192.168.1.255"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLowest, gotHighest := GetLowerUpperIPs(tt.ipNet)
			if !reflect.DeepEqual(gotLowest, tt.wantLow) {
				t.Errorf("GetLowerUpperIPs() gotLowest = %v, wantLow %v", gotLowest.String(), tt.wantLow.String())
			}
			if !reflect.DeepEqual(gotHighest, tt.wantHigh) {
				t.Errorf("GetLowerUpperIPs() gotHighest = %v, wantHigh %v", gotHighest.String(), tt.wantHigh.String())
			}
		})
	}
}

func TestGetRandomIpFromRange(t *testing.T) {
	tests := []struct {
		name  string
		ipNet *net.IPNet
	}{
		{
			"ipv6 test /64",
			mustParseCIDR("2001:0db8:85a3:0000:0000:8a2e:0370:7334/64"),
		},
		{
			"ipv6 test /48",
			mustParseCIDR("2001:0db8:85a3:0000:0000:8a2e:0370:7334/48"),
		},
		{
			"ipv6 test /32",
			mustParseCIDR("2001:db8::/32"),
		},
		{
			"ipv4 test /24",
			mustParseCIDR("65.128.0.120/24"),
		},
		{
			"ipv4 test /32",
			mustParseCIDR("65.128.0.120/32"),
		},
		{
			"ipv4 test /24 (local)",
			mustParseCIDR("192.168.1.0/24"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lo, hi := GetLowerUpperIPsWithUint128(tt.ipNet)

			for i := 0; i < 1000; i++ {
				got := GetRandomIpFromRange(lo, hi)

				if !tt.ipNet.Contains(got.ip) {
					t.Errorf("GetRandomIpFromRange() got = %v which is out of subnet space %s", got.ip.String(), tt.ipNet)
				}

				if lo.addr.cmp(got.addr) == 1 {
					t.Errorf("GetRandomIpFromRange() got = %v, min allowed %v", got.ip.String(), lo.ip.String())
					break
				}
				if hi.addr.cmp(got.addr) == -1 {
					t.Errorf("GetRandomIpFromRange() got = %v, max allowed %v", got.ip.String(), hi.ip.String())
					break
				}

				//t.Logf("Got %s", got.ip.String())
			}
		})
	}
}
