package sharding

import (
	"fmt"
	"hash/crc32"
	"sort"
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

// AddNode thêm một hoặc nhiều node thật vào vòng tròn băm.
// Với mỗi node thật, hàm tạo ra `replicas` node ảo được phân bố đều trên vòng tròn.
// Ví dụ: AddNode("NodeA") với replicas=3 sẽ tạo ra các node ảo:
//   - hash("0NodeA") -> vị trí X trên vòng tròn
//   - hash("1NodeA") -> vị trí Y trên vòng tròn
//   - hash("2NodeA") -> vị trí Z trên vòng tròn
func (r *ConsistentHashRing) AddNode(nodes ...string) {
	for _, node := range nodes {
		for i := 0; i < r.replicas; i++ {
			// Tạo tên node ảo bằng cách ghép index + tên node thật
			virtualNodeName := fmt.Sprintf("%d%s", i, node)
			hash := int(r.hashFunc([]byte(virtualNodeName)))

			// Thêm vị trí hash vào danh sách các key trên vòng tròn
			r.keys = append(r.keys, hash)

			// Ánh xạ từ vị trí hash -> tên node thật
			r.hashMap[hash] = node
		}
	}

	// Sắp xếp các vị trí tăng dần để có thể tìm kiếm nhị phân (binary search)
	sort.Ints(r.keys)
}

// RemoveNode xóa một node thật và toàn bộ node ảo của nó ra khỏi vòng tròn băm.
// Khi một server bị tắt hoặc xóa, hàm này đảm bảo không còn request nào bị điều hướng tới server đó.
func (r *ConsistentHashRing) RemoveNode(node string) {
	for i := 0; i < r.replicas; i++ {
		virtualNodeName := fmt.Sprintf("%d%s", i, node)
		hash := int(r.hashFunc([]byte(virtualNodeName)))

		// Xóa vị trí hash khỏi slice keys bằng binary search
		idx := sort.SearchInts(r.keys, hash)
		if idx < len(r.keys) && r.keys[idx] == hash {
			r.keys = append(r.keys[:idx], r.keys[idx+1:]...)
		}

		// Xóa ánh xạ khỏi hashMap
		delete(r.hashMap, hash)
	}
}

// GetNode tìm node đích phù hợp nhất để xử lý một key.
//
// Thuật toán:
//  1. Tính giá trị hash của key.
//  2. Dùng binary search tìm vị trí đầu tiên trên vòng tròn >= hash(key).
//  3. Nếu không tìm thấy (hash(key) lớn hơn tất cả vị trí), quay về node đầu tiên (index 0) —
//     đây chính là tính chất "vòng tròn" của Consistent Hashing.
//
// Trả về tên node đích (ví dụ: "http://localhost:8081"), hoặc "" nếu ring rỗng.
func (r *ConsistentHashRing) GetNode(key string) string {
	if r.IsEmpty() {
		return ""
	}

	hash := int(r.hashFunc([]byte(key)))

	// Tìm vị trí đầu tiên trên vòng tròn có giá trị >= hash(key)
	// sort.SearchInts trả về index i sao cho r.keys[i] >= hash
	idx := sort.SearchInts(r.keys, hash)

	// Nếu hash(key) lớn hơn tất cả các điểm trên vòng tròn,
	// quay vòng về node đầu tiên (index 0)
	if idx == len(r.keys) {
		idx = 0
	}

	return r.hashMap[r.keys[idx]]
}

// IsEmpty kiểm tra xem vòng tròn băm có đang rỗng (không có node nào) hay không.
func (r *ConsistentHashRing) IsEmpty() bool {
	return len(r.keys) == 0
}

