FROM alpine:edge
MAINTAINER Miek Gieben <miek@miek.nl> (@miekg)

RUN apk --update add bind-tools && rm -rf /var/cache/apk/*
RUN apk --update add git mercurial go build-base && rm -rf /var/cache/apk/*
RUN mkdir /go
RUN export GOPATH="/go" && go get github.com/skynetservices/skydns && cd $GOPATH/src/github.com/skynetservices/skydns && go build -v
RUN mv /go/bin/skydns /bin
RUN rm -rvf /go

RUN apk del git mercurial go build-base

EXPOSE 53 53/udp
ENTRYPOINT ["/bin/skydns"]
