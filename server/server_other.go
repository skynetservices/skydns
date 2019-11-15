// +build !linux

package server

import (
	"github.com/miekg/dns"
)

func (s *server) activateSystemd(mux *dns.ServeMux, dnsReadyMsg func(addr, net string)) error {
	// Noop on non-linux platforms
	return nil
}
