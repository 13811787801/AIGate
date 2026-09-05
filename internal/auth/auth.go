package auth

import (
	"errors"
	"net"
	"net/http"
	"strings"

	"aigate/internal/store"
)

var (
	ErrMissingKey   = errors.New("missing api key")
	ErrInvalidKey   = errors.New("invalid api key")
	ErrKeyDisabled  = errors.New("api key disabled")
	ErrKeyExpired   = errors.New("api key expired")
	ErrUserDisabled = errors.New("user disabled")
	ErrModelDenied  = errors.New("model not allowed for this key")
	ErrIPDenied     = errors.New("ip not allowed")
)

type Principal struct {
	Key      *store.APIKey
	User     *store.User
	Group    *store.UserGroup
	GroupID  int64
	RawKey   string
	ClientIP string
}

type Service struct {
	Store *store.Store
}

func (s *Service) Authenticate(r *http.Request) (*Principal, error) {
	raw := extractAPIKey(r)
	if raw == "" {
		return nil, ErrMissingKey
	}
	hash := store.HashAPIKey(raw)
	key, err := s.Store.GetAPIKeyByHash(hash)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrInvalidKey
		}
		return nil, err
	}
	if key.Status != 1 {
		return nil, ErrKeyDisabled
	}
	if store.IsExpired(key.ExpiredAt) {
		return nil, ErrKeyExpired
	}
	user, err := s.Store.GetUser(key.UserID)
	if err != nil {
		return nil, err
	}
	if user.Status != 1 {
		return nil, ErrUserDisabled
	}
	groupID := user.GroupID
	if key.GroupID.Valid {
		groupID = key.GroupID.Int64
	}
	group, err := s.Store.GetGroup(groupID)
	if err != nil {
		return nil, err
	}
	ip := clientIP(r)
	if len(key.IPWhitelist) > 0 && !ipAllowed(ip, key.IPWhitelist) {
		return nil, ErrIPDenied
	}
	s.Store.TouchAPIKey(key.ID)
	return &Principal{Key: key, User: user, Group: group, GroupID: groupID, RawKey: raw, ClientIP: ip}, nil
}

func (p *Principal) AllowModel(model string) bool {
	if len(p.Key.AllowedModels) == 0 {
		return true
	}
	for _, m := range p.Key.AllowedModels {
		if m == model {
			return true
		}
	}
	return false
}

func extractAPIKey(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	if v := r.Header.Get("X-API-Key"); v != "" {
		return strings.TrimSpace(v)
	}
	return ""
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func ipAllowed(ip string, list []string) bool {
	for _, item := range list {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if item == ip {
			return true
		}
		if _, cidr, err := net.ParseCIDR(item); err == nil {
			if parsed := net.ParseIP(ip); parsed != nil && cidr.Contains(parsed) {
				return true
			}
		}
	}
	return false
}
