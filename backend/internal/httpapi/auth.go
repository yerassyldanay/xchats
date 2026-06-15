package httpapi

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"golang.org/x/crypto/argon2"
)

const sessionCookie = "xchats_session"

type ctxUserKey struct{}

// --- argon2id password hashing (explicitly not sha256) --------------------

type argonParams struct {
	memory, time uint32
	threads      uint8
	keyLen       uint32
}

var defaultArgon = argonParams{memory: 64 * 1024, time: 1, threads: 4, keyLen: 32}

// HashPassword returns an encoded argon2id hash string.
func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	p := defaultArgon
	key := argon2.IDKey([]byte(password), salt, p.time, p.memory, p.threads, p.keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.memory, p.time, p.threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword checks a password against an encoded argon2id hash.
func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var p argonParams
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.time, &p.threads); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	p.keyLen = uint32(len(want))
	got := argon2.IDKey([]byte(password), salt, p.time, p.memory, p.threads, p.keyLen)
	return subtle.ConstantTimeCompare(got, want) == 1
}

// --- session middleware ---------------------------------------------------

func (s *Server) requireSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, err := c.Cookie(sessionCookie)
		if err != nil || sid == "" {
			fail(c, http.StatusUnauthorized, ErrUnauthorized, "no session")
			return
		}
		u, err := s.store.UserForSession(ctx(c), sid)
		if err != nil {
			fail(c, http.StatusUnauthorized, ErrUnauthorized, "invalid session")
			return
		}
		c.Set("user", u)
		c.Next()
	}
}

func currentUser(c *gin.Context) store.User {
	v, _ := c.Get("user")
	u, _ := v.(store.User)
	return u
}

// --- handlers -------------------------------------------------------------

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Email == "" || req.Password == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "email and password required")
		return
	}
	u, err := s.store.UserByEmail(ctx(c), strings.TrimSpace(req.Email))
	if err != nil || !VerifyPassword(req.Password, u.PasswordHash) {
		fail(c, http.StatusUnauthorized, ErrUnauthorized, "invalid credentials")
		return
	}
	sid := newSessionID()
	ttl := time.Duration(s.cfg.SessionTTLHours) * time.Hour
	if err := s.store.CreateSession(ctx(c), sid, u.ID, ttl); err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, "session")
		return
	}
	s.setSessionCookie(c, sid, int(ttl.Seconds()))
	ok(c, s.mePayload(c, u))
}

func (s *Server) handleLogout(c *gin.Context) {
	if sid, err := c.Cookie(sessionCookie); err == nil && sid != "" {
		_ = s.store.DeleteSession(ctx(c), sid)
	}
	s.setSessionCookie(c, "", -1)
	ok(c, nil)
}

func (s *Server) handleMe(c *gin.Context) {
	ok(c, s.mePayload(c, currentUser(c)))
}

func (s *Server) mePayload(c *gin.Context, u store.User) gin.H {
	org, _ := s.store.OrgForUser(ctx(c), u.ID)
	return gin.H{
		"user": gin.H{"id": u.ID, "email": u.Email, "name": u.DisplayName},
		"organization": gin.H{"id": org.ID, "name": org.Name},
	}
}

func (s *Server) handleGetOrg(c *gin.Context) {
	u := currentUser(c)
	org, err := s.store.OrgForUser(ctx(c), u.ID)
	if err != nil {
		fail(c, http.StatusNotFound, ErrNotFound, "no organization")
		return
	}
	ok(c, gin.H{"id": org.ID, "name": org.Name, "auto_response_mode": org.RespondMode})
}

func (s *Server) handleListUsers(c *gin.Context) {
	limit, offset, pageNum, pageSize := s.pageParams(c)
	users, total, err := s.store.ListUsers(ctx(c), limit, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	items := make([]gin.H, 0, len(users))
	for _, u := range users {
		items = append(items, gin.H{"id": u.ID, "email": u.Email, "name": u.DisplayName, "created_at": u.CreatedAt})
	}
	ok(c, page{Items: items, Page: pageNum, PageSize: pageSize, Total: total})
}

type createUserReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (s *Server) handleCreateUser(c *gin.Context) {
	var req createUserReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Email == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "email required")
		return
	}
	if len(req.Password) < s.cfg.MinPasswordLen {
		fail(c, http.StatusBadRequest, ErrValidation, fmt.Sprintf("password must be >= %d chars", s.cfg.MinPasswordLen))
		return
	}
	org, err := s.store.OrgForUser(ctx(c), currentUser(c).ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, "no org")
		return
	}
	hash, err := HashPassword(req.Password)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, "hash")
		return
	}
	u, err := s.store.CreateUser(ctx(c), org.ID, strings.TrimSpace(req.Email), hash, req.Name)
	if err != nil {
		if isUniqueViolation(err) {
			fail(c, http.StatusConflict, ErrConflict, "email already exists")
			return
		}
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	created(c, gin.H{"id": u.ID, "email": u.Email, "name": u.DisplayName})
}

func (s *Server) setSessionCookie(c *gin.Context, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   s.cfg.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

func newSessionID() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}
