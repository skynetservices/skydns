// Copyright (c) 2014 The SkyDNS Authors. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/coreos/go-log/log"
	"github.com/miekg/dns"
)

type DomainConfig struct {
	// The hostmaster responsible for this domain,
	// defaults to config.Hostmaster or hostmaster.<Domain>.
	Hostmaster string `json:"hostmaster,omitempty"`

	localNSAlias FQDN // "ns.dns." + domain (legacy)
	dnsDomain FQDN // "dns". + domain

	// DNSSEC key material
	PubKey  *dns.DNSKEY    `json:"-"`
	KeyTag  uint16         `json:"-"`
	PrivKey dns.PrivateKey `json:"-"`
}

// Guarantees that field 'Hostmaster' is not empty.
func (dc *DomainConfig) FillHostmaster(domain FQDN) {
	if config.Hostmaster == "" {
		dc.Hostmaster = "hostmaster." + string(domain)
	} else {
		// People probably don't know that SOA's email addresses cannot
		// contain @-signs, replace them with dots
		dc.Hostmaster = dns.Fqdn(strings.Replace(config.Hostmaster, "@", ".", -1))
	}
}

func (dc *DomainConfig) PopulateNameserver(domain FQDN) {
	if dc.localNSAlias == NoFQDN {
		dc.localNSAlias = FQDN("ns.dns." + string(domain))
	}
	if dc.dnsDomain == NoFQDN {
		dc.dnsDomain = FQDN("dns." + string(domain))
	}
}

// FQDN in lower case
type FQDN string

const NoFQDN = FQDN("")

func NewFQDN(domain string) FQDN {
	return FQDN(dns.Fqdn(strings.ToLower(domain)))
}

type DomainConfigMap map[FQDN]DomainConfig

// Returns nothing but the keys. For the flag.Value interface.
func (dcm *DomainConfigMap) String() string {
	return "" // XXX
}

// For the flag.Value interface.
func (dcm *DomainConfigMap) Set(value string) error {
	for _, domain := range strings.Split(value, ",") {
		fqdn := NewFQDN(domain)
		t := DomainConfig{}
		t.FillHostmaster(fqdn)
		t.PopulateNameserver(fqdn)
		(*dcm)[fqdn] = t
	}
	return nil
}

// Returns the ancestor of 'subdomain' or NoFQDN.
func (dcm *DomainConfigMap) Contains(subdomain FQDN) FQDN {
	for fqdn, _ := range *dcm {
		if strings.HasSuffix(string(subdomain), string(fqdn)) {
			return fqdn
		}
	}
	return NoFQDN
}

// Config provides options to the SkyDNS resolver.
type Config struct {
	// The ip:port SkyDNS should be listening on for incoming DNS requests.
	DnsAddr string `json:"dns_addr,omitempty"`
	// bind to port(s) activated by systemd. If set to true, this overrides DnsAddr.
	Systemd bool `json:"systemd,omitempty"`
	// The domain SkyDNS is authoritative for, defaults to "skydns.local." → {}
	Domain DomainConfigMap `json:"domain,omitempty"`
	// Domain pointing to a key where service info is stored when being queried
	// for local.dns.skydns.local.
	Local string `json:"local,omitempty"`
	// The default hostmaster for domains where none is set.
	Hostmaster string `json:"hostmaster,omitempty"`
	DNSSEC     string `json:"dnssec,omitempty"`
	// Round robin A/AAAA replies. Default is true.
	RoundRobin bool `json:"round_robin,omitempty"`
	// List of ip:port, seperated by commas of recursive nameservers to forward queries to.
	Nameservers []string      `json:"nameservers,omitempty"`
	ReadTimeout time.Duration `json:"read_timeout,omitempty"`
	// Default priority on SRV records when none is given. Defaults to 10.
	Priority uint16 `json:"priority"`
	// Default TTL, in seconds, when none is given in etcd. Defaults to 3600.
	Ttl uint32 `json:"ttl,omitempty"`
	// Minimum TTL, in seconds, for NXDOMAIN responses. Defaults to 300.
	MinTtl uint32 `json:"min_ttl,omitempty"`
	// SCache, capacity of the signature cache in signatures stored.
	SCache int `json:"scache,omitempty"`
	// RCache, capacity of response cache in resource records stored.
	RCache int `json:"rcache,omitempty"`
	// RCacheTtl, how long to cache in seconds.
	RCacheTtl int `json:"rcache_ttl,omitempty"`
	// How many labels a name should have before we allow forwarding. Default to 2.
	Ndots int `json:"ndot,omitempty"`

	log *log.Logger
}

