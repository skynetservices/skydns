// Copyright (c) 2014 The SkyDNS Authors. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	backendetcd "github.com/skynetservices/skydns/backends/etcd"
	backendetcdv3 "github.com/nokia/skydns/backends/etcd3"
	"github.com/skynetservices/skydns/metrics"
	"github.com/skynetservices/skydns/msg"
	//"github.com/skynetservices/skydns/server"
	"github.com/nokia/skydns/server"

	etcd "github.com/coreos/etcd/client"
	etcdv3 "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
)

var (
	tlskey     = ""
	tlspem     = ""
	cacert     = ""
	username   = ""
	password   = ""
	config     = &server.Config{ReadTimeout: 0, Domain: "", DnsAddr: "", DNSSEC: ""}
	nameserver = ""
	machine    = ""
	stub       = false
	ctx        = context.Background()
)

func env(key, def string) string {
	if x := os.Getenv(key); x != "" {
		return x
	}
	return def
}

func intEnv(key string, def int) int {
	if x := os.Getenv(key); x != "" {
		if v, err := strconv.ParseInt(x, 10, 0); err == nil {
			return int(v)
		}
	}
	return def
}

func boolEnv(key string, def bool) bool {
	if x := os.Getenv(key); x != "" {
		if v, err := strconv.ParseBool(x); err == nil {
			return v
		}
	}
	return def
}

func init() {
	flag.StringVar(&config.Domain, "domain", env("SKYDNS_DOMAIN", "skydns.local."), "domain to anchor requests to (SKYDNS_DOMAIN)")
	flag.StringVar(&config.DnsAddr, "addr", env("SKYDNS_ADDR", "127.0.0.1:53"), "ip:port to bind to (SKYDNS_ADDR)")
	flag.StringVar(&nameserver, "nameservers", env("SKYDNS_NAMESERVERS", ""), "nameserver address(es) to forward (non-local) queries to e.g. 8.8.8.8:53,8.8.4.4:53")
	flag.BoolVar(&config.NoRec, "no-rec", false, "do not provide a recursive service")
	flag.StringVar(&machine, "machines", env("ETCD_MACHINES", "http://127.0.0.1:2379"), "machine address(es) running etcd")
	flag.StringVar(&config.DNSSEC, "dnssec", "", "basename of DNSSEC key file e.q. Kskydns.local.+005+38250")
	flag.StringVar(&config.Local, "local", "", "optional unique value for this skydns instance")
	flag.StringVar(&tlskey, "tls-key", env("ETCD_TLSKEY", ""), "SSL key file used to secure etcd communication")
	flag.StringVar(&tlspem, "tls-pem", env("ETCD_TLSPEM", ""), "SSL certification file used to secure etcd communication")
	flag.StringVar(&cacert, "ca-cert", env("ETCD_CACERT", ""), "SSL Certificate Authority file used to secure etcd communication")
	flag.StringVar(&username, "username", env("ETCD_USERNAME", ""), "Username used to support etcd basic auth")
	flag.StringVar(&password, "password", env("ETCD_PASSWORD", ""), "Password used to support etcd basic auth")
	flag.DurationVar(&config.ReadTimeout, "rtimeout", 2*time.Second, "read timeout")
	flag.BoolVar(&config.RoundRobin, "round-robin", true, "round robin A/AAAA replies")
	flag.BoolVar(&config.NSRotate, "ns-rotate", true, "round robin selection of nameservers from among those listed")
	flag.BoolVar(&stub, "stubzones", false, "support stub zones")
	flag.BoolVar(&config.Verbose, "verbose", false, "log queries")
	flag.BoolVar(&config.Systemd, "systemd", boolEnv("SKYDNS_SYSTEMD", false), "bind to socket(s) activated by systemd (ignore -addr)")

	// Version
	flag.BoolVar(&config.Version, "version", false, "Print the version and exit.")

	// TTl
	// Minttl
	flag.StringVar(&config.Hostmaster, "hostmaster", "hostmaster@skydns.local.", "hostmaster email address to use")
	flag.IntVar(&config.SCache, "scache", server.SCacheCapacity, "capacity of the signature cache")
	flag.IntVar(&config.RCache, "rcache", 0, "capacity of the response cache") // default to 0 for now
	flag.IntVar(&config.RCacheTtl, "rcache-ttl", server.RCacheTtl, "TTL of the response cache")

	// Ndots
	flag.IntVar(&config.Ndots, "ndots", intEnv("SKYDNS_NDOTS", server.Ndots), "How many labels a name should have before we allow forwarding")

	flag.StringVar(&msg.PathPrefix, "path-prefix", env("SKYDNS_PATH_PREFIX", "skydns"), "backend(etcd) path prefix, default: skydns")

	flag.BoolVar(&config.Etcd3, "etcd3", false, "flag that denotes the etcd version to be supported by skydns during runtime. Defaults to false.")
}

