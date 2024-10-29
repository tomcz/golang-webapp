package webapp

import (
	"bytes"
	"sync"
)

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

func BufBorrow() *bytes.Buffer {
	return bufPool.Get().(*bytes.Buffer)
}

func BufReturn(b *bytes.Buffer) {
	b.Reset()
	bufPool.Put(b)
}
