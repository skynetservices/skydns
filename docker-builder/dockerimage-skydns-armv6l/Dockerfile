FROM scratch
MAINTAINER Miek Gieben <miek@miek.nl> (@miekg)

ADD skydns_linux-arm /skydns

EXPOSE 53 53/udp
ENTRYPOINT ["/skydns"]
