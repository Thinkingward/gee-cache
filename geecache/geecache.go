package geecache

import (
	"errors"
	"fmt"
	"geecache/registry"
	"geecache/singleflight"
	"log"
	"sync"
	"time"
)

var DefaultExpireTime = 30 * time.Second

type Group struct {
	name      string
	getter    Getter
	mainCache cache
	hotCache  cache //热点数据
	peers     registry.PeerPicker
	loader    *singleflight.Group
}

var (
	mu     sync.RWMutex
	groups = make(map[string]*Group)
)

type Getter interface {
	Get(key string) ([]byte, error)
}

// 数据源很多，需要区分不同的数据源，通过GetterFunc类型实现Getter接口
// 这样就可以将任意函数类型转换为Getter类型，由用户自己实现
type GetterFunc func(key string) ([]byte, error)

func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

// 创建一个新的Group
func NewGroup(name string, cacheBytes int64, hotcacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
		hotCache:  cache{cacheBytes: hotcacheBytes},
		loader:    &singleflight.Group{},
	}
	groups[name] = g
	return g
}

// 根据name获取Group
func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}

// 根据key获取value
func (g *Group) Get(key string) (*ByteView, error) {
	if key == "" {
		return &ByteView{}, fmt.Errorf("key is required")
	}

	if v, ok := g.lookupCache(key); ok {
		log.Println("[GeeCache] hit")
		return v, nil
	}
	log.Println("[GeeCache] miss,try to add it")
	return g.Load(key)
}

func (g *Group) Load(key string) (value *ByteView, err error) {
	viewi, err := g.loader.Do(key, func() (interface{}, error) { // 确保高并发场景下每个key只请求一次
		if g.peers != nil {
			log.Println("try to search from peers")
			if peer, ok := g.peers.PickPeer(key); ok {
				if value, err = g.getFromPeer(peer, key); err == nil {
					log.Println("get from peer error:", err)
					return nil, err
				}
				return value, nil
			}
		}
		return g.getLocally(key)
	})

	if err == nil {
		return viewi.(*ByteView), nil
	}
	return
}

func (g *Group) getLocally(key string) (*ByteView, error) {
	bytes, err := g.getter.Get(key)
	if err != nil {
		return &ByteView{}, err
	}
	value := &ByteView{b: cloneBytes(bytes), e: time.Now().Add(DefaultExpireTime)}
	g.populateCache(key, value)
	return value, nil
}

func (g *Group) populateCache(key string, value *ByteView) {
	g.mainCache.add(key, value)
}

func (g *Group) lookupCache(key string) (value *ByteView, ok bool) {
	value, ok = g.mainCache.get(key)
	if ok {
		return
	}
	value, ok = g.hotCache.get(key)
	return
}

func (g *Group) RegisterPeers(peers registry.PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

func (g *Group) getFromPeer(peer registry.PeerGetter, key string) (*ByteView, error) {
	bytes, err := peer.Get(g.name, key)
	if err != nil {
		return nil, err
	}
	return &ByteView{b: bytes}, nil
}

func (g *Group) Set(key string, value *ByteView, ishot bool) error {
	if key == "" {
		return errors.New("key is empty")
	}
	if ishot {
		return g.setHotCache(key, value)
	}
	_, err := g.loader.Do(key, func() (interface{}, error) {
		if peer, ok := g.peers.PickPeer(key); ok {
			err := g.setFromPeer(peer, key, value, ishot)
			if err != nil {
				log.Println("set from peer error:", err)
				return nil, err
			}
			return value, nil
		}
		//如果!ok说明选择到当前节点
		g.mainCache.add(key, value)
		return value, nil
	})
	return err
}

func (g *Group) setFromPeer(peer registry.PeerGetter, key string, value *ByteView, ishot bool) error {
	return peer.Set(g.name, key, value.ByteSlice(), value.Expire(), ishot)
}

// 设置热点缓存
func (g *Group) setHotCache(key string, value *ByteView) error {
	if key == "" {
		return errors.New("key is required")
	}
	g.loader.Do(key, func() (interface{}, error) {
		g.hotCache.add(key, value)
		log.Printf("set hot cache %v \n", value.ByteSlice())
		return nil, nil
	})
	return nil
}
