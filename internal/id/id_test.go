package id

import (
	"strings"
	"testing"
)

func TestGenerateID(t *testing.T) {
	id, err := GenerateID()
	if err != nil {
		t.Fatalf("GenerateID error: %v", err)
	}
	if len(id) != 21 {
		t.Errorf("ID length = %d, want 21", len(id))
	}
}

func TestGenerateID_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id, _ := GenerateID()
		if seen[id] {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		seen[id] = true
	}
}

func TestMustGenerateID(t *testing.T) {
	id := MustGenerateID()
	if len(id) != 21 {
		t.Errorf("ID length = %d, want 21", len(id))
	}
}

func TestGenerateOrderNo(t *testing.T) {
	orderNo := GenerateOrderNo()
	// Format: YYYYMMDDHHmmss (14) + 8 digits = 22 chars
	if len(orderNo) != 22 {
		t.Errorf("OrderNo length = %d, want 22", len(orderNo))
	}
	// Should start with "20" (year 20xx)
	if !strings.HasPrefix(orderNo, "20") {
		t.Errorf("OrderNo should start with 20, got %q", orderNo[:2])
	}
}

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
