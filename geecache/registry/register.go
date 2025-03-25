package registry

//提供服务Service注册至etcd的能力
import (
	"context"
	"fmt"
	"log"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/naming/endpoints"
)

var (
	defaultTimeout      = 3 * time.Second
	defaultLeaseExpTime = 10
)

type Etcd struct {
	EtcdCli *clientv3.Client
	leaseId clientv3.LeaseID   //租约ID
	ctx     context.Context    //上下文
	cancel  context.CancelFunc //取消函数，避免内存泄漏
}

func NewEtcd(endpoints []string) (*Etcd, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: defaultTimeout,
	})
	if err != nil {
		log.Println("create etcd register err:", err)
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	svr := &Etcd{
		EtcdCli: client,
		ctx:     ctx,
		cancel:  cancel,
	}
	return svr, nil
}

// 为注册在etcd上的节点创建租约。因为服务端无法保证自身是一直可用的，可能会宕机，所以与etcd的租约是有时间期限的，租约一旦过期，服务端存储在etcd上的服务地址信息就会消失
// 如果服务端正常运行，etcd中的地址信息又必须存在，因此发送心跳检测，一旦发现etcd上没有自己的服务地址，请求重新添加
func (s *Etcd) CreateLease(expireTime int) error {
	res, err := s.EtcdCli.Grant(s.ctx, int64(expireTime))
	if err != nil {
		return err
	}
	s.leaseId = res.ID
	log.Println("create lease success:", s.leaseId)
	return nil
}

// 绑定服务和对应的租约
func (s *Etcd) BindLease(server string, addr string) error {
	_, err := s.EtcdCli.Put(s.ctx, server, addr, clientv3.WithLease(s.leaseId))
	if err != nil {
		return err
	}
	return nil
}

func (s *Etcd) KeepAlive() error {
	log.Println("keep alive start")
	log.Println("s.leaseId:", s.leaseId)
	KeepRespChan, err := s.EtcdCli.KeepAlive(context.Background(), s.leaseId)
	if err != nil {
		log.Println("keep alive err:", err)
	}
	go func() {
		for {
			for KeepResp := range KeepRespChan {
				if KeepResp == nil {
					fmt.Println("keep alive is stop")
					return
				} else {
					fmt.Println("keep alive is ok")
				}
			}
			time.Sleep(5 * time.Second)
		}
	}()
	return nil
}

// 把serviceName作为key，addr作为value存储在etcd中
func (s *Etcd) RegisterServer(serviceName, addr string) error {
	//创建租约
	err := s.CreateLease(defaultLeaseExpTime)
	if err != nil {
		log.Println("create etcd register err:", err)
		return err
	}
	//绑定租约
	err = s.BindLease(serviceName, addr)
	if err != nil {
		log.Println("bind etcd register err:", err)
		return err
	}
	//心跳检测
	err = s.KeepAlive()
	if err != nil {
		log.Println("keep alive register err:", err)
		return err
	}
	//注册服务用于服务发现
	em, err := endpoints.NewManager(s.EtcdCli, serviceName)
	if err != nil {
		log.Println("create etcd register err:", err)
		return err
	}
	return em.AddEndpoint(s.ctx, serviceName+"/"+addr, endpoints.Endpoint{Addr: addr}, clientv3.WithLease(s.leaseId))
}
