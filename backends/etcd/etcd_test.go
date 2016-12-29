// Copyright (c) 2015 All Right Reserved, Improbable Worlds Ltd.

package etcd

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	etcd "github.com/coreos/etcd/client"
	etcd_harness "github.com/mwitkow/go-etcd-harness"
	"github.com/skynetservices/skydns/msg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/context"
)

type etcdBackendTestSuite struct {
	suite.Suite
	etcdBackend *Backend
}

func (s *etcdBackendTestSuite) SetupTest() {
	s.etcdBackend.client.Delete(context.TODO(), "skydns", &etcd.DeleteOptions{Recursive: true, Dir: true})
}

func (s *etcdBackendTestSuite) TestGettingSingleRecordShouldReturnCorrectService() {
	s.createEtcdRecord(msg.Service{Host: "1.1.1.1", Key: "/skydns/com/test"})
	records, err := s.etcdBackend.Records("test.com", true, false)
	assert.NoError(s.T(), err, "no err")
	assert.Equal(s.T(), 1, len(records))
	assert.Equal(s.T(), "1.1.1.1", records[0].Host)
}

func (s *etcdBackendTestSuite) TestStubZone() {
	s.createEtcdRecord(msg.Service{Host: "1.1.1.1", Key: "/skydns/com/test/dns/stub/com/othertest/a/stubzone1/a"})
	s.createEtcdRecord(msg.Service{Host: "1.1.1.2", Key: "/skydns/com/test/dns/stub/com/othertest/a/stubzone1/b"})
	s.createEtcdRecord(msg.Service{Host: "1.1.1.1", Key: "/skydns/com/test/dns/stub/com/othertest/stubzone2/b"})
	s.createEtcdRecord(msg.Service{Host: "1.1.1.2", Key: "/skydns/com/test/dns/stub/com/othertest/stubzone2/a"})
	records, err := s.etcdBackend.Records("stub.dns.test.com", false, true)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 4, len(records))
}

func TestDeploymentServiceSuite(t *testing.T) {
	if testing.Short() {
		t.Skipf("DeploymentServiceSuite is a long integration test suite. Skipping due to test short.")
	}
	if !etcd_harness.LocalEtcdAvailable() {
		t.Skipf("etcd is not available in $PATH, skipping suite")
	}

	server, err := etcd_harness.New(os.Stderr)
	if err != nil {
		t.Fatalf("failed starting test server: %v", err)
	}
	t.Logf("will use etcd test endpoint: %v", server.Endpoint)
	defer func() {
		server.Stop()
		t.Logf("cleaned up etcd test server")
	}()
	etcdClient := etcd.NewKeysAPI(server.Client)
	suite.Run(t, &etcdBackendTestSuite{etcdBackend: NewBackend(etcdClient, context.TODO(), &Config{})})
}

func (s *etcdBackendTestSuite) createEtcdRecord(srv msg.Service) {
	ctx, _ := context.WithTimeout(context.TODO(), time.Second*5)
	value, err := json.Marshal(srv)
	require.NoError(s.T(), err, "marshaling etcd record should not fail")
	_, err = s.etcdBackend.client.Create(ctx, srv.Key, string(value))
	require.NoError(s.T(), err, "creating etcd record should not fail")
}
