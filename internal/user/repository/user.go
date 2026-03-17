package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"go-backend/internal/apperr"
	"go-backend/internal/database/gen"
)

// UserRepository 定义用户数据的访问接口。
// UserRepository defines the interface for user data access.
//
// 接口由使用方（service 层）定义 — 这是 Go 的惯例（与 Java 相反）。
// The interface is defined by the consumer (service layer) — this is Go convention (opposite of Java).
//
// 好处：service 层只依赖自己定义的接口，不依赖具体实现。
// 测试时可以用 mock 实现替换，不需要真实数据库。
// Benefits: service layer depends only on its own interface, not the concrete implementation.
// Tests can use mock implementations, no real database needed.
type UserRepository interface {
	// GetByID 按 ID 查用户。未找到返回 AppError（NotFound）。
	// GetByID finds a user by ID. Returns AppError (NotFound) if not found.
	GetByID(ctx context.Context, id string) (gen.UserServiceUser, error)

	// GetByEmail 按邮箱查用户。未找到返回 AppError（NotFound）。
	// GetByEmail finds a user by email. Returns AppError (NotFound) if not found.
	GetByEmail(ctx context.Context, email string) (gen.UserServiceUser, error)

	// Create 创建新用户。
	// Create creates a new user.
	Create(ctx context.Context, arg gen.CreateUserParams) (gen.UserServiceUser, error)

	// Update 更新用户资料。
	// Update updates a user's profile.
	Update(ctx context.Context, arg gen.UpdateUserParams) (gen.UserServiceUser, error)

	// UpdateLastLogin 更新最后登录时间。
	// UpdateLastLogin updates the last login timestamp.
	UpdateLastLogin(ctx context.Context, id string) error

	// EmailExists 检查邮箱是否已注册。注册前调用，避免 unique constraint 报错。
	// EmailExists checks whether an email is already registered. Called before registration to avoid unique constraint errors.
	EmailExists(ctx context.Context, email string) (bool, error)
}

// userRepository 是 UserRepository 接口的 PostgreSQL 实现。
// userRepository is the PostgreSQL implementation of UserRepository.
//
// 小写开头（未导出）— 外部只能通过 NewUserRepository() 构造函数获取，
// 并且只能通过 UserRepository 接口使用。这强制了面向接口编程。
// Lowercase (unexported) — external code can only obtain it via NewUserRepository(),
// and can only use it through the UserRepository interface. This enforces interface-based programming.
type userRepository struct {
	q *gen.Queries // sqlc 生成的查询对象 / sqlc-generated query object
}

// NewUserRepository 创建 UserRepository 实例。
// NewUserRepository creates a UserRepository instance.
//
// 参数是 gen.DBTX 接口（不是具体的 *pgxpool.Pool），这样同一个 repository
// 既能用连接池，也能用事务（pgx.Tx 也实现了 DBTX 接口）。
// The parameter is the gen.DBTX interface (not a concrete *pgxpool.Pool), so the same repository
// works with both a connection pool and a transaction (pgx.Tx also implements DBTX).
func NewUserRepository(db gen.DBTX) UserRepository {
	return &userRepository{q: gen.New(db)}
}

func (r *userRepository) GetByID(ctx context.Context, id string) (gen.UserServiceUser, error) {
	user, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		// pgx.ErrNoRows — pgx 在 QueryRow 找不到记录时返回的特定错误。
		// pgx.ErrNoRows — the specific error pgx returns when QueryRow finds no records.
		//
		// 类似 TS 里 query result 为 null，但 Go 不用 null 表示"没找到"，而是用错误。
		// Similar to a null query result in TS, but Go uses an error instead of null for "not found".
		if errors.Is(err, pgx.ErrNoRows) {
			return gen.UserServiceUser{}, apperr.New(apperr.ErrCodeUserNotFound, 404, "user not found")
		}
		return gen.UserServiceUser{}, err
	}
	return user, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (gen.UserServiceUser, error) {
	user, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return gen.UserServiceUser{}, apperr.New(apperr.ErrCodeUserNotFound, 404, "user not found")
		}
		return gen.UserServiceUser{}, err
	}
	return user, nil
}

func (r *userRepository) Create(ctx context.Context, arg gen.CreateUserParams) (gen.UserServiceUser, error) {
	return r.q.CreateUser(ctx, arg)
}

func (r *userRepository) Update(ctx context.Context, arg gen.UpdateUserParams) (gen.UserServiceUser, error) {
	user, err := r.q.UpdateUser(ctx, arg)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return gen.UserServiceUser{}, apperr.New(apperr.ErrCodeUserNotFound, 404, "user not found")
		}
		return gen.UserServiceUser{}, err
	}
	return user, nil
}

func (r *userRepository) UpdateLastLogin(ctx context.Context, id string) error {
	return r.q.UpdateLastLogin(ctx, id)
}

func (r *userRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	// 复用 GetUserByEmail 查询：找到了 → true，ErrNoRows → false，其他错误 → 上抛。
	// Reuse GetUserByEmail query: found → true, ErrNoRows → false, other errors → propagate.
	_, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// pgtype 转换辅助函数
// pgtype conversion helpers
// ──────────────────────────────────────────────────────────────────────────────

// TextToStringPtr 将 pgtype.Text 转为 *string。
// TextToStringPtr converts pgtype.Text to *string.
//
// pgtype.Text 是 pgx 对 SQL nullable 文本的表示：
//   - Valid=true  → 数据库里有值，String 是实际内容
//   - Valid=false → 数据库里是 NULL
//
// pgtype.Text is pgx's representation of SQL nullable text:
//   - Valid=true  → database has a value, String contains the actual content
//   - Valid=false → database value is NULL
//
// Go 标准库没有 nullable string，所以用 *string：nil 代表 NULL。
// Go's stdlib has no nullable string, so *string is used: nil represents NULL.
func TextToStringPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

// StringPtrToText 将 *string 转为 pgtype.Text。
// StringPtrToText converts *string to pgtype.Text.
func StringPtrToText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// TimestamptzToTimePtr 将 pgtype.Timestamptz 转为 *time.Time。
// TimestamptzToTimePtr converts pgtype.Timestamptz to *time.Time.
func TimestamptzToTimePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}
