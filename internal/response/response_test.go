package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-backend/internal/apperr"
)

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

func TestPaginated(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/test", nil)

	items := []string{"a", "b", "c"}
	err := Paginated(w, r, items, 30, 2, 10)
	if err != nil {
		t.Fatalf("Paginated error: %v", err)
	}

	var resp SuccessResp
	json.NewDecoder(w.Body).Decode(&resp)

	if !resp.Success {
		t.Error("expected success = true")
	}

	// Check pagination in data
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

func TestHandleError_UnknownError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/test", nil)

	HandleError(w, r, errors.New("something went wrong"))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	var resp ErrorResp
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Message != "internal server error" {
		t.Errorf("message = %q, want %q", resp.Message, "internal server error")
	}
}

func TestPaginated_TotalPagesRoundsUp(t *testing.T) {
	tests := []struct {
		total, pageSize, want int
	}{
		{0, 10, 0},
		{1, 10, 1},
		{10, 10, 1},
		{11, 10, 2},
		{25, 10, 3},
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
