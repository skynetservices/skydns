#!/bin/bash
set -e

# Get rid of existing binaries
echo "Remove old binaries..."
rm -f ./docker-builder/dockerimage-skydns*/skydns_*

# Build Docker image unless we opt out of it
if [[ -z "$SKIP_BUILD" ]]; then
	echo "Build the builder docker image..."
    docker build -t skydns-builder .
fi

# Let's crosscompile and produce static GO binaries
echo "Crosscompile..."
docker run --rm -v `pwd`:/go/src/github.com/skynetservices/skydns skydns-builder bash -c 'GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -installsuffix nocgo -o docker-builder/dockerimage-skydns/skydns_linux-amd64'
docker run --rm -v `pwd`:/go/src/github.com/skynetservices/skydns skydns-builder bash -c 'GOOS=linux GOARCH=arm CGO_ENABLED=0 go build -a -installsuffix nocgo -o docker-builder/dockerimage-skydns-armv6l/skydns_linux-arm'
ls -al docker-builder/dockerimage-skydns*/skydns_linux-*

# Now build minimized Docker images
echo "Build Docker Images..."
docker build -t skydns ./docker-builder/dockerimage-skydns/
docker build -t skydns-armv6l ./docker-builder/dockerimage-skydns-armv6l/
docker images | grep skydns
