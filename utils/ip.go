package utils

import (
	"encoding/binary"
	"net"
)

func ipToUint128(ip net.IP) uint128 {
	var res uint128

	if len(ip) == net.IPv4len {
		// src: netip.AddrFrom4()
		res.lo = 0xffff00000000 | uint64(ip[0])<<24 | uint64(ip[1])<<16 | uint64(ip[2])<<8 | uint64(ip[3])
	} else {
		// src: netip.AddrFrom16():
		// addr: uint128{
		//	 byteorder.BeUint64(addr[:8]), // <- hi
		//	 byteorder.BeUint64(addr[8:]), // <- lo
		// }

		// Handle inverse byte order
		res.hi = binary.BigEndian.Uint64(ip[:8])
		res.lo = binary.BigEndian.Uint64(ip[8:])
	}

	return res
}

func uint128ToIp(ip uint128, len int) net.IP {
	var res net.IP

	if len == net.IPv4len {
		// src: netip pkg
		// func (ip addr) As4() (a4 [4]byte) {
		//	 if ip.z == z4 || ip.Is4In6() {
		//		byteorder.BePutUint32(a4[:], uint32(ip.addr.lo))
		//		return a4
		//	 }
		//	 if ip.z == z0 {
		//		panic("As4 called on ip zero value")
		//	 }
		//	 panic("As4 called on IPv6 address")
		// }

		res = make(net.IP, net.IPv4len)
		binary.BigEndian.PutUint32(res, uint32(ip.lo))
	} else {
		// src: netip pkg
		// func (ip addr) As16() (a16 [16]byte) {
		//	 byteorder.BePutUint64(a16[:8], ip.addr.hi)
		//	 byteorder.BePutUint64(a16[8:], ip.addr.lo)
		//	 return a16
		// }

		res = make(net.IP, net.IPv6len)
		binary.BigEndian.PutUint64(res[:8], ip.hi)
		binary.BigEndian.PutUint64(res[8:], ip.lo)
	}

	return res
}

// GetLowerUpperIPs returns the lower (network) and upper (broadcast) IPs of the given subnet.
func GetLowerUpperIPs(ipNet *net.IPNet) (lo net.IP, hi net.IP) {
	// Lower ip is the network address itself
	lowerIP := ipNet.IP

	// Convert ip and Mask to uint128 easier bit manipulation
	ip := ipToUint128(lowerIP)
	mask := ipToUint128(net.IP(ipNet.Mask))

	upperIp := ip.or(mask.not())

	upperIP := uint128ToIp(upperIp, len(ipNet.IP))

	return lowerIP, upperIP
}

type IPUint128 struct {
	addr uint128
	ip   net.IP
}

func (ip IPUint128) GetUint128Addr() (lo, hi uint64) {
	return ip.addr.lo, ip.addr.hi
}

func (ip IPUint128) GetIP() net.IP {
	res := make([]byte, len(ip.ip))
	copy(res, ip.ip)

	return res
}

// Is4In6 reports whether ip is an IPv4-mapped IPv6 address.
func (ip IPUint128) Is4In6() bool {
	return len(ip.ip) == net.IPv4len && ip.addr.hi == 0 && ip.addr.lo>>32 == 0xffff
}

func GetLowerUpperIPsWithUint128(ipNet *net.IPNet) (lo IPUint128, hi IPUint128) {
	// Lower ip is the network address itself
	lowerIP := ipNet.IP

	// Convert IP and Mask to uint128 easier bit manipulation
	ip := ipToUint128(lowerIP)
	mask := ipToUint128(net.IP(ipNet.Mask))

	upperIp := ip.or(mask.not())

	upperIP := uint128ToIp(upperIp, len(ipNet.IP))
	if len(ipNet.IP) == net.IPv4len {
		upperIp.hi = 0
	}

	return IPUint128{
			addr: ipToUint128(lowerIP),
			ip:   lowerIP,
		}, IPUint128{
			addr: upperIp,
			ip:   upperIP,
		}
}

func GetRandomIpFromRange(min IPUint128, max IPUint128) IPUint128 {
	// Handle IPv4 separately
	if min.Is4In6() {
		minAddr := binary.BigEndian.Uint32(min.ip)
		maxAddr := binary.BigEndian.Uint32(max.ip)

		if minAddr == maxAddr {
			return min
		}

		res := randReader.Uint32N(maxAddr-minAddr) + minAddr

		var ip net.IP = make([]byte, 4)
		binary.BigEndian.PutUint32(ip, res)

		return IPUint128{
			addr: ipToUint128(ip),
			ip:   ip,
		}
	}

	res := generateRandomUint128InRange(min.addr, max.addr)

	return IPUint128{
		addr: res,
		ip:   uint128ToIp(res, len(min.ip)),
	}
}
