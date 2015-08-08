FROM alpine:edge
MAINTAINER Miek Gieben <miek@miek.nl> (@miekg)

RUN apk --update add bind-tools && rm -rf /var/cache/apk/*
RUN apk --update add git mercurial go build-base && rm -rf /var/cache/apk/* && mkdir /go && export GOPATH="/go" && go get github.com/skynetservices/skydns && cd $GOPATH/src/github.com/skynetservices/skydns && go build -v && mv /go/bin/skydns /bin && rm -rvf /go && apk del --purge git mercurial go build-base

EXPOSE 53 53/udp
ENTRYPOINT ["/bin/skydns"]
