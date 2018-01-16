// Copyright (c) 2014 The SkyDNS Authors. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package server

// etcd needs to be running on http://127.0.0.1:2379

// NOTE: this is complete re-iteration of server_test.go with the only difference
// that it utilizes etcdv3 feature objects.

import (
	"crypto/rsa"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	backendetcdv3 "github.com/skynetservices/skydns/backends/etcd3"
	"github.com/skynetservices/skydns/cache"
	"github.com/skynetservices/skydns/msg"

	etcdv3 "github.com/coreos/etcd/clientv3"
	"github.com/miekg/dns"
	//"golang.org/x/net/context"
	//"github.com/skynetservices/skydns/backends/etcd"
)

func addServiceV3(t *testing.T, s *server, k string, ttl time.Duration, m *msg.Service) {
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	path, _ := msg.PathWithWildcard(k)

	_, err = s.backend.(*backendetcdv3.Backendv3).Client().Put(ctx, path, string(b))
	if err != nil {
		// TODO(miek): allow for existing keys...
		t.Fatal(err)
	}
}

func delServiceV3(t *testing.T, s *server, k string) {
	path, _ := msg.PathWithWildcard(k)
	_, err := s.backend.(*backendetcdv3.Backendv3).Client().Delete(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
}

func newTestServerV3(t *testing.T, c bool) *server {
	Port += 10
	StrPort = strconv.Itoa(Port)
	s := new(server)
	client, _ := etcdv3.New(etcdv3.Config{
		Endpoints: []string{"http://127.0.0.1:2379/"},
	})

	// TODO(miek): why don't I use NewServer??
	s.group = new(sync.WaitGroup)
	s.scache = cache.New(100, 0)
	s.rcache = cache.New(100, 0)
	if c {
		s.rcache = cache.New(100, 60) // 100 items, 60s ttl
	}
	s.config = new(Config)
	s.config.Domain = "skydns.test."
	s.config.DnsAddr = "127.0.0.1:" + StrPort
	s.config.Nameservers = []string{"8.8.4.4:53"}
	SetDefaults(s.config)
	s.config.Local = "104.server1.development.region1.skydns.test."
	s.config.Priority = 10
	s.config.RCacheTtl = RCacheTtl
	s.config.Ttl = 3600
	s.config.Ndots = 2
	s.config.Etcd3 = true;

	s.dnsUDPclient = &dns.Client{Net: "udp", ReadTimeout: 2 * s.config.ReadTimeout, WriteTimeout: 2 * s.config.ReadTimeout, SingleInflight: true}
	s.dnsTCPclient = &dns.Client{Net: "tcp", ReadTimeout: 2 * s.config.ReadTimeout, WriteTimeout: 2 * s.config.ReadTimeout, SingleInflight: true}

	s.backend = backendetcdv3.NewBackendv3(*client, ctx, &backendetcdv3.Config{
		Ttl:      s.config.Ttl,
		Priority: s.config.Priority,
	})

	go s.Run()
	time.Sleep(500 * time.Millisecond) // Yeah, yeah, should do a proper fix
	return s
}

func newTestServerDNSSECV3(t *testing.T, cache bool) *server {
	var err error
	s := newTestServerV3(t, cache)
	s.config.PubKey = newDNSKEY("skydns.test. IN DNSKEY 256 3 5 AwEAAaXfO+DOBMJsQ5H4TfiabwSpqE4cGL0Qlvh5hrQumrjr9eNSdIOjIHJJKCe56qBU5mH+iBlXP29SVf6UiiMjIrAPDVhClLeWFe0PC+XlWseAyRgiLHdQ8r95+AfkhO5aZgnCwYf9FGGSaT0+CRYN+PyDbXBTLK5FN+j5b6bb7z+d")
	s.config.KeyTag = s.config.PubKey.KeyTag()
	privKey, err := s.config.PubKey.ReadPrivateKey(strings.NewReader(`Private-key-format: v1.3
Algorithm: 5 (RSASHA1)
Modulus: pd874M4EwmxDkfhN+JpvBKmoThwYvRCW+HmGtC6auOv141J0g6MgckkoJ7nqoFTmYf6IGVc/b1JV/pSKIyMisA8NWEKUt5YV7Q8L5eVax4DJGCIsd1Dyv3n4B+SE7lpmCcLBh/0UYZJpPT4JFg34/INtcFMsrkU36PlvptvvP50=
PublicExponent: AQAB
PrivateExponent: C6e08GXphbPPx6j36ZkIZf552gs1XcuVoB4B7hU8P/Qske2QTFOhCwbC8I+qwdtVWNtmuskbpvnVGw9a6X8lh7Z09RIgzO/pI1qau7kyZcuObDOjPw42exmjqISFPIlS1wKA8tw+yVzvZ19vwRk1q6Rne+C1romaUOTkpA6UXsE=
Prime1: 2mgJ0yr+9vz85abrWBWnB8Gfa1jOw/ccEg8ZToM9GLWI34Qoa0D8Dxm8VJjr1tixXY5zHoWEqRXciTtY3omQDQ==
Prime2: wmxLpp9rTzU4OREEVwF43b/TxSUBlUq6W83n2XP8YrCm1nS480w4HCUuXfON1ncGYHUuq+v4rF+6UVI3PZT50Q==
Exponent1: wkdTngUcIiau67YMmSFBoFOq9Lldy9HvpVzK/R0e5vDsnS8ZKTb4QJJ7BaG2ADpno7pISvkoJaRttaEWD3a8rQ==
Exponent2: YrC8OglEXIGkV3tm2494vf9ozPL6+cBkFsPPg9dXbvVCyyuW0pGHDeplvfUqs4nZp87z8PsoUL+LAUqdldnwcQ==
Coefficient: mMFr4+rDY5V24HZU3Oa5NEb55iQ56ZNa182GnNhWqX7UqWjcUUGjnkCy40BqeFAQ7lp52xKHvP5Zon56mwuQRw==
`), "stdin")
	s.config.PrivKey = privKey.(*rsa.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDNSForwardV3(t *testing.T) {
	s := newTestServerV3(t, false)
	defer s.Stop()

	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion("www.example.com.", dns.TypeA)
	resp, _, err := c.Exchange(m, "127.0.0.1:"+StrPort)
	if err != nil {
		// try twice
		resp, _, err = c.Exchange(m, "127.0.0.1:"+StrPort)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(resp.Answer) == 0 || resp.Rcode != dns.RcodeSuccess {
		t.Fatal("answer expected to have A records or rcode not equal to RcodeSuccess")
	}
	// TCP
	c.Net = "tcp"
	resp, _, err = c.Exchange(m, "127.0.0.1:"+StrPort)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Answer) == 0 || resp.Rcode != dns.RcodeSuccess {
		t.Fatal("answer expected to have A records or rcode not equal to RcodeSuccess")
	}
	// disable recursion and check
	s.config.NoRec = true

	m.SetQuestion("www.example.com.", dns.TypeA)
	resp, _, err = c.Exchange(m, "127.0.0.1:"+StrPort)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Rcode != dns.RcodeServerFailure {
		t.Fatal("answer expected to have rcode equal to RcodeFailure")
	}
}

func TestDNSStubForwardV3(t *testing.T) {
	s := newTestServerV3(t, false)
	defer s.Stop()

	c := new(dns.Client)
	m := new(dns.Msg)

	stubEx := &msg.Service{
		// IP address of a.iana-servers.net.
		Host: "199.43.132.53", Key: "a.example.com.stub.dns.skydns.test.",
	}
	stubBroken := &msg.Service{
		Host: "127.0.0.1", Port: 5454, Key: "b.example.org.stub.dns.skydns.test.",
	}
	stubLoop := &msg.Service{
		Host: "127.0.0.1", Port: Port, Key: "b.example.net.stub.dns.skydns.test.",
	}
	addServiceV3(t, s, stubEx.Key, 0, stubEx)
	defer delServiceV3(t, s, stubEx.Key)
	addServiceV3(t, s, stubBroken.Key, 0, stubBroken)
	defer delServiceV3(t, s, stubBroken.Key)
	addServiceV3(t, s, stubLoop.Key, 0, stubLoop)
	defer delServiceV3(t, s, stubLoop.Key)

	s.UpdateStubZones()

	m.SetQuestion("www.example.com.", dns.TypeA)
	resp, _, err := c.Exchange(m, "127.0.0.1:"+StrPort)
	if err != nil {
		// try twice
		resp, _, err = c.Exchange(m, "127.0.0.1:"+StrPort)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(resp.Answer) == 0 || resp.Rcode != dns.RcodeSuccess {
		t.Fatal("answer expected to have A records or rcode not equal to RcodeSuccess")
	}
	// The main diff. here is that we expect the AA bit to be set, because we directly
	// queried the authoritative servers.
	if resp.Authoritative != true {
		t.Fatal("answer expected to have AA bit set")
	}

	// This should fail.
	m.SetQuestion("www.example.org.", dns.TypeA)
	resp, _, err = c.Exchange(m, "127.0.0.1:"+StrPort)
	if len(resp.Answer) != 0 || resp.Rcode != dns.RcodeServerFailure {
		t.Fatal("answer expected to fail for example.org")
	}

	// This should really fail with a timeout.
	m.SetQuestion("www.example.net.", dns.TypeA)
	resp, _, err = c.Exchange(m, "127.0.0.1:"+StrPort)
	if err == nil {
		t.Fatal("answer expected to fail for example.net")
	} else {
		t.Logf("succesfully failing %s", err)
	}

	// Packet with EDNS0
	m.SetEdns0(4096, true)
	resp, _, err = c.Exchange(m, "127.0.0.1:"+StrPort)
	if err == nil {
		t.Fatal("answer expected to fail for example.net")
	} else {
		t.Logf("succesfully failing %s", err)
	}

	// Now start another SkyDNS instance on a different port,
	// add a stubservice for it and check if the forwarding is
	// actually working.
	oldStrPort := StrPort

	s1 := newTestServerV3(t, false)
	defer s1.Stop()
	s1.config.Domain = "skydns.com."

	// Add forwarding IP for internal.skydns.com. Use Port to point to server s.
	stubForward := &msg.Service{
		Host: "127.0.0.1", Port: Port, Key: "b.internal.skydns.com.stub.dns.skydns.test.",
	}
	addServiceV3(t, s, stubForward.Key, 0, stubForward)
	defer delServiceV3(t, s, stubForward.Key)
	s.UpdateStubZones()

	// Add an answer for this in our "new" server.
	stubReply := &msg.Service{
		Host: "127.1.1.1", Key: "www.internal.skydns.com.",
	}
	addServiceV3(t, s1, stubReply.Key, 0, stubReply)
	defer delServiceV3(t, s1, stubReply.Key)

	m = new(dns.Msg)
	m.SetQuestion("www.internal.skydns.com.", dns.TypeA)
	resp, _, err = c.Exchange(m, "127.0.0.1:"+oldStrPort)
	if err != nil {
		t.Fatalf("failed to forward %s", err)
	}
	if resp.Answer[0].(*dns.A).A.String() != "127.1.1.1" {
		t.Fatalf("failed to get correct reply")
	}

	// Adding an in baliwick internal domain forward.
	s2 := newTestServerV3(t, false)
	defer s2.Stop()
	s2.config.Domain = "internal.skydns.net."

	// Add forwarding IP for internal.skydns.net. Use Port to point to server s.
	stubForward1 := &msg.Service{
		Host: "127.0.0.1", Port: Port, Key: "b.internal.skydns.net.stub.dns.skydns.test.",
	}
	addServiceV3(t, s, stubForward1.Key, 0, stubForward1)
	defer delServiceV3(t, s, stubForward1.Key)
	s.UpdateStubZones()

	// Add an answer for this in our "new" server.
	stubReply1 := &msg.Service{
		Host: "127.10.10.10", Key: "www.internal.skydns.net.",
	}
	addServiceV3(t, s2, stubReply1.Key, 0, stubReply1)
	defer delServiceV3(t, s2, stubReply1.Key)

	m = new(dns.Msg)
	m.SetQuestion("www.internal.skydns.net.", dns.TypeA)
	resp, _, err = c.Exchange(m, "127.0.0.1:"+oldStrPort)
	if err != nil {
		t.Fatalf("failed to forward %s", err)
	}
	if resp.Answer[0].(*dns.A).A.String() != "127.10.10.10" {
		t.Fatalf("failed to get correct reply")
	}
}

func TestDNSTtlRRV3(t *testing.T) {
	s := newTestServerDNSSECV3(t, false)
	defer s.Stop()

	serv := &msg.Service{Host: "10.0.0.2", Key: "ttl.skydns.test.", Ttl: 360}
	addServiceV3(t, s, serv.Key, time.Duration(serv.Ttl)*time.Second, serv)
	defer delServiceV3(t, s, serv.Key)

	c := new(dns.Client)

	tc := dnsTestCases[9] // TTL Test
	t.Logf("%v\n", tc)
	m := new(dns.Msg)
	m.SetQuestion(tc.Qname, tc.Qtype)
	if tc.dnssec == true {
		m.SetEdns0(4096, true)
	}
	resp, _, err := c.Exchange(m, "127.0.0.1:"+StrPort)
	if err != nil {
		t.Errorf("failing: %s: %s\n", m.String(), err.Error())
	}
	t.Logf("%s\n", resp)

	for i, a := range resp.Answer {
		if a.Header().Ttl != 360 {
			t.Errorf("Answer %d should have a Header TTL of %d, but has %d", i, 360, a.Header().Ttl)
		}
	}
}

//type rrSet []dns.RR
//
//func (p rrSet) Len() int           { return len(p) }
//func (p rrSet) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
//func (p rrSet) Less(i, j int) bool { return p[i].String() < p[j].String() }

func TestDNSV3(t *testing.T) {
	s := newTestServerDNSSECV3(t, false)
	defer s.Stop()

	for _, serv := range services {
		addServiceV3(t, s, serv.Key, 0, serv)
		defer delServiceV3(t, s, serv.Key)
	}
	c := new(dns.Client)
	for _, tc := range dnsTestCases {
		m := new(dns.Msg)
		m.SetQuestion(tc.Qname, tc.Qtype)
		if tc.dnssec {
			m.SetEdns0(4096, true)
		}
		if tc.chaos {
			m.Question[0].Qclass = dns.ClassCHAOS
		}
		resp, _, err := c.Exchange(m, "127.0.0.1:"+StrPort)
		if err != nil {
			// try twice, be more resilent against remote lookups
			// timing out.
			resp, _, err = c.Exchange(m, "127.0.0.1:"+StrPort)
			if err != nil {
				t.Fatalf("failing: %s: %s\n", m.String(), err.Error())
			}
		}
		sort.Sort(rrSet(resp.Answer))
		sort.Sort(rrSet(resp.Ns))
		sort.Sort(rrSet(resp.Extra))
		fatal := false
		defer func() {
			if fatal {
				t.Logf("question: %s\n", m.Question[0].String())
				t.Logf("%s\n", resp)
			}
		}()
		if resp.Rcode != tc.Rcode {
			fatal = true
			t.Fatalf("rcode is %q, expected %q", dns.RcodeToString[resp.Rcode], dns.RcodeToString[tc.Rcode])
		}
		if len(resp.Answer) != len(tc.Answer) {
			fatal = true
			t.Fatalf("answer for %q contained %d results, %d expected", tc.Qname, len(resp.Answer), len(tc.Answer))
		}
		for i, a := range resp.Answer {
			if a.Header().Name != tc.Answer[i].Header().Name {
				fatal = true
				t.Fatalf("answer %d should have a Header Name of %q, but has %q", i, tc.Answer[i].Header().Name, a.Header().Name)
			}
			if a.Header().Ttl != tc.Answer[i].Header().Ttl {
				fatal = true
				t.Fatalf("Answer %d should have a Header TTL of %d, but has %d", i, tc.Answer[i].Header().Ttl, a.Header().Ttl)
			}
			if a.Header().Rrtype != tc.Answer[i].Header().Rrtype {
				fatal = true
				t.Fatalf("answer %d should have a header response type of %d, but has %d", i, tc.Answer[i].Header().Rrtype, a.Header().Rrtype)
			}
			switch x := a.(type) {
			case *dns.SRV:
				if x.Priority != tc.Answer[i].(*dns.SRV).Priority {
					fatal = true
					t.Fatalf("answer %d should have a Priority of %d, but has %d", i, tc.Answer[i].(*dns.SRV).Priority, x.Priority)
				}
				if x.Weight != tc.Answer[i].(*dns.SRV).Weight {
					fatal = true
					t.Fatalf("answer %d should have a Weight of %d, but has %d", i, tc.Answer[i].(*dns.SRV).Weight, x.Weight)
				}
				if x.Port != tc.Answer[i].(*dns.SRV).Port {
					fatal = true
					t.Fatalf("answer %d should have a Port of %d, but has %d", i, tc.Answer[i].(*dns.SRV).Port, x.Port)
				}
				if x.Target != tc.Answer[i].(*dns.SRV).Target {
					fatal = true
					t.Fatalf("answer %d should have a Target of %q, but has %q", i, tc.Answer[i].(*dns.SRV).Target, x.Target)
				}
			case *dns.A:
				if x.A.String() != tc.Answer[i].(*dns.A).A.String() {
					fatal = true
					t.Fatalf("answer %d should have a Address of %q, but has %q", i, tc.Answer[i].(*dns.A).A.String(), x.A.String())
				}
			case *dns.AAAA:
				if x.AAAA.String() != tc.Answer[i].(*dns.AAAA).AAAA.String() {
					fatal = true
					t.Fatalf("answer %d should have a Address of %q, but has %q", i, tc.Answer[i].(*dns.AAAA).AAAA.String(), x.AAAA.String())
				}
			case *dns.TXT:
				for j, txt := range x.Txt {
					if txt != tc.Answer[i].(*dns.TXT).Txt[j] {
						fatal = true
						t.Fatalf("answer %d should have a Txt of %q, but has %q", i, tc.Answer[i].(*dns.TXT).Txt[j], txt)
					}
				}
			case *dns.DNSKEY:
				tt := tc.Answer[i].(*dns.DNSKEY)
				if x.Flags != tt.Flags {
					fatal = true
					t.Fatalf("DNSKEY flags should be %q, but is %q", x.Flags, tt.Flags)
				}
				if x.Protocol != tt.Protocol {
					fatal = true
					t.Fatalf("DNSKEY protocol should be %q, but is %q", x.Protocol, tt.Protocol)
				}
				if x.Algorithm != tt.Algorithm {
					fatal = true
					t.Fatalf("DNSKEY algorithm should be %q, but is %q", x.Algorithm, tt.Algorithm)
				}
			case *dns.RRSIG:
				tt := tc.Answer[i].(*dns.RRSIG)
				if x.TypeCovered != tt.TypeCovered {
					fatal = true
					t.Fatalf("RRSIG type-covered should be %d, but is %d", x.TypeCovered, tt.TypeCovered)
				}
				if x.Algorithm != tt.Algorithm {
					fatal = true
					t.Fatalf("RRSIG algorithm should be %d, but is %d", x.Algorithm, tt.Algorithm)
				}
				if x.Labels != tt.Labels {
					fatal = true
					t.Fatalf("RRSIG label should be %d, but is %d", x.Labels, tt.Labels)
				}
				if x.OrigTtl != tt.OrigTtl {
					fatal = true
					t.Fatalf("RRSIG orig-ttl should be %d, but is %d", x.OrigTtl, tt.OrigTtl)
				}
				if x.KeyTag != tt.KeyTag {
					fatal = true
					t.Fatalf("RRSIG key-tag should be %d, but is %d", x.KeyTag, tt.KeyTag)
				}
				if x.SignerName != tt.SignerName {
					fatal = true
					t.Fatalf("RRSIG signer-name should be %q, but is %q", x.SignerName, tt.SignerName)
				}
			case *dns.SOA:
				tt := tc.Answer[i].(*dns.SOA)
				if x.Ns != tt.Ns {
					fatal = true
					t.Fatalf("SOA nameserver should be %q, but is %q", x.Ns, tt.Ns)
				}
			case *dns.PTR:
				tt := tc.Answer[i].(*dns.PTR)
				if x.Ptr != tt.Ptr {
					fatal = true
					t.Fatalf("PTR ptr should be %q, but is %q", x.Ptr, tt.Ptr)
				}
			case *dns.CNAME:
				tt := tc.Answer[i].(*dns.CNAME)
				if x.Target != tt.Target {
					fatal = true
					t.Fatalf("CNAME target should be %q, but is %q", x.Target, tt.Target)
				}
			case *dns.MX:
				tt := tc.Answer[i].(*dns.MX)
				if x.Mx != tt.Mx {
					t.Fatalf("MX Mx should be %q, but is %q", x.Mx, tt.Mx)
				}
				if x.Preference != tt.Preference {
					t.Fatalf("MX Preference should be %q, but is %q", x.Preference, tt.Preference)
				}
			}
		}
		if len(resp.Ns) != len(tc.Ns) {
			fatal = true
			t.Fatalf("authority for %q contained %d results, %d expected", tc.Qname, len(resp.Ns), len(tc.Ns))
		}
		for i, n := range resp.Ns {
			switch x := n.(type) {
			case *dns.SOA:
				tt := tc.Ns[i].(*dns.SOA)
				if x.Ns != tt.Ns {
					fatal = true
					t.Fatalf("SOA nameserver should be %q, but is %q", x.Ns, tt.Ns)
				}
			case *dns.NS:
				tt := tc.Ns[i].(*dns.NS)
				if x.Ns != tt.Ns {
					fatal = true
					t.Fatalf("NS nameserver should be %q, but is %q", x.Ns, tt.Ns)
				}
			case *dns.NSEC3:
				tt := tc.Ns[i].(*dns.NSEC3)
				if x.NextDomain != tt.NextDomain {
					fatal = true
					t.Fatalf("NSEC3 nextdomain should be %q, but is %q", x.NextDomain, tt.NextDomain)
				}
				if x.Hdr.Name != tt.Hdr.Name {
					fatal = true
					t.Fatalf("NSEC3 ownername should be %q, but is %q", x.Hdr.Name, tt.Hdr.Name)
				}
				for j, y := range x.TypeBitMap {
					if y != tt.TypeBitMap[j] {
						fatal = true
						t.Fatalf("NSEC3 bitmap should have %q, but is %q", dns.TypeToString[y], dns.TypeToString[tt.TypeBitMap[j]])
					}
				}
			}
		}
		if len(resp.Extra) != len(tc.Extra) {
			fatal = true
			t.Fatalf("additional for %q contained %d results, %d expected", tc.Qname, len(resp.Extra), len(tc.Extra))
		}
		for i, e := range resp.Extra {
			switch x := e.(type) {
			case *dns.A:
				if x.A.String() != tc.Extra[i].(*dns.A).A.String() {
					fatal = true
					t.Fatalf("extra %d should have a address of %q, but has %q", i, tc.Extra[i].(*dns.A).A.String(), x.A.String())
				}
			case *dns.AAAA:
				if x.AAAA.String() != tc.Extra[i].(*dns.AAAA).AAAA.String() {
					fatal = true
					t.Fatalf("extra %d should have a address of %q, but has %q", i, tc.Extra[i].(*dns.AAAA).AAAA.String(), x.AAAA.String())
				}
			case *dns.CNAME:
				tt := tc.Extra[i].(*dns.CNAME)
				if x.Target != tt.Target {
					// Super super gross hack.
					if x.Target == "a.ipaddr.skydns.test." && tt.Target == "b.ipaddr.skydns.test." {
						// These records are randomly choosen, either one is OK.
						continue
					}
					fatal = true
					t.Fatalf("CNAME target should be %q, but is %q", x.Target, tt.Target)
				}
			}
		}
	}
}

func TestTargetStripAdditionalV3(t *testing.T) {
	s := newTestServerV3(t, false)
	defer s.Stop()

	c := new(dns.Client)
	m := new(dns.Msg)

	pre := "bliep."
	expected := "blaat.skydns.test."
	serv := &msg.Service{
		Host: "199.43.132.53", Key: pre + expected, TargetStrip: 1, Text: "Text",
	}
	addServiceV3(t, s, serv.Key, 0, serv)
	defer delServiceV3(t, s, serv.Key)

	m.SetQuestion(pre+expected, dns.TypeSRV)
	resp, _, err := c.Exchange(m, "127.0.0.1:"+StrPort)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s", resp.String())
	if resp.Extra[0].Header().Name != expected {
		t.Fatalf("expected %s, got %s for SRV with v4", expected, resp.Extra[0].Header().Name)
	}

	serv = &msg.Service{
		Host: "2001::1", Key: pre + expected, TargetStrip: 1, Text: "Text",
	}
	delServiceV3(t, s, serv.Key)
	// previous defer still stands
	addServiceV3(t, s, serv.Key, 0, serv)

	m.SetQuestion(pre+expected, dns.TypeSRV)
	resp, _, err = c.Exchange(m, "127.0.0.1:"+StrPort)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s", resp.String())
	if resp.Extra[0].Header().Name != expected {
		t.Fatalf("expected %s, got %s for SRV with v6", expected, resp.Extra[0].Header().Name)
	}
}

func TestMsgOverflowV3(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	s := newTestServerV3(t, false)
	defer s.Stop()

	c := new(dns.Client)
	m := new(dns.Msg)

	for i := 0; i < 2000; i++ {
		is := strconv.Itoa(i)
		m := &msg.Service{
			Host: "2001::" + is, Key: "machine" + is + ".machines.skydns.test.",
		}
		addServiceV3(t, s, m.Key, 0, m)
		defer delServiceV3(t, s, m.Key)
	}
	m.SetQuestion("machines.skydns.test.", dns.TypeSRV)
	resp, _, err := c.Exchange(m, "127.0.0.1:"+StrPort)
	if err != nil {
		// Unpack can fail, and it should (i.e. msg too large)
		t.Logf("%s", err)
		return
	}
	t.Logf("%s", resp)

	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("expecting server failure, got %d", resp.Rcode)
	}
}

func BenchmarkDNSSingleCacheV3(b *testing.B) {
	b.StopTimer()
	t := new(testing.T)
	s := newTestServerDNSSECV3(t, true)
	defer s.Stop()

	serv := services[0]
	addServiceV3(t, s, serv.Key, 0, serv)
	defer delServiceV3(t, s, serv.Key)

	c := new(dns.Client)
	tc := dnsTestCases[0]
	m := new(dns.Msg)
	m.SetQuestion(tc.Qname, tc.Qtype)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		c.Exchange(m, "127.0.0.1:"+StrPort)
	}
}

func BenchmarkDNSWildcardCacheV3(b *testing.B) {
	b.StopTimer()
	t := new(testing.T)
	s := newTestServerDNSSECV3(t, true)
	defer s.Stop()

	for _, serv := range services {
		m := &msg.Service{Host: serv.Host, Port: serv.Port}
		addServiceV3(t, s, serv.Key, 0, m)
		defer delServiceV3(t, s, serv.Key)
	}

	c := new(dns.Client)
	tc := dnsTestCases[8] // Wildcard Test
	m := new(dns.Msg)
	m.SetQuestion(tc.Qname, tc.Qtype)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		c.Exchange(m, "127.0.0.1:"+StrPort)
	}
}

func BenchmarkDNSSECSingleCacheV3(b *testing.B) {
	b.StopTimer()
	t := new(testing.T)
	s := newTestServerDNSSECV3(t, true)
	defer s.Stop()

	serv := services[0]
	addServiceV3(t, s, serv.Key, 0, serv)
	defer delServiceV3(t, s, serv.Key)

	c := new(dns.Client)
	tc := dnsTestCases[0]
	m := new(dns.Msg)
	m.SetQuestion(tc.Qname, tc.Qtype)
	m.SetEdns0(4096, true)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		c.Exchange(m, "127.0.0.1:"+StrPort)
	}
}

