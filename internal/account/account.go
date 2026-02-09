package account

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Mapping đại định cho cấu hình mapping subdomain -> local target
type Mapping struct {
	Subdomain   string `json:"subdomain"`
	LocalTarget string `json:"local_target"` // Ví dụ: http://localhost:3000
}

// Account đại diện cho một tài khoản người dùng/client
type Account struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	Token        string    `json:"token"`       // Token dùng để agent authenticate
	AdminToken   string    `json:"admin_token"` // Token dùng để truy cập User Portal API qua portal
	Subdomains   []string  `json:"subdomains"`  // Các subdomain được phép sử dụng (danh sách trắng)
	Mappings     []Mapping `json:"mappings"`    // Cấu hình mapping từ xa
	MaxConns     int       `json:"max_conns"`   // Số lượng kết nối tối đa
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// SetPassword hash và lưu password
func (a *Account) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	a.PasswordHash = string(hash)
	return nil
}

// CheckPassword kiểm tra password
func (a *Account) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte(password))
	return err == nil
}

// Store interface để quản lý tài khoản
type Store interface {
	Get(id string) (*Account, error)
	GetByUsername(username string) (*Account, error)
	GetByToken(token string) (*Account, error)
	Save(acc *Account) error
	Delete(id string) error
	List() ([]*Account, error)
}

// JSONStore implement Store bằng file JSON
type JSONStore struct {
	filePath string
	accounts map[string]*Account
	mu       sync.RWMutex
}

// NewJSONStore tạo mới JSONStore
func NewJSONStore(filePath string) (*JSONStore, error) {
	store := &JSONStore{
		filePath: filePath,
		accounts: make(map[string]*Account),
	}

	if err := store.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load accounts: %w", err)
	}

	return store, nil
}

func (s *JSONStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.accounts)
}

func (s *JSONStore) save() error {
	data, err := json.MarshalIndent(s.accounts, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

func (s *JSONStore) Get(id string) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	acc, ok := s.accounts[id]
	if !ok {
		return nil, fmt.Errorf("account not found: %s", id)
	}
	return acc, nil
}

func (s *JSONStore) GetByUsername(username string) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, acc := range s.accounts {
		if acc.Username == username {
			return acc, nil
		}
	}
	return nil, fmt.Errorf("account not found for username: %s", username)
}

func (s *JSONStore) GetByToken(token string) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, acc := range s.accounts {
		if acc.Token == token {
			return acc, nil
		}
	}
	return nil, fmt.Errorf("account not found for token")
}

func (s *JSONStore) Save(acc *Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	acc.UpdatedAt = time.Now()
	if acc.CreatedAt.IsZero() {
		acc.CreatedAt = acc.UpdatedAt
	}

	s.accounts[acc.ID] = acc
	return s.save()
}

func (s *JSONStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.accounts, id)
	return s.save()
}

func (s *JSONStore) List() ([]*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*Account, 0, len(s.accounts))
	for _, acc := range s.accounts {
		list = append(list, acc)
	}
	return list, nil
}
