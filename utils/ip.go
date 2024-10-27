package utils

import (
	"encoding/binary"
	"math/rand/v2"
	"net/netip"
)

func GetRandomIpFromPrefix(randReader *rand.Rand, prefix netip.Prefix) netip.Addr {
	if prefix.IsSingleIP() {
		return prefix.Addr()
	}

	// Generate a random number with the remaining host bits.
	hostBits := uint8(prefix.Addr().BitLen() - prefix.Bits())

	if prefix.Addr().Is4() {
		// For IPv4, generate a random 32-bit number and mask it to fit the host bits
		maxValue := uint32(1<<hostBits) - 1

		rnd := randReader.Uint32() & maxValue

		// Apply the random host bits to the network prefix
		addrBe := prefix.Addr().As4()
		ip4 := binary.BigEndian.Uint32(addrBe[:]) | rnd

		return netip.AddrFrom4([4]byte{
			byte(ip4 >> 24),
			byte(ip4 >> 16),
			byte(ip4 >> 8),
			byte(ip4),
		})
	}

	if prefix.Addr().Is6() {
		// For IPv6, generate a random 128-bit number and mask it to fit the host bits
		maxValue := uint128{}.addOne().lsh(uint(hostBits)).subOne()

		rnd := uint128{
			hi: randReader.Uint64(),
			lo: randReader.Uint64(),
		}.and(maxValue)

		// Apply the random host bits to the network prefix
		addrBe := prefix.Addr().As16()
		ip6 := uint128{
			hi: binary.BigEndian.Uint64(addrBe[:8]),
			lo: binary.BigEndian.Uint64(addrBe[8:]),
		}.or(rnd)

		return netip.AddrFrom16([16]byte{
			byte(ip6.hi >> 56),
			byte(ip6.hi >> 48),
			byte(ip6.hi >> 40),
			byte(ip6.hi >> 32),
			byte(ip6.hi >> 24),
			byte(ip6.hi >> 16),
			byte(ip6.hi >> 8),
			byte(ip6.hi),

			byte(ip6.lo >> 56),
			byte(ip6.lo >> 48),
			byte(ip6.lo >> 40),
			byte(ip6.lo >> 32),
			byte(ip6.lo >> 24),
			byte(ip6.lo >> 16),
			byte(ip6.lo >> 8),
			byte(ip6.lo),
		})
	}

	panic("Unknown prefix")
}
