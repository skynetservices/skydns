// Copyright (c) 2014 The SkyDNS Authors. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

// Package etcd provides the default SkyDNS server Backend implementation,
// which looks up records stored under the `/skydns` key in etcd when queried.
// This one particularly concerns with the support of etcd version 3.
package etcd3

import (
	etcdv3 "github.com/coreos/etcd/clientv3"
	"golang.org/x/net/context"
	"github.com/skynetservices/skydns/singleflight"
	"strings"
	"github.com/skynetservices/skydns/msg"
	"fmt"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"encoding/json"
)

type Config struct {
	Ttl uint32
	Priority uint16
}

type Backendv3 struct {
	client etcdv3.Client
	ctx context.Context
	config *Config
	inflight *singleflight.Group
}

// NewBackendv3 returns a new Backend for SkyDNS, backed by etcd v3
func NewBackendv3 (client etcdv3.Client, ctx context.Context, config *Config) *Backendv3 {
	return &Backendv3{
		client: client,
		ctx: ctx,
		config: config,
		inflight: &singleflight.Group{},
	}
}

func (g *Backendv3) Records(name string, exact bool) ([]msg.Service, error) {
	path, star := msg.PathWithWildcard(name)
	r, err := g.get(path, true)
	if err != nil {
		return nil, err
	}
	segments := strings.Split(msg.Path(name), "/")

	return g.loopNodes(r.Kvs, segments, star, nil)
}

func (g *Backendv3) ReverseRecord(name string) (*msg.Service, error) {
	path, star := msg.PathWithWildcard(name)
	if star {
		return nil, fmt.Errorf("reverse can not contain wildcards")
	}

	r, err := g.get(path, true)
	if err != nil {
		return nil, err
	}

	segments := strings.Split(msg.Path(name), "/")
	records, err := g.loopNodes(r.Kvs, segments, false, nil)
	if err != nil {
		return nil, err
	}
	if len(records) != 1 {
		return nil, fmt.Errorf("must be only one service record")
	}
	return &records[0], nil
}

func (g *Backendv3) get(path string, recursive bool) (*etcdv3.GetResponse, error) {
	resp, err := g.inflight.Do(path, func() (interface{}, error){
		if recursive == true {
			r, e := g.client.Get(g.ctx, path, etcdv3.WithPrefix())
			if e != nil {
				return nil, e
			}
			return r, e
		} else {
			r, e := g.client.Get(g.ctx, path)
			if e != nil {
				return nil, e
			}
			return r, e
		}
	})

	if err != nil {
		return nil, err
	}
	return resp.(*etcdv3.GetResponse), err
}

type bareService struct {
	Host string
	Port int
	Priority int
	Weight int
	Text string
}

func (g *Backendv3) loopNodes(kv []*mvccpb.KeyValue, nameParts []string, star bool, bx map[bareService]bool) (sx []msg.Service, err error) {
	if bx == nil {
		bx = make(map[bareService]bool)
	}
Nodes:
	for _, item := range kv {

		if star {
			s := string(item.Key[:])
			keyParts := strings.Split(s, "/")
			for i, n := range nameParts {
				if i > len(keyParts)-1 {
					continue Nodes
				}
				if n == "*" || n == "any" {
					continue
				}
				if keyParts[i] != n {
					continue Nodes
				}
			}
		}


		serv := new(msg.Service)
		if err := json.Unmarshal(item.Value, serv); err != nil {
			return nil, err
		}

		b := bareService{serv.Host,
				 serv.Port,
				 serv.Priority,
				 serv.Weight,
				 serv.Text}

		bx[b] = true
		serv.Key = string(item.Key)
		//TODO: another call (LeaseRequest) for TTL when RPC in etcdv3 is ready
		serv.Ttl = g.calculateTtl(item, serv)

		if serv.Priority == 0 {
			serv.Priority = int(g.config.Priority)
		}

		sx = append(sx, *serv)
	}
	return sx, nil
}

func (g *Backendv3) calculateTtl(kv *mvccpb.KeyValue, serv *msg.Service) uint32 {
	etcdTtl := uint32(kv.Lease) //TODO: default value for now, should be an rpc call for least request when it becomes available in etcdv3's api

	if etcdTtl == 0 && serv.Ttl == 0 {
		return g.config.Ttl
	}
	if etcdTtl == 0 {
		return serv.Ttl
	}
	if serv.Ttl == 0 {
		return etcdTtl
	}
	if etcdTtl < serv.Ttl {
		return etcdTtl
	}
	return serv.Ttl
}

func (g *Backendv3) Client() etcdv3.Client {
	return g.client
}
