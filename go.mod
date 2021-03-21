module github.com/skynetservices/skydns

go 1.16

replace github.com/coreos/bbolt => go.etcd.io/bbolt v1.3.5

replace go.etcd.io/bbolt => github.com/coreos/bbolt v1.3.5

require (
	github.com/coreos/bbolt v0.0.0-00010101000000-000000000000 // indirect
	github.com/coreos/etcd v3.3.25+incompatible
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf
	github.com/miekg/dns v1.1.41
	github.com/prometheus/client_golang v1.10.0
	golang.org/x/net v0.0.0-20210316092652-d523dce5a7f4
)
