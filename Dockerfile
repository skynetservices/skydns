FROM crosbymichael/golang
MAINTAINER Miek Gieben <miek@miek.nl> (@miekg)

ADD . /go/src/github.com/skynetservices/skydns
RUN go get github.com/skynetservices/skydns

EXPOSE 53 53/udp
ENTRYPOINT ["skydns"]
