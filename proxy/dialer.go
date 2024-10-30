package proxy

import (
	"github.com/codercms/freebind-proxy/utils"
	"log"
	"math/rand/v2"
	"net"
	"net/netip"
	"syscall"
)

type DialerFactoryIface interface {
	GetDialer() *net.Dialer
}

// StaticDialerFactory provides user specified dialer
// if user provided dialer is unspecified provides empty dialer struct
type StaticDialerFactory struct {
	dialer *net.Dialer
}

func MakeNoIpDialerFactory(dialer *net.Dialer) *StaticDialerFactory {
	if dialer == nil {
		dialer = &net.Dialer{}
	}

	return &StaticDialerFactory{
		dialer: dialer,
	}
}

func (f *StaticDialerFactory) GetDialer() *net.Dialer {
	return f.dialer
}

// RandIpDialerFactory provides dialer with random IP from provided network prefix
type RandIpDialerFactory struct {
	randReader *rand.Rand

	prefix netip.Prefix
}

func MakeRandIpDialerFactory(randReader *rand.Rand, prefix netip.Prefix) *RandIpDialerFactory {
	return &RandIpDialerFactory{
		randReader: randReader,

		prefix: prefix,
	}
}

func (f *RandIpDialerFactory) GetDialer() *net.Dialer {
	randIp := utils.GetRandomIpFromPrefix(f.randReader, f.prefix)

	d := net.Dialer{
		LocalAddr: &net.TCPAddr{
			IP: randIp.AsSlice(),
		},

		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				if err := syscall.SetsockoptInt(int(fd), syscall.SOL_IP, syscall.IP_FREEBIND, 1); err != nil {
					log.Printf("Failed to set IP_FREEBIND: %v\n", err)
				}

				//if err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_KEEPALIVE, 1); err != nil {
				//	log.Printf("Failed to set SO_KEEPALIVE: %v\n", err)
				//}
			})
		},
	}

	return &d
}
