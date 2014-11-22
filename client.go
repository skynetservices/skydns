// Copyright (c) 2014 The SkyDNS Authors. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package main

import (
	"log"
	"net/url"
	"strings"

	"github.com/coreos/go-etcd/etcd"
)

func NewClient(machines []string) (client *etcd.Client) {
	// set default if not specified in env
	if len(machines) == 1 && machines[0] == "" {
		machines[0] = "http://127.0.0.1:4001"
	}
	if strings.HasPrefix(machines[0], "https://") {
		var err error
		// TODO(miek): machines is local, the rest is global, ugly.
		if client, err = etcd.NewTLSClient(machines, tlspem, tlskey, cacert); err != nil {
			// TODO(miek): would be nice if this wasn't a fatal error
			log.Fatalf("failure to connect: %s\n", err)
		}
		client.SyncCluster()
	} else {
		client = etcd.NewClient(machines)
		client.SyncCluster()
	}
	return client
}

// updateClient updates the client with the machines found in v2/_etcd/machines.
func (s *server) UpdateClient(resp *etcd.Response) {
	machines := make([]string, 0, 3)
	for _, m := range resp.Node.Nodes {
		u, e := url.Parse(m.Value)
		if e != nil {
			continue
		}
		// etcd=bla&raft=bliep
		// TODO(miek): surely there is a better way to do this
		ms := strings.Split(u.String(), "&")
		if len(ms) == 0 {
			continue
		}
		if len(ms[0]) < 5 {
			continue
		}
		machines = append(machines, ms[0][5:])
	}
	// When CoreOS (and thus ectd) crash this call back seems to also trigger, but we
	// don't have any machines. Don't trigger this update then, as a) it
	// crashes SkyDNS and b) potentially leaves with no machines to connect to.
	// Keep the old ones and hope they still work and wait for another update
	// in the future.
	if len(machines) > 0 {
		s.config.log.Infof("setting new etcd cluster to %v", machines)
		c := NewClient(machines)
		// This is our RCU, switch the pointer, old readers get the old
		// one, new reader get the new one.
		s.client = c
	}
}

// lookup is a wrapper for client.Get that uses SingleInflight to suppress multiple
// outstanding queries.
func lookup(client *etcd.Client, path string, recursive bool) (*etcd.Node, error) {
	resp, err, _ := etcdInflight.Do(path, func() (*etcd.Response, error) {
		r, e := client.Get(path, false, recursive)
		if e != nil {
			return nil, e
		}
		return r, e
	})

	if (resp == nil) {
		return nil, err
	}

	// shared?
	return resp.Node, err
}

// first, we look for entries matching incoming request
// if this fails then check if there is a wildcard DNS entry for the subdomain of incoming request
func get(client *etcd.Client, path string, recursive bool) (*etcd.Node, error) {
	n1, err := lookup(client, path, recursive)

	//no matching records => try wildcard dns
	if (n1 == nil) {
		//load all defined wildcard entries
		//For most of use cases there will be one or two wild card DNS rules => this should not be perf issue to load all
		n2, e := lookup(client, "/skydns/*", true)

		if (e != nil) {
			return nil, e
		}

        //look for most specific entry matching request path
		best := bestMatch(wildcardPath(path), &n2.Nodes)
		if (best == nil) {
			return n1, err //no match, return response from first lookup and this will result in proper "no such domain"
		}

		return best, nil
	}

	return n1, err
}

//Hardcoding "/skydns" here is a bit ugly. Passing prefix and actual lookup string might be cleaner solution but it requires
//   larger code refactoring and prefix is hardcoded in other places anyways.
func wildcardPath(path string) string {
	return strings.Replace(path, "/skydns", "/skydns/*", 1)
}

//find matching wildcard DNS entries.
// Looking for leaf directory entry those key is prefix of lookup path
//
// Motivating example:
//   if we have entry for *.a.example.com then it should match
//     a.example.com
//     x.a.example.com
//     y.x.example.com
//   but should not match
//     b.example.com
func bestMatch(path string, nodes *etcd.Nodes) *etcd.Node {
	for _, n := range *nodes {
		//Need to match /com/example but not /com/example1. Thus use HasPrefix
		if (n.Dir && (path == n.Key || strings.HasPrefix(path, n.Key + "/"))) {
			if (isLeafDirectory(n)) {
				return n
			}
			return bestMatch(path, &n.Nodes)
		}
	}

	return nil
}

func isLeafDirectory(n *etcd.Node) bool {
	return n.Dir && n.Nodes.Len() != 0 && !n.Nodes[0].Dir
}
