// Package service contains business logic sitting between handlers and repositories.
package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"nowdone/internal/config"
	"nowdone/internal/models"
	"nowdone/internal/repository"
)

var (
	ErrInvalidCode    = errors.New("invalid or expired code")
	ErrCodeNotReady   = errors.New("code not yet confirmed via telegram")
	ErrInvalidPIN     = errors.New("invalid pin")
	ErrPINAlreadySet  = errors.New("pin already set")
)

// ResetCodeSender delivers a PIN-reset code straight to the user's Telegram
// chat. Implemented by *TelegramCodeSender in the API process; the bot/worker
// pass nil since they never originate a reset from the site.
type ResetCodeSender interface {
	SendResetCode(chatID int64, code string) error
}

// AuthService implements Telegram deep-link login, JWT issuance and PIN management.
type AuthService struct {
	users     *repository.UserRepository
	authCodes *repository.AuthCodeRepository
	cfg       *config.Config
	sender    ResetCodeSender
}

func NewAuthService(users *repository.UserRepository, authCodes *repository.AuthCodeRepository, cfg *config.Config, sender ResetCodeSender) *AuthService {
	return &AuthService{users: users, authCodes: authCodes, cfg: cfg, sender: sender}
}

// GenerateLoginCode creates a new 6-digit code and returns the bot deep link.
func (s *AuthService) GenerateLoginCode(ctx context.Context) (code string, deepLink string, err error) {
	code, err = randomDigits(6)
	if err != nil {
		return "", "", fmt.Errorf("generate code: %w", err)
	}
	if _, err := s.authCodes.Create(ctx, code, models.PurposeAuth, nil, s.cfg.AuthCodeTTL); err != nil {
		return "", "", fmt.Errorf("store auth code: %w", err)
	}
	deepLink = fmt.Sprintf("https://t.me/%s?start=auth_%s", s.cfg.BotUsername, code)
	return code, deepLink, nil
}

// GenerateResetCode creates a 6-digit PIN-reset code for an already-authenticated
// user and delivers it straight to their Telegram chat — no bot /start, no deep
// link. The code is bound to the user's chat immediately (so it is redeemable
// without the bot round-trip) and expires after cfg.ResetCodeTTL (5 minutes).
func (s *AuthService) GenerateResetCode(ctx context.Context, userID uuid.UUID) (code string, err error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("get user: %w", err)
	}

	code, err = randomDigits(6)
	if err != nil {
		return "", fmt.Errorf("generate code: %w", err)
	}

	id := userID
	if _, err := s.authCodes.Create(ctx, code, models.PurposeReset, &id, s.cfg.ResetCodeTTL); err != nil {
		return "", fmt.Errorf("store reset code: %w", err)
	}

	// Pre-confirm the code against the user's own Telegram chat. For a private
	// chat the chat ID equals the user's Telegram ID. This is what the bot used
	// to do on /start; we do it up front because the code is delivered directly.
	if err := s.authCodes.ConfirmByCode(ctx, code, models.PurposeReset, userID, user.TelegramID); err != nil {
		return "", fmt.Errorf("confirm reset code: %w", err)
	}

	if s.sender != nil {
		if err := s.sender.SendResetCode(user.TelegramID, code); err != nil {
			return "", fmt.Errorf("send reset code: %w", err)
		}
	}
	return code, nil
}

// ConfirmCodeFromBot is called by the Telegram bot handler when a user opens
// the deep link and presses Start. It binds the code to the user's chat so
// the code can then be delivered back to them and later redeemed on the site.
func (s *AuthService) ConfirmCodeFromBot(ctx context.Context, code string, purpose models.AuthCodePurpose, telegramID int64, username, firstName string, chatID int64) (*models.User, error) {
	ac, err := s.authCodes.GetByCode(ctx, code, purpose)
	if err != nil {
		return nil, ErrInvalidCode
	}
	if ac.IsUsed || ac.IsExpired() {
		return nil, ErrInvalidCode
	}

	user, err := s.users.GetOrCreateByTelegramID(ctx, telegramID, username, firstName)
	if err != nil {
		return nil, fmt.Errorf("get or create user: %w", err)
	}

	// For reset codes, the code was already tied to a specific user_id; only
	// allow confirmation if it's the same Telegram account.
	if purpose == models.PurposeReset && ac.UserID != nil && *ac.UserID != user.ID {
		return nil, ErrInvalidCode
	}

	if err := s.authCodes.ConfirmByCode(ctx, code, purpose, user.ID, chatID); err != nil {
		return nil, fmt.Errorf("confirm code: %w", err)
	}
	return user, nil
}

