FROM crosbymichael/golang
MAINTAINER Miek Gieben <miek@miek.nl> (@miekg)

RUN apt-get update && apt-get install --no-install-recommends -y \
    dnsutils

RUN go get github.com/tools/godep
WORKDIR /go/src/github.com/skynetservices/skydns

ADD . /go/src/github.com/skynetservices/skydns

RUN godep get
RUN godep go build -v

EXPOSE 53
ENTRYPOINT ["skydns"]
