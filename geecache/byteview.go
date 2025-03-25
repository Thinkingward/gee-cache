package geecache

import "time"

type ByteView struct {
	b []byte
	e time.Time
}

func (v ByteView) Len() int {
	return len(v.b)
}

func (v ByteView) Expire() time.Time {
	return v.e
}

func (v ByteView) ByteSlice() []byte { //b只读，返回一个拷贝，防止缓存值被外部程序修改
	return cloneBytes(v.b)
}

func (v ByteView) String() string {
	return string(v.b)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

func NewByteView(b []byte, e time.Time) *ByteView {
	return &ByteView{b: b, e: e}
}
