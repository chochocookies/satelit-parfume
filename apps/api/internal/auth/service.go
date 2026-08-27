package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"satelit-parfume-api/internal/customers"
	"satelit-parfume-api/internal/users"
	"satelit-parfume-api/pkg/jwt"
	"satelit-parfume-api/pkg/password"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrTooManyAttempts    = errors.New("too many login attempts, try again later")
)

const (
	maxLoginAttempts   = 5
	loginAttemptWindow = 15 * time.Minute
)

type Service struct {
	users              *users.Repository
	customers          *customers.Repository
	refreshTokens      *Repository
	accessTokens       *jwt.Manager
	refreshTokenIssuer *jwt.Manager
	redis              *redis.Client
}

type ServiceDeps struct {
	Users               *users.Repository
	Customers           *customers.Repository
	RefreshTokens       *Repository
	AccessTokenManager  *jwt.Manager
	RefreshTokenManager *jwt.Manager
	Redis               *redis.Client
}

func NewService(d ServiceDeps) *Service {
	return &Service{
		users:              d.Users,
		customers:          d.Customers,
		refreshTokens:      d.RefreshTokens,
		accessTokens:       d.AccessTokenManager,
		refreshTokenIssuer: d.RefreshTokenManager,
		redis:              d.Redis,
	}
}

// Register creates a new customer account and immediately logs it in —
// there is deliberately no separate staff self-registration: staff
// accounts are provisioned by an admin (Phase 9), not signed up for.
func (s *Service) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error) {
	hash, err := password.Hash(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	c, err := s.customers.Create(ctx, req.Name, req.Email, req.Phone, hash)
	if err != nil {
		return nil, err // customers.ErrEmailTaken surfaces to the handler as-is
	}

	return s.issueTokens(ctx, c.ID, "customer", c.Name, c.Email, nil)
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	if err := s.checkRateLimit(ctx, "customer", req.Email); err != nil {
		return nil, err
	}

	c, err := s.customers.GetByEmail(ctx, req.Email)
	if err != nil {
		s.recordFailedAttempt(ctx, "customer", req.Email)
		return nil, ErrInvalidCredentials
	}
	if !password.Verify(c.PasswordHash, req.Password) || !c.IsActive() {
		s.recordFailedAttempt(ctx, "customer", req.Email)
		return nil, ErrInvalidCredentials
	}

	s.clearAttempts(ctx, "customer", req.Email)
	return s.issueTokens(ctx, c.ID, "customer", c.Name, c.Email, nil)
}

func (s *Service) StaffLogin(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	if err := s.checkRateLimit(ctx, "user", req.Email); err != nil {
		return nil, err
	}

	u, err := s.users.GetByEmail(ctx, req.Email)
	if err != nil {
		s.recordFailedAttempt(ctx, "user", req.Email)
		return nil, ErrInvalidCredentials
	}
	if !password.Verify(u.PasswordHash, req.Password) || !u.IsActive() {
		s.recordFailedAttempt(ctx, "user", req.Email)
		return nil, ErrInvalidCredentials
	}

	s.clearAttempts(ctx, "user", req.Email)
	return s.issueTokens(ctx, u.ID, "user", u.Name, u.Email, u.Roles)
}

// Refresh verifies the refresh JWT, atomically consumes it (see
// Repository.ConsumeRefreshToken), re-checks the account is still active,
// and issues a brand new access/refresh pair. The old refresh token is
// unusable after this call even if it hasn't expired yet — that's what
// "rotation" means.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*AuthResponse, error) {
	claims, err := s.refreshTokenIssuer.Verify(refreshToken)
	if err != nil {
		return nil, ErrRefreshTokenInvalid
	}

	if err := s.refreshTokens.ConsumeRefreshToken(ctx, claims.ID); err != nil {
		return nil, err
	}

	var name, email string
	var roles []string

	switch claims.SubjectType {
	case "user":
		u, err := s.users.GetByID(ctx, claims.Subject)
		if err != nil || !u.IsActive() {
			return nil, ErrInvalidCredentials
		}
		name, email, roles = u.Name, u.Email, u.Roles
	case "customer":
		c, err := s.customers.GetByID(ctx, claims.Subject)
		if err != nil || !c.IsActive() {
			return nil, ErrInvalidCredentials
		}
		name, email = c.Name, c.Email
	default:
		return nil, ErrRefreshTokenInvalid
	}

	return s.issueTokens(ctx, claims.Subject, claims.SubjectType, name, email, roles)
}