func BenchmarkDNSSingleNoCacheV3(b *testing.B) {
	b.StopTimer()
	t := new(testing.T)
	s := newTestServerDNSSECV3(t, false)
	defer s.Stop()

	serv := services[0]
	addServiceV3(t, s, serv.Key, 0, serv)
	defer delServiceV3(t, s, serv.Key)

	c := new(dns.Client)
	tc := dnsTestCases[0]
	m := new(dns.Msg)
	m.SetQuestion(tc.Qname, tc.Qtype)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		c.Exchange(m, "127.0.0.1:"+StrPort)
	}
}

func BenchmarkDNSWildcardNoCacheV3(b *testing.B) {
	b.StopTimer()
	t := new(testing.T)
	s := newTestServerDNSSECV3(t, false)
	defer s.Stop()

	for _, serv := range services {
		m := &msg.Service{Host: serv.Host, Port: serv.Port}
		addServiceV3(t, s, serv.Key, 0, m)
		defer delServiceV3(t, s, serv.Key)
	}

	c := new(dns.Client)
	tc := dnsTestCases[8] // Wildcard Test
	m := new(dns.Msg)
	m.SetQuestion(tc.Qname, tc.Qtype)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		c.Exchange(m, "127.0.0.1:"+StrPort)
	}
}

func BenchmarkDNSSECSingleNoCacheV3(b *testing.B) {
	b.StopTimer()
	t := new(testing.T)
	s := newTestServerDNSSECV3(t, false)
	defer s.Stop()

	serv := services[0]
	addServiceV3(t, s, serv.Key, 0, serv)
	defer delServiceV3(t, s, serv.Key)

	c := new(dns.Client)
	tc := dnsTestCases[0]
	m := new(dns.Msg)
	m.SetQuestion(tc.Qname, tc.Qtype)
	m.SetEdns0(4096, true)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		c.Exchange(m, "127.0.0.1:"+StrPort)
	}
}