func main() {
	fmt.Printf("FBDL with etcd3 flag version\n")
	flag.Parse()

	if config.Version {
		fmt.Printf("skydns server version: %s\n", server.Version)
		os.Exit(0)
	}

	machines := strings.Split(machine, ",")
	//client, err := newEtcdClient(machines, tlspem, tlskey, cacert, username, password)

	//TODO: FBDL can be refactored further
	var clientptr *etcdv3.Client
	var err error
	var clientv3 etcdv3.Client
	var clientv2 etcd.KeysAPI

	if config.Etcd3 {
		fmt.Printf("Creating new etcdv3 client\n")
		clientptr, err = newEtcdV3Client(machines, tlspem, tlskey, cacert)
		clientv3 = *clientptr
	} else {
		fmt.Printf("Creating new etcdv2 client\n")
		clientv2, err = newEtcdV2Client(machines, tlspem, tlskey, cacert, username, password)
	}

	if err != nil {
		panic(err)
	}

	if nameserver != "" {
		for _, hostPort := range strings.Split(nameserver, ",") {
			if err := validateHostPort(hostPort); err != nil {
				log.Fatalf("skydns: nameserver is invalid: %s", err)
			}
			config.Nameservers = append(config.Nameservers, hostPort)
		}
	}
	if err := validateHostPort(config.DnsAddr); err != nil {
		log.Fatalf("skydns: addr is invalid: %s", err)
	}

	//if err := loadConfig(client, config); err != nil {
	//	log.Fatalf("skydns: %s", err)
	//}
	if config.Etcd3 {
		fmt.Printf("Loading v3 config\n")
		if err := loadEtcdV3Config(clientv3, config); err != nil {
			log.Fatalf("skydns: %s", err)
		}
	} else {
		fmt.Printf("Loading v2 config\n")
		if err := loadEtcdV2Config(clientv2, config); err != nil {
			log.Fatalf("skydns: %s", err)
		}
	}


	if err := server.SetDefaults(config); err != nil {
		log.Fatalf("skydns: defaults could not be set from /etc/resolv.conf: %v", err)
	}

	if config.Local != "" {
		config.Local = dns.Fqdn(config.Local)
	}


	//backend := backendetcd.NewBackend(client, ctx, &backendetcd.Config{
	//	Ttl:      config.Ttl,
	//	Priority: config.Priority,
	//})
	var backend server.Backend
	if config.Etcd3 {
		fmt.Printf("Establish v3 backend\n")
		backend = backendetcdv3.NewBackendv3(clientv3, ctx, &backendetcdv3.Config{
			Ttl: config.Ttl,
			Priority: config.Priority,
		})
	} else {
		fmt.Printf("Establish v2 backend\n")
		backend = backendetcd.NewBackend(clientv2, ctx, &backendetcd.Config{
			Ttl: config.Ttl,
			Priority: config.Priority,
		})
	}

	s := server.New(backend, config)
	if stub {
		s.UpdateStubZones()
		go func() {
			duration := 1 * time.Second

			if config.Etcd3 {
				fmt.Printf("Doing v3 watcher\n")
				var watcher etcdv3.WatchChan
				watcher = clientv3.Watch(ctx, msg.Path(config.Domain) + "/dns/stub/", etcdv3.WithPrefix())

				for wresp := range watcher {
					if wresp.Err() != nil {
						log.Printf("skydns: stubzone update failed, sleeping %s + ~3s", duration)
						time.Sleep(duration + (time.Duration(rand.Float32() * 3e9)))
						duration *= 2
						if duration > 32 * time.Second {
							duration = 32 * time.Second
						}
					} else {
						s.UpdateStubZones()
						log.Printf("skydns: stubzone update")
						duration = 1 * time.Second //reset
					}
				}
			} else {
				fmt.Printf("Doing v2 watcher\n")
				var watcher etcd.Watcher

				watcher = clientv2.Watcher(msg.Path(config.Domain)+"/dns/stub/", &etcd.WatcherOptions{AfterIndex: 0, Recursive: true})

				for {
					_, err := watcher.Next(ctx)

					if err != nil {
						//
						log.Printf("skydns: stubzone update failed, sleeping %s + ~3s", duration)
						time.Sleep(duration + (time.Duration(rand.Float32() * 3e9))) // Add some random.
						duration *= 2
						if duration > 32*time.Second {
							duration = 32 * time.Second
						}
					} else {
						s.UpdateStubZones()
						log.Printf("skydns: stubzone update")
						duration = 1 * time.Second // reset
					}
				}
			}
		}()
	}

	fmt.Printf("Launching metrics\n")
	if err := metrics.Metrics(); err != nil {
		log.Fatalf("skydns: %s", err)
	} else {
		log.Printf("skydns: metrics enabled on :%s%s", metrics.Port, metrics.Path)
	}

	fmt.Printf("Invoking run\n")
	if err := s.Run(); err != nil {
		log.Fatalf("skydns: %s", err)
	}
}

