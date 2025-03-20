package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

// 允许用于替换为自定义的哈希函数
type Hash func(data []byte) uint32

type Map struct {
	hash     Hash
	replicas int   // 虚拟节点倍数
	keys     []int //哈希环
	hashMap  map[int]string
}

func New(replicas int, fn Hash) *Map {
	m := &Map{
		replicas: replicas,
		hash:     fn,
		hashMap:  make(map[int]string),
	}
	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE //默认为crc32算法
	}
	return m
}

// 添加真实节点
func (m *Map) Add(keys ...string) {
	for _, key := range keys {
		for i := 0; i < m.replicas; i++ {
			hash := int(m.hash([]byte(strconv.Itoa(i) + key))) //虚拟节点名称哈希值
			m.keys = append(m.keys, hash)                      //添加到环上
			m.hashMap[hash] = key                              //添加到哈希表中
		}
	}
	sort.Ints(m.keys)
}

// 获取真实节点
func (m *Map) Get(key string) string {
	if len(m.keys) == 0 {
		return ""
	}

	hash := int(m.hash([]byte(key)))
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash //找到第一个大于等于hash的节点
	})

	return m.hashMap[m.keys[idx%len(m.keys)]] //取余防止越界
}
