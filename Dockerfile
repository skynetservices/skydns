FROM golang:1.4-cross

RUN go get \
    github.com/coreos/etcd \
    github.com/coreos/go-etcd/etcd \
    github.com/coreos/go-systemd/activation \
    github.com/miekg/dns \
    github.com/rcrowley/go-metrics \
    github.com/rcrowley/go-metrics/stathat

WORKDIR /go/src/github.com/skynetservices/skydns
ADD . /go/src/github.com/skynetservices/skydns
