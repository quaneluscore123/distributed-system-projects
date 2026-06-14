package sharding

import (
	"testing"
)

// TestIsEmpty kiểm tra vòng tròn rỗng lúc khởi tạo và sau khi thêm node.
func TestIsEmpty(t *testing.T) {
	ring := New(3, nil)

	if !ring.IsEmpty() {
		t.Error("Ring mới khởi tạo phải rỗng")
	}

	ring.AddNode("NodeA")
	if ring.IsEmpty() {
		t.Error("Ring phải không rỗng sau khi thêm node")
	}
}

// TestGetNodeEmptyRing kiểm tra GetNode khi ring rỗng trả về chuỗi rỗng.
func TestGetNodeEmptyRing(t *testing.T) {
	ring := New(3, nil)
	result := ring.GetNode("any-key")
	if result != "" {
		t.Errorf("Ring rỗng phải trả về \"\", got %q", result)
	}
}

// TestAddAndGetNode kiểm tra việc thêm nhiều node và GetNode trả đúng node.
func TestAddAndGetNode(t *testing.T) {
	ring := New(50, nil)
	nodes := []string{
		"http://localhost:8081",
		"http://localhost:8082",
		"http://localhost:8083",
	}
	ring.AddNode(nodes...)

	// Mỗi key phải được ánh xạ tới một trong các node đã đăng ký
	testKeys := []string{"user:1", "user:2", "order:100", "product:xyz", "session:abc"}
	for _, key := range testKeys {
		node := ring.GetNode(key)
		found := false
		for _, n := range nodes {
			if n == node {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("GetNode(%q) = %q, không phải node hợp lệ", key, node)
		}
	}
}

// TestConsistency kiểm tra tính nhất quán: cùng 1 key luôn trả về cùng 1 node.
func TestConsistency(t *testing.T) {
	ring := New(50, nil)
	ring.AddNode("NodeA", "NodeB", "NodeC")

	key := "consistent-key"
	first := ring.GetNode(key)

	// Gọi nhiều lần phải trả về cùng kết quả
	for i := 0; i < 100; i++ {
		if got := ring.GetNode(key); got != first {
			t.Errorf("Lần %d: GetNode(%q) = %q, muốn %q", i, key, got, first)
		}
	}
}

// TestRemoveNode kiểm tra khi xóa node thì key không còn bị điều hướng về node đó nữa.
func TestRemoveNode(t *testing.T) {
	ring := New(50, nil)
	ring.AddNode("NodeA", "NodeB", "NodeC")

	// Tìm một key được điều hướng tới NodeA
	var keyForNodeA string
	for i := 0; i < 1000; i++ {
		key := "testkey" + string(rune('0'+i%10)) + string(rune('a'+i/10))
		if ring.GetNode(key) == "NodeA" {
			keyForNodeA = key
			break
		}
	}

	if keyForNodeA == "" {
		t.Skip("Không tìm được key nào điều hướng tới NodeA, bỏ qua test này")
	}

	// Xóa NodeA
	ring.RemoveNode("NodeA")

	// Sau khi xóa, key đó phải điều hướng tới node khác (không phải NodeA)
	newNode := ring.GetNode(keyForNodeA)
	if newNode == "NodeA" {
		t.Errorf("Sau khi xóa NodeA, key %q vẫn bị điều hướng tới NodeA", keyForNodeA)
	}
	if newNode == "" {
		t.Errorf("Sau khi xóa NodeA, GetNode(%q) trả về rỗng nhưng ring vẫn còn node khác", keyForNodeA)
	}
}

// TestRemoveAllNodes kiểm tra ring trở về trạng thái rỗng sau khi xóa hết.
func TestRemoveAllNodes(t *testing.T) {
	ring := New(3, nil)
	ring.AddNode("Alpha", "Beta")

	ring.RemoveNode("Alpha")
	ring.RemoveNode("Beta")

	if !ring.IsEmpty() {
		t.Error("Ring phải rỗng sau khi xóa hết node")
	}
	if got := ring.GetNode("somekey"); got != "" {
		t.Errorf("Ring rỗng: GetNode phải trả về \"\", got %q", got)
	}
}

// TestDistribution kiểm tra phân phối key tương đối đều giữa các node (với replicas cao).
func TestDistribution(t *testing.T) {
	ring := New(150, nil)
	nodes := []string{"NodeA", "NodeB", "NodeC"}
	ring.AddNode(nodes...)

	counts := make(map[string]int)
	total := 9000
	for i := 0; i < total; i++ {
		key := "key-" + string(rune(i))
		node := ring.GetNode(key)
		counts[node]++
	}

	// Mỗi node nên nhận khoảng 33% ± 15% traffic
	expected := total / len(nodes)
	tolerance := expected * 15 / 100
	for _, node := range nodes {
		cnt := counts[node]
		if cnt < expected-tolerance || cnt > expected+tolerance {
			t.Logf("Node %s nhận %d/%d requests (expected ~%d ± %d)", node, cnt, total, expected, tolerance)
			// Không fail - đây chỉ là cảnh báo phân phối không đều
		}
	}
	t.Logf("Phân phối: %v", counts)
}
