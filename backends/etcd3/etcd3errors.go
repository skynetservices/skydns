package etcd3

import (
	"fmt"
)

/*
 * etcdv3 doesn't throw errors anymore when a key is not found,
 * however skydns utilizes this feature, so we need to create our own
 * instance of error() and have skydns' algo use it accordingly as before.
 */
type Etcd3Error struct {
	Code int
	Message string
}

func (e Etcd3Error) Error() string {
	return fmt.Sprintf("%v - %v", e.Code, e.Message);
}

const (
	KeyNotFound = 100 //the value 100 is an arbritary value to coincide with what was being used in etcdv2.
)