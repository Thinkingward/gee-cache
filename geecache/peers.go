package geecache

import pb "geecache/geecachepb"

//根据传入的key选择相应节点PeerGetter
type PeerPicker interface {
	PickPeer(key string) (peer PeerGetter, ok bool)
}

//从对应的group查找缓存值
type PeerGetter interface {
	Get(in *pb.Request, out *pb.Response) error
}
