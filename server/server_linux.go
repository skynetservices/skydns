package server

import (
	"fmt"
	"net"

	"github.com/coreos/go-systemd/activation"
	"github.com/miekg/dns"
)

func (s *server) activateSystemd(mux *dns.ServeMux, dnsReadyMsg func(addr, net string)) error {
	packetConns, err := activation.PacketConns(false)
	if err != nil {
		return err
	}
	listeners, err := activation.Listeners(true)
	if err != nil {
		return err
	}
	if len(packetConns) == 0 && len(listeners) == 0 {
		return fmt.Errorf("no UDP or TCP sockets supplied by systemd")
	}
	for _, p := range packetConns {
		if u, ok := p.(*net.UDPConn); ok {
			s.group.Add(1)
			go func() {
				defer s.group.Done()
				if err := dns.ActivateAndServe(nil, u, mux); err != nil {
					fatalf("%s", err)
				}
			}()
			dnsReadyMsg(u.LocalAddr().String(), "udp")
		}
	}
	for _, l := range listeners {
		if t, ok := l.(*net.TCPListener); ok {
			s.group.Add(1)
			go func() {
				defer s.group.Done()
				if err := dns.ActivateAndServe(t, nil, mux); err != nil {
					fatalf("%s", err)
				}
			}()
			dnsReadyMsg(t.Addr().String(), "tcp")
		}
	}
	return nil
}
