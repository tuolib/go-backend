package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-backend/internal/apperr"
)

// TestSuccess 测试成功响应的格式是否正确。
// TestSuccess verifies the success response format is correct.
func TestSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/test", nil)

	err := Success(w, r, map[string]string{"hello": "world"})
	if err != nil {
		t.Fatalf("Success error: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// 反序列化响应体，检查各字段 / Deserialize response body and check each field
	var resp SuccessResp
	json.NewDecoder(w.Body).Decode(&resp)

	if !resp.Success {
		t.Error("expected success = true")
	}
	if resp.Code != 0 {
		t.Errorf("code = %d, want 0", resp.Code)
	}
	if resp.Message != "ok" {
		t.Errorf("message = %q, want %q", resp.Message, "ok")
	}
}

// TestPaginated 测试分页响应是否正确计算页码信息。
// TestPaginated tests that paginated responses correctly calculate pagination info.
func TestPaginated(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/test", nil)

	items := []string{"a", "b", "c"}
	err := Paginated(w, r, items, 30, 2, 10) // 总 30 条，第 2 页，每页 10 条 / Total 30 items, page 2, 10 per page
	if err != nil {
		t.Fatalf("Paginated error: %v", err)
	}

	var resp SuccessResp
	json.NewDecoder(w.Body).Decode(&resp)

	if !resp.Success {
		t.Error("expected success = true")
	}

	// resp.Data 是 any 类型，需要经过 JSON 重编码才能解析到具体结构体。
	// resp.Data is type any — needs JSON re-encoding to parse into a concrete struct.
	dataBytes, _ := json.Marshal(resp.Data)
	var pg PaginatedData
	json.Unmarshal(dataBytes, &pg)

	if pg.Pagination.Page != 2 {
		t.Errorf("page = %d, want 2", pg.Pagination.Page)
	}
	if pg.Pagination.Total != 30 {
		t.Errorf("total = %d, want 30", pg.Pagination.Total)
	}
	if pg.Pagination.TotalPages != 3 {
		t.Errorf("totalPages = %d, want 3", pg.Pagination.TotalPages)
	}
}

// TestHandleError_AppError 测试 HandleError 能正确处理 AppError 类型。
// TestHandleError_AppError tests that HandleError correctly handles AppError types.
func TestHandleError_AppError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/test", nil)

	appErr := apperr.NewNotFound("user", "test@example.com")
	HandleError(w, r, appErr)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}

	var resp ErrorResp
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Success {
		t.Error("expected success = false")
	}
	if resp.Code != apperr.ErrCodeNotFound {
		t.Errorf("code = %d, want %d", resp.Code, apperr.ErrCodeNotFound)
	}
	if resp.Meta == nil {
		t.Fatal("expected meta to be non-nil")
	}
}

// TestHandleError_UnknownError 测试非 AppError 类型的错误被统一转为 500。
// TestHandleError_UnknownError tests that non-AppError errors are uniformly converted to 500.
func TestHandleError_UnknownError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/test", nil)

	// errors.New 创建一个普通的 error，不是 AppError。
	// errors.New creates a plain error, not an AppError.
	HandleError(w, r, errors.New("something went wrong"))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	var resp ErrorResp
	json.NewDecoder(w.Body).Decode(&resp)

	// 普通错误的消息被替换为通用文本，防止泄露内部细节。
	// Plain error messages are replaced with generic text to prevent leaking internal details.
	if resp.Message != "internal server error" {
		t.Errorf("message = %q, want %q", resp.Message, "internal server error")
	}
}

// TestPaginated_TotalPagesRoundsUp 用表驱动测试验证向上取整的分页计算。
// TestPaginated_TotalPagesRoundsUp uses table-driven tests to verify ceiling division for pagination.
func TestPaginated_TotalPagesRoundsUp(t *testing.T) {
	tests := []struct {
		total, pageSize, want int
	}{
		{0, 10, 0},  // 0 条数据 → 0 页 / 0 items → 0 pages
		{1, 10, 1},  // 1 条 → 1 页 / 1 item → 1 page
		{10, 10, 1}, // 刚好整除 → 1 页 / Exact division → 1 page
		{11, 10, 2}, // 多 1 条 → 向上取整为 2 页 / 1 extra → rounds up to 2 pages
		{25, 10, 3}, // 25/10 = 2.5 → 向上取整为 3 页 / 25/10 = 2.5 → rounds up to 3 pages
	}
	for _, tt := range tests {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		Paginated(w, r, []string{}, tt.total, 1, tt.pageSize)

		var resp SuccessResp
		json.NewDecoder(w.Body).Decode(&resp)
		dataBytes, _ := json.Marshal(resp.Data)
		var pg PaginatedData
		json.Unmarshal(dataBytes, &pg)

		if pg.Pagination.TotalPages != tt.want {
			t.Errorf("total=%d pageSize=%d: totalPages=%d, want %d",
				tt.total, tt.pageSize, pg.Pagination.TotalPages, tt.want)
		}
	}
}
