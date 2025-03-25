package lru

import (
	"container/list"
	"math/rand"
	"time"
)

var DefaultMaxBytes int64 = 10
var DefaultExpireRandom time.Duration = 3 * time.Minute

type NowFunc func() time.Time

var nowFunc NowFunc = time.Now

type Cache struct {
	maxBytes     int64 //允许使用的最大内存
	nbytes       int64 //当前已使用的内存
	ll           *list.List
	cache        map[string]*list.Element
	OnEvicted    func(key string, value Value) //某条记录被移除时的回调函数
	Now          NowFunc
	ExpireRandom time.Duration
}

type entry struct { //双向链表节点的数据类型
	key     string //保存key是为了移除记录时需要使用key将其从map中删除
	value   Value
	expire  time.Time //过期时间
	addTime time.Time
}

type Value interface {
	Len() int //返回占用的内存大小
}

func New(maxBytes int64, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:     maxBytes,
		ll:           list.New(),
		cache:        make(map[string]*list.Element),
		OnEvicted:    onEvicted,
		Now:          nowFunc,
		ExpireRandom: DefaultExpireRandom,
	}
}

func (c *Cache) Len() int {
	return c.ll.Len() //返回链表长度
}

// 查找功能
func (c *Cache) Get(key string) (value Value, ok bool) {
	if ele, ok := c.cache[key]; ok {

		kv := ele.Value.(*entry) //将ele.Value断言为*entry类型
		//如果kv过期了，将它们移除缓存
		if kv.expire.Before(time.Now()) {
			c.removeElement(ele)
			return nil, false
		}
		//如果没有过期，更新键值对
		expireTime := kv.expire.Sub(kv.addTime)
		kv.expire = time.Now().Add(expireTime)
		kv.addTime = time.Now()
		c.ll.MoveToFront(ele) //将链表中对应节点移到队尾
		return kv.value, true
	}
	return nil, false
}

// 删除最老的
func (c *Cache) RemoveOldest() {
	ele := c.ll.Back() //取队首节点
	if ele != nil {
		c.ll.Remove(ele) //从链表中删除
		kv := ele.Value.(*entry)
		delete(c.cache, kv.key)                                //从map中删除
		c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len()) //更新当前所用内存
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value) //调用回调函数
		}
	}
}

func (c *Cache) Remove(key string) {
	if ele, ok := c.cache[key]; ok {
		c.removeElement(ele)
	}
}

func (c *Cache) removeElement(ele *list.Element) {
	kv := ele.Value.(*entry)
	delete(c.cache, kv.key)
	c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len())
	if c.OnEvicted != nil {
		c.OnEvicted(kv.key, kv.value)
	}
}

func (c *Cache) Add(key string, value Value, expire time.Time) {
	//randomDuration是用户添加的过期时间进行一定范围的随机，防止缓存雪崩
	randomDuration := time.Duration(rand.Int63n(int64(c.ExpireRandom)))

	if ele, ok := c.cache[key]; ok { //如果键已经存在，更新value并将节点移到队尾
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		c.nbytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value
		kv.expire = expire.Add(randomDuration)
	} else { //如果键不存在，创建新节点并添加到队尾
		ele := c.ll.PushFront(&entry{key, value, expire.Add(randomDuration), time.Now()})
		c.cache[key] = ele
		c.nbytes += int64(len(key)) + int64(value.Len())
	}
	for c.maxBytes != 0 && c.maxBytes < c.nbytes { //如果当前内存超过了设定的最大值，移除最近最少访问的节点
		c.RemoveOldest()
	}
}
