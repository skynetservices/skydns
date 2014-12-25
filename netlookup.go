package main

import (
	"net"
	"strings"

	"github.com/miekg/dns"
	"github.com/skynetservices/skydns/msg"
)

func (s *server) NetLookup(w dns.ResponseWriter, req *dns.Msg) (records []dns.RR) {
	var (
		err    error
		ips    []net.IP
		record dns.RR
		name   string
		serv   = new(msg.Service)
	)

	for _, q := range req.Question {
		name = strings.ToLower(q.Name)
		name = strings.TrimRight(name, ".")

		if ips, err = net.LookupIP(name); err == nil && len(ips) > 0 {
			for _, ip := range ips {
				record = serv.NewA(q.Name, ip)
				records = append(records, record)
			}
		}
	}

	return
}
