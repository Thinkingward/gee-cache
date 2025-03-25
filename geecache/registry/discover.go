package registry

import (
	"context"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/naming/resolver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// EtcdDial向grpc请求一个服务
// 通过提供一个etcd client和服务名即可获得connection
func DialPeer(c *clientv3.Client, service string) (conn *grpc.ClientConn, err error) {
	PeerResolver, err := resolver.NewBuilder(c)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return grpc.Dial(
		"etcd:///"+service,
		grpc.WithResolvers(PeerResolver),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
}

// 根据服务名在etcd中进行服务发现并返回对应的ip地址
func GetAddrByName(c *clientv3.Client, name string) (addr string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := c.Get(ctx, name)
	if err != nil {
		return "", err
	}
	return string(resp.Kvs[0].Value), nil
}
