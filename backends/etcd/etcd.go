// Copyright (c) 2014 The SkyDNS Authors. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

// Package etcd provides the default SkyDNS server Backend implementation,
// which looks up records stored under the `/skydns` key in etcd when queried.
package etcd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/skynetservices/skydns/msg"
	"github.com/skynetservices/skydns/singleflight"

	etcd "github.com/coreos/etcd/client"
	etcdv3 "github.com/coreos/etcd/clientv3"
	"golang.org/x/net/context"
	"github.com/coreos/etcd/mvcc/mvccpb"
)

// Config represents configuration for the Etcd backend - these values
// should be taken directly from server.Config
type Config struct {
	Ttl      uint32
	Priority uint16
}

type Backend struct {
	client   etcd.KeysAPI
	ctx      context.Context
	config   *Config
	inflight *singleflight.Group
}

type Backendv3 struct {
	client   etcdv3.Client
	ctx      context.Context
	config   *Config
	inflight *singleflight.Group
}

// NewBackend returns a new Backend for SkyDNS, backed by etcd.
func NewBackend(client etcd.KeysAPI, ctx context.Context, config *Config) *Backend {
	return &Backend{
		client:   client,
		ctx:      ctx,
		config:   config,
		inflight: &singleflight.Group{},
	}
}

// NewBackendv3 returns a new Backend for SkyDNS, backed by etcd v3
func NewBackendv3(client etcdv3.Client, ctx context.Context, config *Config) *Backendv3 {
	return &Backendv3{
		client:   client,
		ctx:      ctx,
		config:   config,
		inflight: &singleflight.Group{},
	}
}

func (g *Backend) Records(name string, exact bool) ([]msg.Service, error) {
	path, star := msg.PathWithWildcard(name)
	r, err := g.get(path, true)
	if err != nil {
		return nil, err
	}
	segments := strings.Split(msg.Path(name), "/")
	switch {
	case exact && r.Node.Dir:
		return nil, nil
	case r.Node.Dir:
		return g.loopNodes(r.Node.Nodes, segments, star, nil)
	default:
		return g.loopNodes([]*etcd.Node{r.Node}, segments, false, nil)
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

func (g *Backend) ReverseRecord(name string) (*msg.Service, error) {
	path, star := msg.PathWithWildcard(name)
	if star {
		return nil, fmt.Errorf("reverse can not contain wildcards")
	}
	r, err := g.get(path, true)
	if err != nil {
		return nil, err
	}
	if r.Node.Dir {
		return nil, fmt.Errorf("reverse must not be a directory")
	}
	segments := strings.Split(msg.Path(name), "/")
	records, err := g.loopNodes([]*etcd.Node{r.Node}, segments, false, nil)
	if err != nil {
		return nil, err
	}
	if len(records) != 1 {
		return nil, fmt.Errorf("must be only one service record")
	}
	return &records[0], nil
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

// get is a wrapper for client.Get that uses SingleInflight to suppress multiple
// outstanding queries.
func (g *Backend) get(path string, recursive bool) (*etcd.Response, error) {
	resp, err := g.inflight.Do(path, func() (interface{}, error) {
		r, e := g.client.Get(g.ctx, path, &etcd.GetOptions{Sort: false, Recursive: recursive})
		if e != nil {
			return nil, e
		}
		return r, e
	})
	if err != nil {
		return nil, err
	}
	return resp.(*etcd.Response), err
}

func (g *Backendv3) get(path string, recursive bool) (*etcdv3.GetResponse, error) {
	resp, err := g.inflight.Do(path, func() (interface{}, error) {
		if recursive == true {
			r, e := g.client.Get(g.ctx, path, etcdv3.WithPrefix())
			if r.Kvs == nil {
				return nil, fmt.Errorf("ErrorCodeKeyNotFound")
			}
			if e != nil {
				return nil, e
			}
			return r, e
		} else {
			r, e := g.client.Get(g.ctx, path)
			if r.Kvs == nil {
				return nil, fmt.Errorf("ErrorCodeKeyNotFound")
			}
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
	Host     string
	Port     int
	Priority int
	Weight   int
	Text     string
}

// skydns/local/skydns/east/staging/web
// skydns/local/skydns/west/production/web
//
// skydns/local/skydns/*/*/web
// skydns/local/skydns/*/web

// loopNodes recursively loops through the nodes and returns all the values. The nodes' keyname
// will be match against any wildcards when star is true.
func (g *Backend) loopNodes(ns []*etcd.Node, nameParts []string, star bool, bx map[bareService]bool) (sx []msg.Service, err error) {
	if bx == nil {
		bx = make(map[bareService]bool)
	}
Nodes:
	for _, n := range ns {
		if n.Dir {
			nodes, err := g.loopNodes(n.Nodes, nameParts, star, bx)
			if err != nil {
				return nil, err
			}
			sx = append(sx, nodes...)
			continue
		}
		if star {
			keyParts := strings.Split(n.Key, "/")
			for i, n := range nameParts {
				if i > len(keyParts)-1 {
					// name is longer than key
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
		if err := json.Unmarshal([]byte(n.Value), serv); err != nil {
			return nil, err
		}
		b := bareService{serv.Host, serv.Port, serv.Priority, serv.Weight, serv.Text}
		if _, ok := bx[b]; ok {
			continue
		}
		bx[b] = true

		serv.Key = n.Key
		serv.Ttl = g.calculateTtl(n, serv)
		if serv.Priority == 0 {
			serv.Priority = int(g.config.Priority)
		}
		sx = append(sx, *serv)
	}
	return sx, nil
}

func (g *Backendv3) loopNodes(kv []*mvccpb.KeyValue, nameParts []string, star bool, bx map[bareService]bool) (sx []msg.Service, err error) {
	if bx == nil {
		bx = make(map[bareService]bool)
	}

	index := 0
	lengthOfEntries := len(kv)
	for index < lengthOfEntries {

		serviceInstance := new(msg.Service)

		if err := json.Unmarshal(kv[index].Value, serviceInstance); err != nil {
			return nil, err
		}

		b := bareService{serviceInstance.Host, serviceInstance.Port, serviceInstance.Priority, serviceInstance.Weight, serviceInstance.Text}

		bx[b] = true
		serviceInstance.Key = string(kv[index].Key)
		//TODO: shouldn't be that another call (LeaseRequest) for TTL
		serviceInstance.Ttl = g.calculateTtl(kv[index], serviceInstance)

		if serviceInstance.Priority == 0 {
			serviceInstance.Priority = int(g.config.Priority)
		}

		sx = append(sx, *serviceInstance)
		index++
	}

	return sx, nil
}

// calculateTtl returns the smaller of the etcd TTL and the service's
// TTL. If neither of these are set (have a zero value), the server
// default is used.
func (g *Backend) calculateTtl(node *etcd.Node, serv *msg.Service) uint32 {
	etcdTtl := uint32(node.TTL)

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

func (g *Backendv3) calculateTtl(kv *mvccpb.KeyValue, serv *msg.Service) uint32 {
	etcdTtl := uint32(kv.Lease) //TODO: still waiting for Least request rpc to be available in etcdv3's api

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

// Client exposes the underlying Etcd client (used in tests).
func (g *Backend) Client() etcd.KeysAPI {
       return g.client
}

func (g *Backendv3) Client() etcdv3.Client {
	return g.client
}