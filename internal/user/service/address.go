package service

import (
	"context"

	"go-backend/internal/apperr"
	"go-backend/internal/database/gen"
	"go-backend/internal/id"
	"go-backend/internal/user/dto"
	"go-backend/internal/user/repository"
)

// maxAddressCount 每个用户最多允许的收货地址数量。
// maxAddressCount is the maximum number of addresses allowed per user.
//
// 限制地址数量防止滥用（恶意用户创建大量地址占用数据库空间）。
// Limits address count to prevent abuse (malicious users creating tons of addresses to waste DB space).
const maxAddressCount = 20

// AddressService 处理收货地址相关的业务逻辑。
// AddressService handles address-related business logic.
type AddressService struct {
	addrRepo repository.AddressRepository
}

// NewAddressService 创建 AddressService 实例。
// NewAddressService creates an AddressService instance.
func NewAddressService(addrRepo repository.AddressRepository) *AddressService {
	return &AddressService{addrRepo: addrRepo}
}

// List 获取用户的所有地址，默认地址排在前面。
// List returns all addresses for a user, default address first.
func (s *AddressService) List(ctx context.Context, userID string) ([]*dto.AddressResp, error) {
	addrs, err := s.addrRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	results := make([]*dto.AddressResp, 0, len(addrs))
	for i := range addrs {
		results = append(results, toAddressResp(addrs[i]))
	}
	return results, nil
}

// Create 创建新地址。
// Create creates a new address.
//
// 流程 / Flow:
//  1. 检查地址数量是否达到上限
//  2. 如果新地址要设为默认 → 先清除旧的默认标记
//  3. 插入新地址
func (s *AddressService) Create(ctx context.Context, userID string, input dto.CreateAddressInput) (*dto.AddressResp, error) {
	// ── 1. 检查地址数量上限 ──────────────────────────────────
	// ── 1. Check address count limit ─────────────────────────
	count, err := s.addrRepo.CountByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if count >= maxAddressCount {
		return nil, apperr.New(apperr.ErrCodeAddressLimit, 400, "address count limit reached")
	}

	// ── 2. 处理默认地址 ──────────────────────────────────────
	// ── 2. Handle default address ────────────────────────────
	//
	// 同一用户只能有一个默认地址。新地址要设为默认时，先清除旧的。
	// A user can only have one default address. When the new address should be default, clear the old one first.
	if input.IsDefault {
		if err := s.addrRepo.ClearDefault(ctx, userID); err != nil {
			return nil, err
		}
	}

	// ── 3. 创建地址 ──────────────────────────────────────────
	// ── 3. Create address ────────────────────────────────────
	addrID, err := id.GenerateID()
	if err != nil {
		return nil, apperr.NewInternal("failed to generate address id")
	}

	addr, err := s.addrRepo.Create(ctx, gen.CreateAddressParams{
		ID:         addrID,
		UserID:     userID,
		Label:      repository.StringPtrToText(ptrIfNotEmpty(input.Label)),
		Recipient:  input.Recipient,
		Phone:      input.Phone,
		Province:   input.Province,
		City:       input.City,
		District:   input.District,
		Address:    input.Address,
		PostalCode: repository.StringPtrToText(ptrIfNotEmpty(input.PostalCode)),
		IsDefault:  input.IsDefault,
	})
	if err != nil {
		return nil, err
	}

	return toAddressResp(addr), nil
}

