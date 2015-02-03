# SkyDNS [![Build Status](https://travis-ci.org/skynetservices/skydns.png?branch=master)](https://travis-ci.org/skynetservices/skydns)
*Version 2.0.1d*

SkyDNS is a distributed service for announcement and discovery of services built on
top of [etcd](https://github.com/coreos/etcd). It utilizes DNS queries
to discover available services. This is done by leveraging SRV records in DNS,
with special meaning given to subdomains, priorities and weights.

This is the original [announcement blog post](http://blog.gopheracademy.com/skydns) for version 1.
Since then, SkyDNS has seen some changes, most notably the ability to use etcd as a backend.
[Here you can find the SkyDNS2 announcement](http://miek.nl/posts/2014/Jun/08/announcing%20SkyDNS%20version%202/).

# Changes since version 1

SkyDNS2:

* Does away with Raft and uses etcd (which uses raft).
* Makes is possible to query arbitrary domain names.
* Is a thin layer above etcd, that translates etcd keys and values to the DNS.
* Does DNSSEC with NSEC3 instead of NSEC.

Note that bugs in SkyDNS1 will still be fixed, but the main development effort will be focussed on version 2.
[Version 1 of SkyDNS can be found here](https://github.com/skynetservices/skydns1).

# Future ideas

* Abstract away the backend in an interface, so different backends can be used.

## Setup / Install
Download/compile and run etcd. See the documentation for etcd at <https://github.com/coreos/etcd>.

Install godep:

    go get github.com/tools/godep

Then get and compile SkyDNS:

    cd $GOPATH/src/github.com/skynetservices/skydns
    godep restore
    go build -v

Or:

    godep get github.com/skynetservices/skydns

SkyDNS' configuration is stored *in* etcd: but there are also flags and environment variabes
you can set. To start SkyDNS, set the etcd machines with the environment variable ETCD_MACHINES:

    export ETCD_MACHINES='http://192.168.0.1:4001,http://192.168.0.2:4001'
    ./skydns

If `ETCD_MACHINES` is not set, SkyDNS will default to using `http://127.0.0.1:4001` to connect to etcd.
Or you can use the flag `-machines`. Auto-discovering new machines added to the network can
be enabled by enabling the flag `-discover`.

Optionally (but recommended) give it a nameserver:

    curl -XPUT http://127.0.0.1:4001/v2/keys/skydns/local/skydns/dns/ns \
        -d value='{"host":"192.168.0.1"}'

Also see the section "NS Records".

## Configuration
SkyDNS' configuration is stored in etcd as a JSON object under the key `/skydns/config`. The following parameters
may be set:

* `dns_addr`: IP:port on which SkyDNS should listen, defaults to `127.0.0.1:53`.
* `domain`: domain for which SkyDNS is authoritative, defaults to `skydns.local.`.
* `dnssec`: enable DNSSEC
* `hostmaster`: hostmaster email address to use.
* `local`: optional unique value for this skydns instance, default is none. This is returned
    when queried for `local.dns.skydns.local`.
* `round_robin`: enable round-robin sorting for A and AAAA responses, defaults to true.
* `nameservers`: forward DNS requests to these nameservers (array of IP:port combination), when not
    authoritative for a domain.
* `read_timeout`: network read timeout, for DNS and talking with etcd.
* `ttl`: default TTL in seconds to use on replies when none is set in etcd, defaults to 3600.
* `min_ttl`: minimum TTL in seconds to use on NXDOMAIN, defaults to 30.
* `scache`: the capacity of the DNSSEC signature cache, defaults to 10000 records if not set.
* `rcache`: the capacity of the response cache, defaults to 0 records if not set.
* `rcache_ttl`: the TTL of the response cache, defaults to 60 if not set.
* `systemd`: bind to socket(s) activated by systemd (ignores -addr).

To set the configuration, use something like:

    curl -XPUT http://127.0.0.1:4001/v2/keys/skydns/config \
        -d value='{"dns_addr":"127.0.0.1:5354","ttl":3600, "nameservers": ["8.8.8.8:53","8.8.4.4:53"]}'

SkyDNS needs to be restarted for configuration changes to take effect. This might change, so that SkyDNS
can re-read the config from etcd after a HUP signal.

You can also use the command line options, however the settings in etcd take precedence.

### Commandline flags

* `-addr`: used to specify the address to listen on (note: this will be changed into `-dns_addr` to match the json.
* `-local`: used to specify a unique service for this SkyDNS instance. This should point to a (unique) domain into etcd, when
    SkyDNS receives a query for the name `local.dns.skydns.local` it will fetch this service and return it.
    For instance: `-local e2016c14-fbba-11e3-ae08-10604b7efbe2.dockerhosts.skydns.local` and then 

        curl -XPUT http://127.0.0.1:4001/v2/keys/skydns/local/skydns/dockerhosts/2016c14-fbba-11e3-ae08-10604b7efbe2 \
            -d value='{"host":"10.1.1.16"}'

    To register the local IP address. Now when SkyDNS receives a query for local.dns.skydns.local it will fetch the above
    key and returns that one service. In other words skydns will substitute `e2016c14-fbba-11e3-ae08-10604b7efbe2.dockerhosts.skydns.local`
    for `local.dns.skydns.local`. This follows the same rules as the other services, so it can also be an external names, which
    will be resolved.

    Also see the section Host Local Values.

### Environment Variables

SkyDNS uses these environment variables:

* `ETCD_MACHINES` - list of etcd machines, "http://localhost:4001,http://etcd.example.com:4001".
* `ETCD_TLSKEY` - path of TLS client certificate - private key.
* `ETCD_TLSPEM` - path of TLS client certificate - public key.
* `ETCD_CACERT` - path of TLS certificate authority public key
* `SKYDNS_ADDR` - specify address to bind to
* `SKYDNS_DOMAIN` - set a default domain if not specified by etcd config
* `SKYDNS_NAMESERVERS` - set a list of nameservers to forward DNS requests to when not authoritative for a domain, "8.8.8.8:53,8.8.4.4:53".

And these are used for statistics:

* `GRAPHITE_SERVER`
* `GRAPHITE_PREFIX`
* `STATHAT_USER`

### SSL Usage and Authentication with Client Certificates

In order to connect to an SSL-secured etcd, you will at least need to set ETCD_CACERT to be the public key
of the Certificate Authority which signed the server certificate.

If the SSL-secured etcd expects client certificates to authorize connections, you also need to set ETCD_TLSKEY
to the *private* key of the client, and ETCD_TLSPEM to the *public* key of the client.

## Service Announcements
Announce your service by submitting JSON over HTTP to etcd with information about your service.
This information will then be available for queries via DNS.
We use the directory `/skydns` to anchor all names.

When providing information you will need to fill out (some of) the following values.

* Path - The path of the key in etcd, e.g. if the domain you want to register is "rails.production.east.skydns.local", you need to reverse it and replace the dots with slashes. So the name here becomes:
    `local/skydns/east/production/rails`.
  Then prefix the `/skydns/` string too, so the final path becomes
    `/v2/keys/skydns/local/skydns/east/production/rails`
* Host - The name of your service, e.g., `service5.mydomain.com` or an IP address (either v4 or v6);
* Port - the port where the service can be reached;
* Priority - the priority of the service, the lower the value, the more preferred;
* Weight - a weight factor that will be used for services with the same Priority;
* Text - text you want to add (this returned when doing a TXT query);
* TTL - the time-to-live of the service, overriding the default TTL. If the etcd key also has a TTL, the minimum of this value and the etcd TTL is used.

Path is the only mandatory field.

Adding the service can thus be done with:

    curl -XPUT http://127.0.0.1:4001/v2/keys/skydns/local/skydns/east/production/rails \
        -d value='{"host":"service5.example.com","priority":20}'

Or with [`etcdctl`](https://github.com/coreos/etcdctl):

    etcdctl set /skydns/local/skydns/east/production/rails \
        '{"host":"service5.example.com","priority":20}'

When doing a SRV query for these keys an SRV record is returned with the priority and a certain weight.
The weight of a service is calculated as follows. We treat weight as a percentage, so if there are
3 services, the weight is set to 33 for each:

| Service | Weight  | SRV.Weight |
| --------| ------- | ---------- |
|    a    |   100   |    33      |
|    b    |   100   |    33      |
|    c    |   100   |    33      |

If we add other weights to the equation some services will get a different Weight:

| Service | Weight  | SRV.Weight |
| --------| ------- | ---------- |
|    a    |   120   |    34      |
|    b    |   100   |    28      |
|    c    |   130   |    37      |

Note, all calculations are rounded down, so the sum total might be lower than 100.

When querying the DNS for services you can use wildcards or query for subdomains. See the section named "Wildcards" below for more information.

## Service Discovery via the DNS

You can find services by querying SkyDNS via any DNS client or utility. It uses a known domain syntax with subdomains to find matching services.

For the purpose of this document, let's suppose we have added the following services to etcd:

* 1.rails.production.east.skydns.local, mapping to service1.example.com
* 2.rails.production.west.skydns.local, mapping to service2.example.com
* 4.rails.staging.east.skydns.local, mapping to 10.0.1.125
* 6.rails.staging.east.skydns.local, mapping to 2003::8:1

These names can be added with:

    curl -XPUT http://127.0.0.1:4001/v2/keys/skydns/local/skydns/east/production/rails/1 \
        -d value='{"host":"service1.example.com","port":8080}'
    curl -XPUT http://127.0.0.1:4001/v2/keys/skydns/local/skydns/west/production/rails/2 \
        -d value='{"host":"service2.example.com","port":8080}'
    curl -XPUT http://127.0.0.1:4001/v2/keys/skydns/local/skydns/east/staging/rails/4 \
        -d value='{"host":"10.0.1.125","port":8080}'
    curl -XPUT http://127.0.0.1:4001/v2/keys/skydns/local/skydns/east/staging/rails/6 \
        -d value='{"host":"2003::8:1","port":8080}'

Testing one of the names with `dig`:

    % dig @localhost SRV 1.rails.production.east.skydns.local

    ;; QUESTION SECTION:
    ;1.rails.production.east.skydns.local.      IN      SRV

    ;; ANSWER SECTION:
    1.rails.production.east.skydns.local. 3600 IN SRV 10 0 8080 service1.example.com.

### Wildcards

Of course using the full names isn't *that* useful, so SkyDNS lets you query for subdomains, and returns responses based upon the amount of services matched by the subdomain or from the wildcard query.

If we are interested in all the servers in the `east` region, we simply omit the rightmost labels from our query:

    % dig @localhost SRV east.skydns.local

    ;; QUESTION SECTION
    ; east.skydns.local.    IN      SRV

    ;; ANSWER SECTION:
    east.skydns.local.      3600    IN      SRV     10 20 8080 service1.example.com.
    east.skydns.local.      3600    IN      SRV     10 20 8080 4.rails.staging.east.skydns.local.
    east.skydns.local.      3600    IN      SRV     10 20 8080 6.rails.staging.east.skydns.local.

    ;; ADDITIONAL SECTION:
    4.rails.staging.east.skydns.local. 3600 IN A    10.0.1.125
    6.rails.staging.east.skydns.local. 3600 IN AAAA 2003::8:1

Here all three entries of the `east` are returned.

There is one other feature at play here. The second and third names, `{4,6}.rails.staging.east.skydns.local`, only had an IP record configured. Here SkyDNS used the ectd path to construct a target name and then puts the actual IP address in the additional section. Directly querying for the A records of `4.rails.staging.east.skydns.local.` of course also works:

    % dig @localhost -p 5354 +noall +answer A 4.rails.staging.east.skydns.local.

    4.rails.staging.east.skydns.local. 3600 IN A    10.0.1.125

Another way to leads to the same result it to query for `*.east.skydns.local`, you even put the wildcard
(the `*`) in the middle of a name `staging.*.skydns.local` is a valid query, which returns all name
in staging, regardless of the region. Multiple wildcards per name are also permitted.

### Examples

Now we can try some of our example DNS lookups:

#### SRV Records

Get all Services in staging.east:

    % dig @localhost staging.east.skydns.local. SRV

    ;; QUESTION SECTION:
    ;staging.east.skydns.local. IN  SRV

    ;; ANSWER SECTION:
    staging.east.skydns.local. 3600 IN  SRV 10 50 8080 4.rails.staging.east.skydns.local.
    staging.east.skydns.local. 3600 IN  SRV 10 50 8080 6.rails.staging.east.skydns.local.

    ;; ADDITIONAL SECTION:
    4.rails.staging.east.skydns.local. 3600 IN A    10.0.1.125
    6.rails.staging.east.skydns.local. 3600 IN AAAA 2003::8:1

#### A/AAAA Records
To return A records, simply run a normal DNS query for a service matching the above patterns.

Now do a normal DNS query:

    % dig @localhost staging.east.skydns.local. A

    ;; QUESTION SECTION:
    ;staging.east.skydns.local. IN  A

    ;; ANSWER SECTION:
    staging.east.skydns.local. 3600 IN  A   10.0.1.125

Now you have a list of all known IP Addresses registered running in staging in
the east area.

Because we're returning A records and not SRV records, there are no ports
listed, so this is only useful when you're querying for services running on
ports known to you in advance.

#### CNAME Records
If for an A or AAAA query the IP address can not be parsed, SkyDNS will try to see if there is
a chain of names that will lead to an IP address. The chain can not be longer than 8. So for instance
if the following services have been registered:

    curl -XPUT http://127.0.0.1:4001/v2/keys/skydns/local/skydns/east/production/rails/1 \
        -d value='{"host":"service1.skydns.local","port":8080}'

and

    curl -XPUT http://127.0.0.1:4001/v2/keys/skydns/local/skydns/service1 \
        -d value='{"host":"10.0.2.15","port":8080}'

We have created the following CNAME chain: `1.rails.production.east.skydns.local` -> `service1.skydns.local` ->
`10.0.2.15`. If you then query for an A or AAAA for 1.rails.production.east.skydns.local SkyDNS returns:

    1.rails.production.east.skydns.local. 3600  IN  CNAME   service1.skydns.local.
    service1.skydns.local.                 3600  IN  A       10.0.2.15

##### External Names

If the CNAME chains leads to a name that falls outside of the domain (i.e. does not end with `skydns.local.`),
a.k.a. an external name, SkyDNS will attempt to resolve that name using the supplied nameservers. If this succeeds
the reply is concatenated to the current one and send to the client. So if we register this service:

    curl -XPUT http://127.0.0.1:4001/v2/keys/skydns/local/skydns/east/production/rails/1 \
        -d value='{"host":"www.miek.nl","port":8080}'

Doing an A/AAAA query for this will lead to the following response:

    1.rails.production.east.skydns.local. 3600 IN CNAME www.miek.nl.
    www.miek.nl.            3600    IN      CNAME   a.miek.nl.
    a.miek.nl.              3600    IN      A       176.58.119.54

The first CNAME is generated from within SkyDNS, the other CNANE is returned from the remote name server.

#### TXT Records

SkyDNS also allows you to query for TXT records. Just register a json with the 'text' field set.

#### NS Records

For DNS to work properly SkyDNS needs to tell peers its nameservers. This information is stored
inside etcd, in the key `local/skydns/dns/`. There multiple services maybe stored. Note these
services MUST use IP address, using names will not work. For instance:

    curl -XPUT http://127.0.0.1:4001/v2/keys/skydns/local/skydns/dns/ns \
        -d value='{"host":"172.16.0.1"}'

Registers `ns.dns.skydns.local` as a nameserver with ip address 172.16.0.1:

    % dig @localhost NS skydns.local

    ;; QUESTION SECTION:
    ;skydns.local.          IN  NS

    ;; ANSWER SECTION:
    skydns.local.       3600    IN  NS  ns.dns.skydns.local.

    ;; ADDITIONAL SECTION:
    ns.dns.skydns.local.    3600    IN  A   172.16.0.1

The first nameserver should have the hostname `ns`  (as this is used in the SOA record). Having the nameserver(s)
in etcd make sense because usualy it is hard for SkyDNS to figure this out by itself, espcially when
running behind NAT or running on 127.0.0.1:53 and being forwarded packets IPv6 packets, etc. etc.

#### PTR Records: Reverse Addresses

When registering a service with an IP address only, you might also want to register
the reverse (the hostname the address points to). In the DNS these records are called
PTR records.

So looking back at some of the services in the section "Service Discovery via the DNS",
we register these IP only ones:

    4.rails.staging.east.skydns.local. 10.0.1.125

To add the reverse of these address you need to add the following names and values:

    125.1.0.10.in-addr.arpa. service1.example.com.

These can be added with:

    curl -XPUT http://127.0.0.1:4001/v2/keys/skydns/arpa/in-addr/10/0/1/125 \
        -d value='{"host":"service1.example.com}'

If SkyDNS receives a PTR query it will check these paths and will return the
contents. Note that these replies are sent with the AA (Authoritative Answer)
bit *off*. If nothing is found locally the query is forwarded to the local
recursor (if so configured), otherwise SERVFAIL is returned.

This also works for IPv6 addresses, except that the reverse path is quite long.

#### DNS Forwarding

By specifying nameservers in SkyDNS's config, for instance `8.8.8.8:53,8.8.4.4:53`,
you create a DNS forwarding proxy. In this case it round-robins between the two
nameserver IPs mentioned.

Requests for which SkyDNS isn't authoritative will be forwarded and proxied back to
the client. This means that you can set SkyDNS as the primary DNS server in
`/etc/resolv.conf` and use it for both service discovery and normal DNS operations.

#### DNSSEC

SkyDNS supports signing DNS answers, also known as DNSSEC. To use it, you need to
create a DNSSEC keypair and use that in SkyDNS. For instance, if the domain for
SkyDNS is `skydns.local`:

    % dnssec-keygen skydns.local
    Generating key pair............++++++ ...................................++++++
    Kskydns.local.+005+49860

This creates two files with the basename `Kskydns.local.+005.49860`, one with the
extension `.key` (this holds the public key) and one with the extension `.private` which
holds the private key. The basename of these files should be given to SkyDNS's DNSSEC configuration
option like so (together with some other options):

    curl -XPUT http://127.0.0.1:4001/v2/keys/skydns/config -d \
        value='{"dns_addr":"127.0.0.1:5354","dnssec":"Kskydns.local.+005+55656"}'

If you then query with `dig +dnssec` you will get signatures, keys and NSEC3 records returned.
Authenticated denial of existence is implemented using NSEC3 white lies,
see [RFC7129](http://tools.ietf.org/html/rfc7129), Appendix B.

#### Host Local Values

SkyDNS supports storing values which are specific for that *instance* of SkyDNS.

This can be useful when you have SkyDNS running on multiple hosts, but want to store values that are specific
for a single host. For example the public IP-address of the host or the IP-address on the tenant network.

To do that you need to specify a unique value for that host with `-local`. A good unique value for that
would be an UUID which you can generate with `uuidgen` for instance.

That unique value is used as a path in etcd to store the values separately from the normal values. It is still stored
in the etcd backend so a restart of SkyDNS with the same unique value will give it access to the old data.

In the example here, we don't use an UUID, we use `public.addresses`:

    % skydns -local public.addresses.skydns.local &

    % curl -XPUT http://127.0.0.1:4001/v2/keys/skydns/local/skydns/local/addresses/public \
        -d value='{"host":"192.0.2.1"}'

    % dig @127.0.0.1 local.dns.skydns.local. A

    ;; QUESTION SECTION:
    ;local.dns.skydns.local. IN  A

    ;; ANSWER SECTION:
    local.dns.skydns.local. 3600 IN  A   192.0.2.1

## Kubernetes
SkyDNS now has primitive support for watching the API of a Kubernetes master and
inserting DNS records to represent the services running in a Kubernetes cluster.

The service name in Kubernetes will be registered as a host (A) record under the SkyDNS
domain.  For example, if you use the default `skydns.local` configuration, a service called 
`redismaster` will be available at `redismaster.skydns.local`.  Additionally, SRV records
are created for each service that is registered, so queries for SRV records will return all
information necessary to connect to your service:

```
;; ANSWER SECTION:
redismaster.skydns.local. 30    IN      SRV     10 100 10000 10.0.2.17
```
In the query above, you can see the IP address, the weight and the port have been set
by SkyDNS.

Kubernets support is experimental and will improve with time.  To enable it, start SkyDNS
with the `-kubernetes` flag and the client configuration parameters that you would use to connect
to an APIServer instance.  At a minimum you need to pass the -master flag.  A common example to 
start a SkyDNS server:

```
sudo skydns -kubernetes -domain kubernetes.local. -master="http://127.0.0.1:8080"
```
This command starts a SkyDNS service listening on port 53, connecting to the
Kubernetes APIServer on localhost, and serving the domain `kubernetes.local`, meaning all
services in Kubernetes will be resolved in the form `servicename.kubernetes.local`

For questions on SkyDNS/Kubernetes integration please see the #google-containers channel
on freenode, or open tickets in the SkyDNS repository.

# FAQ

## How Do I Create an Address Pool and Round Robin Between Them

You have 3 machines with 3 different IP addresses and you want to have
1 name pointing to all 3 possible addresses. The name we want to use is:
`db.skydns.local` and the 3 addresses are 127.0.0.{1,2,3}. For this
to work we create the hosts named `x{1,2,3}.db.skydns.local` in etcd:

    curl -XPUT http://127.0.0.1:4001/v2/keys/skydns/local/skydns/db/x1 -d \
        value='{"Host":"127.0.0.1"}'
    curl -XPUT http://127.0.0.1:4001/v2/keys/skydns/local/skydns/db/x2 -d \
        value='{"Host": "127.0.0.2"'}
    curl -XPUT http://127.0.0.1:4001/v2/keys/skydns/local/skydns/db/x3 -d \
        value='{"Host": "127.0.0.3"'}

Now the name `db.skydns.local` is the "load balanced" name for the database, SkyDNS
will round-robin by default in this case unless `-round-robin=false` is enabled.

# Docker
Official Docker images are at the [Docker Hub](https://registry.hub.docker.com/u/skynetservices/skydns/):

* master -> skynetservices/skydns:latest
* latest tag -> skynetservices/skydns:latest-tagged

# License
The MIT License (MIT)

Copyright © 2014 The SkyDNS Authors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