func loadConfig(client *etcd.Client, config *Config) (*Config, error) {
	config.log = log.New("skydns", false,
		log.CombinedSink(os.Stderr, "[%s] %s %-9s | %s\n", []string{"prefix", "time", "priority", "message"}))

	// Override wat isn't set yet from the command line.
	n, err := client.Get("/skydns/config", false, false)
	if err != nil {
		config.log.Info("falling back to default configuration, could not read from etcd:", err)
		if err := setDefaults(config); err != nil {
			return nil, err
		}
		return config, nil
	}
	if err := json.Unmarshal([]byte(n.Node.Value), &config); err != nil {
		return nil, err
	}
	if err := setDefaults(config); err != nil {
		return nil, err
	}
	return config, nil
}

func setDefaults(config *Config) error {
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 2 * time.Second
	}
	if config.DnsAddr == "" {
		config.DnsAddr = "127.0.0.1:53"
	}
	if config.MinTtl == 0 {
		config.MinTtl = 60
	}
	if config.Ttl == 0 {
		config.Ttl = 3600
	}
	if config.Priority == 0 {
		config.Priority = 10
	}
	if config.RCache < 0 {
		config.RCache = 0
	}
	if config.SCache < 0 {
		config.SCache = 0
	}
	if config.RCacheTtl == 0 {
		config.RCacheTtl = RCacheTtl
	}
	if config.Ndots <= 0 {
		config.Ndots = 2
	}

	if len(config.Nameservers) == 0 {
		c, err := dns.ClientConfigFromFile("/etc/resolv.conf")
		if err != nil {
			return err
		}
		for _, s := range c.Servers {
			config.Nameservers = append(config.Nameservers, net.JoinHostPort(s, c.Port))
		}
	}

	if len(config.Domain) == 0 {
		config.Domain.Set("skydns.local.")
	}
	if config.Hostmaster == "" {
		config.Hostmaster = "hostmaster@skydns.local."
	}
	config.Hostmaster = dns.Fqdn(strings.Replace(config.Hostmaster, "@", ".", -1))
	for fqdn := range config.Domain {
		if config.Domain[fqdn].Hostmaster != "" {
			continue
		}
		tmpDomainConfig := config.Domain[fqdn]
		tmpDomainConfig.FillHostmaster(fqdn)
		config.Domain[fqdn] = tmpDomainConfig
	}
	if config.DNSSEC != "" {
		for _, keyfileName := range strings.Split(config.DNSSEC, ",") {
			// For some reason the + are replaces by spaces in etcd. Re-replace them
			keyfile := strings.Replace(keyfileName, " ", "+", -1)
			k, p, err := ParseKeyFile(keyfile)
			if err != nil {
				return err
			}
			fqdnFromHeader := NewFQDN(k.Header().Name)
			tmpDomainConfig, keyExists := config.Domain[fqdnFromHeader]
			if !keyExists {
				return fmt.Errorf(fmt.Sprintf("ownername of DNSKEY in '%s' must match SkyDNS domain", keyfile))
			}
			k.Header().Ttl = config.Ttl
			tmpDomainConfig.PubKey = k
			tmpDomainConfig.KeyTag = k.KeyTag()
			tmpDomainConfig.PrivKey = p
			config.Domain[fqdnFromHeader] = tmpDomainConfig
		}
	}
	return nil
}
