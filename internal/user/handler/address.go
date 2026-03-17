package handler

import (
	"net/http"

	"go-backend/internal/middleware"
	"go-backend/internal/response"
	"go-backend/internal/user/dto"
	"go-backend/internal/user/service"
)

// AddressHandler 处理收货地址相关的 HTTP 请求。
// AddressHandler handles address-related HTTP requests.
type AddressHandler struct {
	addrService *service.AddressService
}

// NewAddressHandler 创建 AddressHandler 实例。
// NewAddressHandler creates an AddressHandler instance.
func NewAddressHandler(addrService *service.AddressService) *AddressHandler {
	return &AddressHandler{addrService: addrService}
}

// List 获取当前用户的所有地址。
// List returns all addresses for the current user.
//
// POST /api/v1/user/address/list（需要认证）
// POST /api/v1/user/address/list (requires auth)
func (h *AddressHandler) List(w http.ResponseWriter, r *http.Request) error {
	userID := middleware.UserIDFrom(r.Context())

	result, err := h.addrService.List(r.Context(), userID)
	if err != nil {
		return err
	}

	return response.Success(w, r, result)
}

// Create 新增收货地址。
// Create adds a new address.
//
// POST /api/v1/user/address/create（需要认证）
// POST /api/v1/user/address/create (requires auth)
func (h *AddressHandler) Create(w http.ResponseWriter, r *http.Request) error {
	userID := middleware.UserIDFrom(r.Context())

	var input dto.CreateAddressInput
	if err := decodeAndValidate(r, &input); err != nil {
		return err
	}

	result, err := h.addrService.Create(r.Context(), userID, input)
	if err != nil {
		return err
	}

	return response.Success(w, r, result)
}

// Update 更新收货地址。
// Update modifies an existing address.
//
// POST /api/v1/user/address/update（需要认证）
// POST /api/v1/user/address/update (requires auth)
func (h *AddressHandler) Update(w http.ResponseWriter, r *http.Request) error {
	userID := middleware.UserIDFrom(r.Context())

	var input dto.UpdateAddressInput
	if err := decodeAndValidate(r, &input); err != nil {
		return err
	}

	result, err := h.addrService.Update(r.Context(), userID, input)
	if err != nil {
		return err
	}

	return response.Success(w, r, result)
}

// Delete 删除收货地址。
// Delete removes an address.
//
// POST /api/v1/user/address/delete（需要认证）
// POST /api/v1/user/address/delete (requires auth)
func (h *AddressHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	userID := middleware.UserIDFrom(r.Context())

	var input dto.DeleteAddressInput
	if err := decodeAndValidate(r, &input); err != nil {
		return err
	}

	if err := h.addrService.Delete(r.Context(), userID, input.ID); err != nil {
		return err
	}

	return response.Success(w, r, nil)
}

// ──────────────────────────────────────────────────────────────────────────────
// 内部接口
// Internal endpoint
// ──────────────────────────────────────────────────────────────────────────────

// AddressDetail 内部接口：按 ID + userID 获取地址。
// AddressDetail is an internal endpoint: get address by ID + userID.
//
// POST /internal/user/address/detail
//
// 调用方：订单服务创建订单时需要获取收货地址详情。
// Caller: order service needs address details when creating an order.
func (h *AddressHandler) AddressDetail(w http.ResponseWriter, r *http.Request) error {
	var input dto.GetAddressDetailInput
	if err := decodeAndValidate(r, &input); err != nil {
		return err
	}

	result, err := h.addrService.GetByID(r.Context(), input.AddressID, input.UserID)
	if err != nil {
		return err
	}

	return response.Success(w, r, result)
}
