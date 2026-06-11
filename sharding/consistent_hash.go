package sharding

import (
	"hash/crc32"
)

// HashFunc định nghĩa kiểu hàm băm nhận vào mảng byte và trả về uint32.
type HashFunc func(data []byte) uint32

// ConsistentHashRing lưu trữ thông tin về vòng tròn băm (Consistent Hash Ring).
type ConsistentHashRing struct {
	hashFunc HashFunc       // Hàm băm được sử dụng (mặc định là crc32.ChecksumIEEE)
	replicas int            // Số lượng virtual node (node ảo) cho mỗi node thật
	keys     []int          // Các vị trí trên vòng tròn (lưu các giá trị hash đã được sắp xếp tăng dần)
	hashMap  map[int]string // Map dùng để ánh xạ từ giá trị hash tới tên của node (VD: 123456 -> "NodeA")
}

// New khởi tạo một ConsistentHashRing mới.
func New(replicas int, fn HashFunc) *ConsistentHashRing {
	ring := &ConsistentHashRing{
		replicas: replicas,
		hashFunc: fn,
		hashMap:  make(map[int]string),
	}
	// Nếu không truyền hàm băm tùy chỉnh thì dùng hàm mặc định crc32.ChecksumIEEE
	if ring.hashFunc == nil {
		ring.hashFunc = crc32.ChecksumIEEE
	}
	return ring
}