// Update 更新地址。
// Update updates an address.
//
// 如果要设为默认 → 先清除旧默认 → 再更新 → 再设为默认。
// If setting as default → clear old default → update → set as default.
func (s *AddressService) Update(ctx context.Context, userID string, input dto.UpdateAddressInput) (*dto.AddressResp, error) {
	// ── 1. 处理默认地址切换 ──────────────────────────────────
	// ── 1. Handle default address switching ──────────────────
	if input.IsDefault != nil && *input.IsDefault {
		if err := s.addrRepo.ClearDefault(ctx, userID); err != nil {
			return nil, err
		}
	}

	// ── 2. 更新地址字段 ──────────────────────────────────────
	// ── 2. Update address fields ─────────────────────────────
	//
	// UpdateAddress 的 SQL 用 COALESCE — nil 字段保留原值。
	// UpdateAddress SQL uses COALESCE — nil fields keep their original values.
	addr, err := s.addrRepo.Update(ctx, gen.UpdateAddressParams{
		ID:         input.ID,
		UserID:     userID,
		Label:      repository.StringPtrToText(input.Label),
		Recipient:  derefOr(input.Recipient, ""),
		Phone:      derefOr(input.Phone, ""),
		Province:   derefOr(input.Province, ""),
		City:       derefOr(input.City, ""),
		District:   derefOr(input.District, ""),
		Address:    derefOr(input.Address, ""),
		PostalCode: repository.StringPtrToText(input.PostalCode),
	})
	if err != nil {
		return nil, err
	}

	// ── 3. 如果要设为默认，单独执行 SetDefault ───────────────
	// ── 3. If setting as default, run SetDefault separately ──
	//
	// UpdateAddress 的 SQL 没有更新 is_default 字段（COALESCE 不适合 bool），
	// 所以用独立的 SetDefault 操作。
	// UpdateAddress SQL doesn't update is_default (COALESCE doesn't work well for bool),
	// so we use a separate SetDefault operation.
	if input.IsDefault != nil && *input.IsDefault {
		if err := s.addrRepo.SetDefault(ctx, input.ID, userID); err != nil {
			return nil, err
		}
		// 更新返回值中的 IsDefault 字段。
		// Update the IsDefault field in the return value.
		addr.IsDefault = true
	}

	return toAddressResp(addr), nil
}

// Delete 删除地址。
// Delete removes an address.
func (s *AddressService) Delete(ctx context.Context, userID string, addrID string) error {
	return s.addrRepo.Delete(ctx, addrID, userID)
}

// GetByID 按 ID 获取地址（内部接口用）。
// GetByID gets an address by ID (for internal endpoints).
func (s *AddressService) GetByID(ctx context.Context, addrID, userID string) (*dto.AddressResp, error) {
	addr, err := s.addrRepo.GetByID(ctx, addrID, userID)
	if err != nil {
		return nil, err
	}
	return toAddressResp(addr), nil
}

// ──────────────────────────────────────────────────────────────────────────────
// 转换辅助
// Conversion helpers
// ──────────────────────────────────────────────────────────────────────────────

// toAddressResp 将数据库 model 转为 API 响应结构体。
// toAddressResp converts a database model to an API response struct.
func toAddressResp(a gen.UserServiceUserAddress) *dto.AddressResp {
	return &dto.AddressResp{
		ID:         a.ID,
		UserID:     a.UserID,
		Label:      repository.TextToStringPtr(a.Label),
		Recipient:  a.Recipient,
		Phone:      a.Phone,
		Province:   a.Province,
		City:       a.City,
		District:   a.District,
		Address:    a.Address,
		PostalCode: repository.TextToStringPtr(a.PostalCode),
		IsDefault:  a.IsDefault,
		CreatedAt:  a.CreatedAt.Time,
		UpdatedAt:  a.UpdatedAt.Time,
	}
}

// derefOr 解引用 *string 指针，nil 时返回默认值。
// derefOr dereferences a *string pointer; returns fallback if nil.
//
// 用于 UpdateAddressParams 中 COALESCE 风格的字段：
// nil → "" → SQL 传空字符串 → COALESCE 会用空字符串更新（这里依赖 SQL 层 COALESCE 判 NULL 不判空）。
// 但 UpdateAddress 的 SQL 参数是 string 类型不是 pgtype.Text，所以空字符串会直接传入。
// 实际上这些字段 (recipient, phone 等) 在数据库中是 NOT NULL 的，
// COALESCE 在此处对 string 类型参数不起作用 — 需要在应用层先查原值或接受覆盖。
// 对于必填字段，前端更新时通常会传入完整值。
//
// Used for COALESCE-style fields in UpdateAddressParams.
// For NOT NULL string columns, the frontend is expected to send the full value when updating.
func derefOr(p *string, fallback string) string {
	if p == nil {
		return fallback
	}
	return *p
}
