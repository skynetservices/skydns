// Copyright (c) 2014 The SkyDNS Authors. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/miekg/dns"
)

const Version = "2.0.0d"

var (
	tlskey     = ""
	tlspem     = ""
	cacert     = ""
	config     = &Config{ReadTimeout: 0, Domain: make(DomainConfigMap), DnsAddr: "", DNSSEC: ""}
	nameserver = ""
	machine    = ""
	discover   = false
	verbose    = false
)

const (
	SCacheCapacity = 10000
	RCacheCapacity = 100000
	RCacheTtl      = 60
)

func env(key, def string) string {
	if x := os.Getenv(key); x != "" {
		return x
	}
	return def
}

func init() {
	flag.Var(&config.Domain, "domain", "domain to anchor requests to (SKYDNS_DOMAIN)")
	flag.StringVar(&config.DnsAddr, "addr", env("SKYDNS_ADDR", "127.0.0.1:53"), "ip:port to bind to (SKYDNS_ADDR)")
	flag.StringVar(&nameserver, "nameservers", env("SKYDNS_NAMESERVERS", ""), "nameserver address(es) to forward (non-local) queries to e.g. 8.8.8.8:53,8.8.4.4:53")
	flag.StringVar(&machine, "machines", env("ETCD_MACHINES", ""), "machine address(es) running etcd")
	flag.StringVar(&config.DNSSEC, "dnssec", "", "basename of DNSSEC key file e.q. Kskydns.local.+005+38250")
	flag.StringVar(&config.Local, "local", "", "optional unique value for this skydns instance")
	flag.StringVar(&tlskey, "tls-key", env("ETCD_TLSKEY", ""), "TLS Private Key path")
	flag.StringVar(&tlspem, "tls-pem", env("ETCD_TLSPEM", ""), "X509 Certificate")
	flag.StringVar(&cacert, "ca-cert", env("ECTD_CACERT", ""), "CA Certificate")
	flag.DurationVar(&config.ReadTimeout, "rtimeout", 2*time.Second, "read timeout")
	flag.BoolVar(&config.RoundRobin, "round-robin", true, "round robin A/AAAA replies")
	flag.BoolVar(&discover, "discover", false, "discover new machines by watching /v2/_etcd/machines")
	flag.BoolVar(&verbose, "verbose", false, "log queries")
	flag.BoolVar(&config.Systemd, "systemd", false, "bind to socket(s) activated by systemd (ignore -addr)")

	// TTl
	// Minttl
	flag.StringVar(&config.Hostmaster, "hostmaster", "hostmaster@skydns.local.", "hostmaster email address to use")
	flag.IntVar(&config.SCache, "scache", SCacheCapacity, "capacity of the signature cache")
	flag.IntVar(&config.RCache, "rcache", 0, "capacity of the response cache") // default to 0 for now
	flag.IntVar(&config.RCacheTtl, "rcache-ttl", RCacheTtl, "TTL of the response cache")
}

func main() {
	flag.Parse()
	machines := strings.Split(machine, ",")
	client := NewClient(machines)
	if nameserver != "" {
		for _, hostPort := range strings.Split(nameserver, ",") {
			if err := validateHostPort(hostPort); err != nil {
				log.Fatalf("nameserver is invalid: %s\n", err)
			}
			config.Nameservers = append(config.Nameservers, hostPort)
		}
	}
	if err := validateHostPort(config.DnsAddr); err != nil {
		log.Fatalf("addr is invalid: %s\n", err)
	}
	if len(config.Domain) == 0 {
		config.Domain.Set(env("SKYDNS_DOMAIN", "skydns.local."))
	}
	config, err := loadConfig(client, config)
	if err != nil {
		log.Fatal(err)
	}
	s := NewServer(config, client)
	if s.config.Local != "" {
		s.config.Local = dns.Fqdn(s.config.Local)
	}

	if discover {
		go func() {
			recv := make(chan *etcd.Response)
			go s.client.Watch("/_etcd/machines/", 0, true, recv, nil)
			for {
				select {
				case n := <-recv:
					// we can see an n == nil, probably when we can't connect to etcd.
					if n != nil {
						s.UpdateClient(n)
					}
				}
			}
		}()
	}

	statsCollect()

	if err := s.Run(); err != nil {
		log.Fatal(err)
	}
}

func validateHostPort(hostPort string) error {
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		return err
	}
	if ip := net.ParseIP(host); ip == nil {
		return fmt.Errorf("bad IP address: %s", host)
	}

	if p, _ := strconv.Atoi(port); p < 1 || p > 65535 {
		return fmt.Errorf("bad port number %s", port)
	}
	return nil
}