// Logout revokes the refresh token. An already-invalid token still
// results in a successful logout — there's nothing left to revoke, and
// the client's intent (end this session) is already satisfied.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	claims, err := s.refreshTokenIssuer.Verify(refreshToken)
	if err != nil {
		return nil
	}
	return s.refreshTokens.RevokeRefreshToken(ctx, claims.ID)
}

// GetSubject re-fetches the current, live record rather than trusting the
// access token's claims — roles or status may have changed since the
// token was issued, and /me should reflect the truth now, not at issue
// time.
func (s *Service) GetSubject(ctx context.Context, subjectType, subjectID string) (*Subject, error) {
	switch subjectType {
	case "user":
		u, err := s.users.GetByID(ctx, subjectID)
		if err != nil {
			return nil, err
		}
		return &Subject{ID: u.ID, Type: "user", Name: u.Name, Email: u.Email, Roles: u.Roles}, nil
	case "customer":
		c, err := s.customers.GetByID(ctx, subjectID)
		if err != nil {
			return nil, err
		}
		return &Subject{ID: c.ID, Type: "customer", Name: c.Name, Email: c.Email}, nil
	default:
		return nil, ErrRefreshTokenInvalid
	}
}

func (s *Service) issueTokens(ctx context.Context, subjectID, subjectType, name, email string, roles []string) (*AuthResponse, error) {
	access, _, accessExp, err := s.accessTokens.Issue(subjectID, subjectType, roles)
	if err != nil {
		return nil, fmt.Errorf("issue access token: %w", err)
	}

	refresh, refreshJTI, refreshExp, err := s.refreshTokenIssuer.Issue(subjectID, subjectType, roles)
	if err != nil {
		return nil, fmt.Errorf("issue refresh token: %w", err)
	}

	if err := s.refreshTokens.StoreRefreshToken(ctx, refreshJTI, subjectType, subjectID, refreshExp); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &AuthResponse{
		Subject: Subject{ID: subjectID, Type: subjectType, Name: name, Email: email, Roles: roles},
		Tokens: TokenPair{
			AccessToken:           access,
			AccessTokenExpiresAt:  accessExp,
			RefreshToken:          refresh,
			RefreshTokenExpiresAt: refreshExp,
		},
	}, nil
}

// ── Redis-backed login rate limiting (section 52's "rate limiting") ──
//
// Deliberately simple: a per-(subject type, email) counter with a fixed
// window, not a sliding-window/token-bucket algorithm. Good enough to
// blunt brute-force attempts against one account; an IP-based layer in
// front (e.g. at Nginx) is complementary, not replaced by this.

func (s *Service) checkRateLimit(ctx context.Context, subjectType, email string) error {
	count, err := s.redis.Get(ctx, rateLimitKey(subjectType, email)).Int()
	if err != nil && !errors.Is(err, redis.Nil) {
		// A Redis hiccup shouldn't lock everyone out of logging in.
		return nil
	}
	if count >= maxLoginAttempts {
		return ErrTooManyAttempts
	}
	return nil
}

func (s *Service) recordFailedAttempt(ctx context.Context, subjectType, email string) {
	key := rateLimitKey(subjectType, email)
	pipe := s.redis.TxPipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, loginAttemptWindow)
	_, _ = pipe.Exec(ctx)
}

func (s *Service) clearAttempts(ctx context.Context, subjectType, email string) {
	_ = s.redis.Del(ctx, rateLimitKey(subjectType, email)).Err()
}

func rateLimitKey(subjectType, email string) string {
	return fmt.Sprintf("login_attempts:%s:%s", subjectType, email)
}
