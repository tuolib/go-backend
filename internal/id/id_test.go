package id

import (
	"strings"
	"testing"
)

// TestGenerateID 测试生成的 nanoid 长度为 21 字符。
// TestGenerateID tests that the generated nanoid is 21 characters long.
func TestGenerateID(t *testing.T) {
	id, err := GenerateID()
	if err != nil {
		t.Fatalf("GenerateID error: %v", err)
	}
	if len(id) != 21 {
		t.Errorf("ID length = %d, want 21", len(id))
	}
}

// TestGenerateID_Unique 生成 1000 个 ID 确保没有重复。
// TestGenerateID_Unique generates 1000 IDs and ensures no duplicates.
func TestGenerateID_Unique(t *testing.T) {
	// map[string]bool 用作集合（set），Go 没有内置的 set 类型。
	// map[string]bool is used as a set — Go has no built-in set type.
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id, _ := GenerateID()
		if seen[id] {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		seen[id] = true
	}
}

// TestMustGenerateID 测试 Must 版本（不返回 error，出错直接 panic）。
// TestMustGenerateID tests the Must variant (no error return, panics on failure).
func TestMustGenerateID(t *testing.T) {
	id := MustGenerateID()
	if len(id) != 21 {
		t.Errorf("ID length = %d, want 21", len(id))
	}
}

// TestGenerateOrderNo 测试订单号格式：22 字符，以 "20" 开头。
// TestGenerateOrderNo tests order number format: 22 chars, starts with "20".
func TestGenerateOrderNo(t *testing.T) {
	orderNo := GenerateOrderNo()
	// 格式：YYYYMMDDHHmmss(14位) + 随机数(8位) = 22 字符
	// Format: YYYYMMDDHHmmss (14 digits) + random (8 digits) = 22 chars
	if len(orderNo) != 22 {
		t.Errorf("OrderNo length = %d, want 22", len(orderNo))
	}
	// 以 "20" 开头说明年份正确（20xx 年）/ Starts with "20" confirms correct year (20xx)
	if !strings.HasPrefix(orderNo, "20") {
		t.Errorf("OrderNo should start with 20, got %q", orderNo[:2])
	}
}

// TestGenerateOrderNo_Unique 生成 100 个订单号确保没有重复。
// TestGenerateOrderNo_Unique generates 100 order numbers and ensures no duplicates.
func TestGenerateOrderNo_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		no := GenerateOrderNo()
		if seen[no] {
			t.Fatalf("duplicate OrderNo: %s", no)
		}
		seen[no] = true
	}
}