// RedeemCode is called by the website after the user types in the code they
// received from the bot. It issues a JWT on success.
func (s *AuthService) RedeemCode(ctx context.Context, code string) (token string, user *models.User, err error) {
	ac, err := s.authCodes.GetByCode(ctx, code, models.PurposeAuth)
	if err != nil {
		return "", nil, ErrInvalidCode
	}
	if ac.IsUsed || ac.IsExpired() {
		return "", nil, ErrInvalidCode
	}
	if !ac.Confirmed() || ac.UserID == nil {
		return "", nil, ErrCodeNotReady
	}

	user, err = s.users.GetByID(ctx, *ac.UserID)
	if err != nil {
		return "", nil, fmt.Errorf("get user: %w", err)
	}

	if err := s.authCodes.MarkUsed(ctx, ac.ID); err != nil {
		return "", nil, fmt.Errorf("mark code used: %w", err)
	}

	token, err = s.IssueJWT(user.ID)
	if err != nil {
		return "", nil, err
	}
	return token, user, nil
}

// RedeemResetCode validates a PIN-reset code without issuing a JWT; the
// caller must already be authenticated (or we trust the code as a short-lived
// bearer for the reset flow).
func (s *AuthService) RedeemResetCode(ctx context.Context, code string) (*models.User, error) {
	ac, err := s.authCodes.GetByCode(ctx, code, models.PurposeReset)
	if err != nil {
		return nil, ErrInvalidCode
	}
	if ac.IsUsed || ac.IsExpired() || !ac.Confirmed() || ac.UserID == nil {
		return nil, ErrInvalidCode
	}

	user, err := s.users.GetByID(ctx, *ac.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if err := s.authCodes.MarkUsed(ctx, ac.ID); err != nil {
		return nil, fmt.Errorf("mark code used: %w", err)
	}
	return user, nil
}

func (s *AuthService) IssueJWT(userID uuid.UUID) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.cfg.JWTTokenTTL)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

func (s *AuthService) ParseJWT(tokenString string) (uuid.UUID, error) {
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return uuid.Nil, fmt.Errorf("invalid token: %w", err)
	}
	return uuid.Parse(claims.Subject)
}

// SetPIN sets the initial PIN. Fails if one is already set (use ResetPIN instead).
func (s *AuthService) SetPIN(ctx context.Context, userID uuid.UUID, pin string) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.HasPIN() {
		return ErrPINAlreadySet
	}
	return s.hashAndStorePIN(ctx, userID, pin)
}

// ResetPIN overwrites an existing PIN, used after a successful reset-code flow.
func (s *AuthService) ResetPIN(ctx context.Context, userID uuid.UUID, pin string) error {
	return s.hashAndStorePIN(ctx, userID, pin)
}

func (s *AuthService) hashAndStorePIN(ctx context.Context, userID uuid.UUID, pin string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash pin: %w", err)
	}
	return s.users.SetPINHash(ctx, userID, string(hash))
}

func (s *AuthService) VerifyPIN(ctx context.Context, userID uuid.UUID, pin string) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if !user.HasPIN() {
		return ErrInvalidPIN
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PINHash), []byte(pin)); err != nil {
		return ErrInvalidPIN
	}
	return nil
}

func randomDigits(n int) (string, error) {
	digits := make([]byte, n)
	for i := range digits {
		num, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		digits[i] = byte('0' + num.Int64())
	}
	return string(digits), nil
}