func loadEtcdV2Config(client etcd.KeysAPI, config *server.Config) error {
	// Override what isn't set yet from the command line.
	configPath := "/" + msg.PathPrefix + "/config"
	resp, err := client.Get(ctx, configPath, nil)
	if err != nil {
		log.Printf("skydns: falling back to default configuration, could not read from etcd: %s", err)
		return nil
	}
	if err := json.Unmarshal([]byte(resp.Node.Value), config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %s", err.Error())
	}
	return nil
}

//TODO: FBDL for refactoring maybe to some other file yung mga v3
func loadEtcdV3Config(client etcdv3.Client, config *server.Config) error {
	configPath := "/" + msg.PathPrefix + "/config"
	resp, err := client.Get(ctx, configPath)
	if err != nil {
		log.Printf("skydns: falling back to default configuration, could not read from etcd: %s", err)
		return nil
	}
	for _, ev := range resp.Kvs {
		if err := json.Unmarshal([]byte(ev.Value), config); err != nil {
			return fmt.Errorf("failed to unmarshal config: %s", err.Error())
		}
	}
	return nil
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

func newEtcdV2Client(machines []string, certFile, keyFile, caFile, username, password string) (etcd.KeysAPI, error) {
	t, err := newHTTPSTransport(certFile, keyFile, caFile)
	if err != nil {
		return nil, err
	}

	cli, err := etcd.New(etcd.Config{
		Endpoints: machines,
		Transport: t,
		Username:  username,
		Password:  password,
	})
	if err != nil {
		return nil, err
	}
	return etcd.NewKeysAPI(cli), nil
}

//TODO: FBDL for refactoring
func newEtcdV3Client(machines []string, tlsCert, tlsKey, tlsCACert string) (*etcdv3.Client, error) {

	etcdCfg := etcdv3.Config {
		Endpoints: machines,
		TLS: newTlsConfig(tlsCert, tlsKey, tlsCACert),
	}
	cli, err := etcdv3.New(etcdCfg)
	if err != nil {
		return nil, err
	}
	return cli, nil
}

//TODO: FBDL for refactoring
func newTlsConfig(tlsCertFile, tlsKeyFile, tlsCACertFile string) *tls.Config {

	var cc *tls.Config = nil

	if tlsCertFile != "" && tlsKeyFile != "" {
		var rpool *x509.CertPool
		if tlsCACertFile != "" {
			if pemBytes, err := ioutil.ReadFile(tlsCACertFile); err == nil {
				rpool = x509.NewCertPool()
				rpool.AppendCertsFromPEM(pemBytes)
			}
		}

		if tlsCert, err := tls.LoadX509KeyPair(tlsCertFile, tlsKeyFile); err == nil {
			cc = &tls.Config {
				RootCAs: rpool,
				Certificates: []tls.Certificate{tlsCert},
				InsecureSkipVerify: true,
			}
		}
	}
	return cc
}

func newHTTPSTransport(certFile, keyFile, caFile string) (*http.Transport, error) {
	info := transport.TLSInfo{
		CertFile: certFile,
		KeyFile:  keyFile,
		CAFile:   caFile,
	}
	cfg, err := info.ClientConfig()
	if err != nil {
		return nil, err
	}

	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     cfg,
	}

	return tr, nil
}
