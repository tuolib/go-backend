package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"go-backend/internal/apperr"
	"go-backend/internal/database/gen"
)

// AddressRepository 定义地址数据的访问接口。
// AddressRepository defines the interface for address data access.
type AddressRepository interface {
	// ListByUser 获取用户的所有地址，默认地址排在前面。
	// ListByUser returns all addresses for a user, default address first.
	ListByUser(ctx context.Context, userID string) ([]gen.UserServiceUserAddress, error)

	// GetByID 按 ID + userID 查地址。同时校验归属关系，防止越权。
	// GetByID finds an address by ID + userID. Also verifies ownership to prevent unauthorized access.
	GetByID(ctx context.Context, id, userID string) (gen.UserServiceUserAddress, error)

	// Create 创建新地址。
	// Create creates a new address.
	Create(ctx context.Context, arg gen.CreateAddressParams) (gen.UserServiceUserAddress, error)

	// Update 更新地址。
	// Update updates an address.
	Update(ctx context.Context, arg gen.UpdateAddressParams) (gen.UserServiceUserAddress, error)

	// Delete 删除地址（物理删除）。
	// Delete removes an address (hard delete).
	Delete(ctx context.Context, id, userID string) error

	// CountByUser 统计用户地址数量，用于限制最大地址数。
	// CountByUser counts a user's addresses, used to enforce the maximum address limit.
	CountByUser(ctx context.Context, userID string) (int64, error)

	// ClearDefault 清除用户的所有默认地址标记。设置新默认前调用。
	// ClearDefault clears all default flags for a user. Called before setting a new default.
	ClearDefault(ctx context.Context, userID string) error

	// SetDefault 将指定地址设为默认。
	// SetDefault marks a specific address as the default.
	SetDefault(ctx context.Context, id, userID string) error
}

// addressRepository 是 AddressRepository 的 PostgreSQL 实现。
// addressRepository is the PostgreSQL implementation of AddressRepository.
type addressRepository struct {
	q *gen.Queries
}

// NewAddressRepository 创建 AddressRepository 实例。
// NewAddressRepository creates an AddressRepository instance.
func NewAddressRepository(db gen.DBTX) AddressRepository {
	return &addressRepository{q: gen.New(db)}
}

func (r *addressRepository) ListByUser(ctx context.Context, userID string) ([]gen.UserServiceUserAddress, error) {
	return r.q.ListAddressesByUser(ctx, userID)
}

func (r *addressRepository) GetByID(ctx context.Context, id, userID string) (gen.UserServiceUserAddress, error) {
	addr, err := r.q.GetAddressByID(ctx, gen.GetAddressByIDParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return gen.UserServiceUserAddress{}, apperr.NewNotFound("address", id)
		}
		return gen.UserServiceUserAddress{}, err
	}
	return addr, nil
}

func (r *addressRepository) Create(ctx context.Context, arg gen.CreateAddressParams) (gen.UserServiceUserAddress, error) {
	return r.q.CreateAddress(ctx, arg)
}

func (r *addressRepository) Update(ctx context.Context, arg gen.UpdateAddressParams) (gen.UserServiceUserAddress, error) {
	addr, err := r.q.UpdateAddress(ctx, arg)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return gen.UserServiceUserAddress{}, apperr.NewNotFound("address", arg.ID)
		}
		return gen.UserServiceUserAddress{}, err
	}
	return addr, nil
}

func (r *addressRepository) Delete(ctx context.Context, id, userID string) error {
	return r.q.DeleteAddress(ctx, gen.DeleteAddressParams{
		ID:     id,
		UserID: userID,
	})
}

func (r *addressRepository) CountByUser(ctx context.Context, userID string) (int64, error) {
	return r.q.CountAddressesByUser(ctx, userID)
}

func (r *addressRepository) ClearDefault(ctx context.Context, userID string) error {
	return r.q.ClearDefaultAddress(ctx, userID)
}

func (r *addressRepository) SetDefault(ctx context.Context, id, userID string) error {
	return r.q.SetDefaultAddress(ctx, gen.SetDefaultAddressParams{
		ID:     id,
		UserID: userID,
	})
}
