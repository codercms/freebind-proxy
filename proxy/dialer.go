package proxy

import (
	"github.com/codercms/freebind-proxy/utils"
	"log"
	"net"
	"syscall"
)

type DialerFactoryIface interface {
	GetDialer() *net.Dialer
}

type RandIpDialerFactory struct {
	subnet *net.IPNet

	minAddr utils.IPUint128
	maxAddr utils.IPUint128
}

func MakeRandIpDialerFactory(subnet *net.IPNet) *RandIpDialerFactory {
	minAddr, maxAddr := utils.GetLowerUpperIPsWithUint128(subnet)

	return &RandIpDialerFactory{
		subnet:  subnet,
		minAddr: minAddr,
		maxAddr: maxAddr,
	}
}

func (f *RandIpDialerFactory) GetDialer() *net.Dialer {
	randIp := utils.GetRandomIpFromRange(f.minAddr, f.maxAddr)

	d := net.Dialer{
		LocalAddr: &net.TCPAddr{
			IP: randIp.GetIP(),
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
