package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"nowdone/internal/config"
	"nowdone/internal/models"
	"nowdone/internal/repository"
	"nowdone/internal/service"
)

// userJSON is the public shape of a user returned by the auth endpoints.
func userJSON(u *models.User) gin.H {
	return gin.H{
		"id":             u.ID,
		"first_name":     u.FirstName,
		"has_pin":        u.HasPIN(),
		"current_streak": u.CurrentStreak,
		"max_streak":     u.MaxStreak,
		"timezone":       u.Timezone,
	}
}

// AuthHandler exposes the Telegram-deep-link login flow and PIN management.
type AuthHandler struct {
	auth  *service.AuthService
	users *repository.UserRepository
	cfg   *config.Config
	log   *slog.Logger
}

func NewAuthHandler(auth *service.AuthService, users *repository.UserRepository, cfg *config.Config, log *slog.Logger) *AuthHandler {
	return &AuthHandler{auth: auth, users: users, cfg: cfg, log: log}
}

// POST /auth/login/start — generates a one-time code + bot deep link.
func (h *AuthHandler) StartLogin(c *gin.Context) {
	_, deepLink, err := h.auth.GenerateLoginCode(c.Request.Context())
	if err != nil {
		h.log.Error("generate login code", "error", err)
		respondError(c, http.StatusInternalServerError, "не удалось создать код входа, попробуйте ещё раз")
		return
	}
	c.JSON(http.StatusOK, gin.H{"deep_link": deepLink})
}

type redeemCodeRequest struct {
	Code string `json:"code" binding:"required,len=6,numeric"`
}

// POST /auth/login/redeem — exchanges a confirmed code for a session cookie.
func (h *AuthHandler) RedeemLogin(c *gin.Context) {
	var req redeemCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "неверный формат кода")
		return
	}

	token, user, err := h.auth.RedeemCode(c.Request.Context(), req.Code)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrCodeNotReady):
			respondError(c, http.StatusAccepted, "код ещё не подтверждён в Telegram")
		case errors.Is(err, service.ErrInvalidCode):
			respondError(c, http.StatusBadRequest, "код неверен или истёк")
		default:
			h.log.Error("redeem login code", "error", err)
			respondError(c, http.StatusInternalServerError, "не удалось выполнить вход")
		}
		return
	}

	setSessionCookie(c, h.cfg, token)
	c.JSON(http.StatusOK, gin.H{"user": userJSON(user)})
}

// POST /auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	c.SetCookie("session", "", -1, "/", h.cfg.CookieDomain, true, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type setPINRequest struct {
	PIN string `json:"pin" binding:"required,len=4,numeric"`
}

// POST /auth/pin — sets the initial PIN for the logged-in user.
func (h *AuthHandler) SetPIN(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "не авторизован")
		return
	}

	var req setPINRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "PIN должен состоять из 4 цифр")
		return
	}

	if err := h.auth.SetPIN(c.Request.Context(), userID, req.PIN); err != nil {
		if errors.Is(err, service.ErrPINAlreadySet) {
			respondError(c, http.StatusConflict, "PIN уже установлен")
			return
		}
		h.log.Error("set pin", "error", err)
		respondError(c, http.StatusInternalServerError, "не удалось установить PIN")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// POST /auth/pin/verify — verifies the PIN, unlocking the session for hidden notes / app access.
func (h *AuthHandler) VerifyPIN(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "не авторизован")
		return
	}

	var req setPINRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "PIN должен состоять из 4 цифр")
		return
	}

	if err := h.auth.VerifyPIN(c.Request.Context(), userID, req.PIN); err != nil {
		respondError(c, http.StatusUnauthorized, "неверный PIN")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// POST /auth/pin/reset/start — generates a 6-digit reset code and sends it
// straight to the current user's Telegram chat. No bot deep link is returned;
// the client just opens the code-entry modal.
func (h *AuthHandler) StartPINReset(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "не авторизован")
		return
	}

	if _, err := h.auth.GenerateResetCode(c.Request.Context(), userID); err != nil {
		h.log.Error("generate reset code", "error", err)
		respondError(c, http.StatusInternalServerError, "не удалось отправить код сброса в Telegram")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// POST /auth/pin/reset/redeem — validates the reset code, then the client can call SetNewPIN.
func (h *AuthHandler) RedeemPINReset(c *gin.Context) {
	var req redeemCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "неверный формат кода")
		return
	}

	user, err := h.auth.RedeemResetCode(c.Request.Context(), req.Code)
	if err != nil {
		respondError(c, http.StatusBadRequest, "код неверен или истёк")
		return
	}

	token, err := h.auth.IssueJWT(user.ID)
	if err != nil {
		h.log.Error("issue jwt after reset", "error", err)
		respondError(c, http.StatusInternalServerError, "не удалось выполнить вход")
		return
	}
	setSessionCookie(c, h.cfg, token)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// POST /auth/pin/reset/confirm — sets the new PIN after a redeemed reset code / verified session.
func (h *AuthHandler) SetNewPIN(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "не авторизован")
		return
	}

	var req setPINRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "PIN должен состоять из 4 цифр")
		return
	}

	if err := h.auth.ResetPIN(c.Request.Context(), userID, req.PIN); err != nil {
		h.log.Error("reset pin", "error", err)
		respondError(c, http.StatusInternalServerError, "не удалось установить новый PIN")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GET /auth/me — returns the current user, used on app boot to restore session.
func (h *AuthHandler) Me(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "не авторизован")
		return
	}

	user, err := h.users.GetByID(c.Request.Context(), userID)
	if err != nil {
		respondError(c, http.StatusUnauthorized, "не авторизован")
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": userJSON(user)})
}

type updateSettingsRequest struct {
	Timezone string `json:"timezone" binding:"required"`
}

// PATCH /auth/me — updates user preferences. Currently just the timezone, which
// must be a valid IANA name (e.g. "Europe/Moscow") known to the runtime.
func (h *AuthHandler) UpdateSettings(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "не авторизован")
		return
	}

	var req updateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "укажите часовой пояс")
		return
	}
	if _, err := time.LoadLocation(req.Timezone); err != nil {
		respondError(c, http.StatusBadRequest, "неизвестный часовой пояс")
		return
	}

	if err := h.users.UpdateTimezone(c.Request.Context(), userID, req.Timezone); err != nil {
		h.log.Error("update timezone", "error", err)
		respondError(c, http.StatusInternalServerError, "не удалось сохранить часовой пояс")
		return
	}

	user, err := h.users.GetByID(c.Request.Context(), userID)
	if err != nil {
		h.log.Error("reload user after settings update", "error", err)
		respondError(c, http.StatusInternalServerError, "настройки сохранены, но не удалось их прочитать")
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": userJSON(user)})
}

func setSessionCookie(c *gin.Context, cfg *config.Config, token string) {
	maxAge := int(cfg.JWTTokenTTL / time.Second)
	c.SetCookie("session", token, maxAge, "/", cfg.CookieDomain, true, true)
}
