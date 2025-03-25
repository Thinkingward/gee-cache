package registry

import "time"

//peers是用于rpc交流的模块

//根据传入的key选择相应节点PeerGetter
type PeerPicker interface {
	PickPeer(key string) (peer PeerGetter, ok bool)
}

//从对应的group查找缓存值
type PeerGetter interface {
	Get(group string, key string) ([]byte, error)
	Set(group string, key string, value []byte, expire time.Time, ishot bool) error
}
