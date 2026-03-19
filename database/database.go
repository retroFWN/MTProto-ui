package database

import (
	"log"
	"time"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var DB *gorm.DB

// ── Models ───────────────────────────────────────────────────────────────

type User struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	Username     string `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash string `gorm:"size:256;not null" json:"-"`
	CreatedAt    int64  `gorm:"autoCreateTime" json:"created_at"`
}

type Proxy struct {
	ID               uint     `gorm:"primaryKey" json:"id"`
	Name             string   `gorm:"size:128;not null" json:"name"`
	Port             int      `gorm:"uniqueIndex;not null" json:"port"`
	FakeTLSDomain    string   `gorm:"size:256;default:google.com" json:"fake_tls_domain"`
	ContainerID      string   `gorm:"size:128" json:"container_id"`
	Enabled          bool     `gorm:"default:true" json:"enabled"`
	TrafficUp        int64    `gorm:"default:0" json:"traffic_up"`
	TrafficDown      int64    `gorm:"default:0" json:"traffic_down"`
	TrafficLimit     int64    `gorm:"default:0" json:"traffic_total_limit"`
	CreatedAt        int64    `gorm:"autoCreateTime" json:"created_at"`
	Clients          []Client `gorm:"foreignKey:ProxyID;constraint:OnDelete:CASCADE" json:"clients,omitempty"`
}

type Client struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	ProxyID      uint   `gorm:"not null;index" json:"proxy_id"`
	Name         string `gorm:"size:128;not null" json:"name"`
	Secret       string `gorm:"size:64;uniqueIndex;not null" json:"secret"`
	Enabled      bool   `gorm:"default:true" json:"enabled"`
	TrafficUp    int64  `gorm:"default:0" json:"traffic_up"`
	TrafficDown  int64  `gorm:"default:0" json:"traffic_down"`
	TrafficLimit int64  `gorm:"default:0" json:"traffic_limit"`
	ExpiryTime   int64  `gorm:"default:0" json:"expiry_time"`
	LastOnline   int64  `gorm:"default:0" json:"last_online"`
	CreatedAt    int64  `gorm:"autoCreateTime" json:"created_at"`
}

type Setting struct {
	ID    uint   `gorm:"primaryKey" json:"id"`
	Key   string `gorm:"uniqueIndex;size:128;not null" json:"key"`
	Value string `gorm:"type:text" json:"value"`
}

// ── Init ─────────────────────────────────────────────────────────────────

func Init(dbPath string) {
	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	DB.AutoMigrate(&User{}, &Proxy{}, &Client{}, &Setting{})
}

func Seed(username, password string) {
	var count int64
	DB.Model(&User{}).Count(&count)
	if count > 0 {
		return
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	DB.Create(&User{
		Username:     username,
		PasswordHash: string(hash),
	})
	log.Printf("Created default admin user (%s/%s)", username, password)
}

// ── Helpers ──────────────────────────────────────────────────────────────

func GetServerIP() string {
	var s Setting
	if DB.Where("`key` = ?", "server_ip").First(&s).Error == nil && s.Value != "" {
		return s.Value
	}
	return "YOUR_SERVER_IP"
}

func GetEnabledSecrets(proxyID uint) []string {
	var clients []Client
	DB.Where("proxy_id = ? AND enabled = ?", proxyID, true).Find(&clients)
	secrets := make([]string, 0, len(clients))
	for _, c := range clients {
		secrets = append(secrets, c.Secret)
	}
	return secrets
}

func DisableExpiredClients() int {
	now := time.Now().Unix()
	result := DB.Model(&Client{}).
		Where("expiry_time > 0 AND expiry_time < ? AND enabled = ?", now, true).
		Update("enabled", false)
	return int(result.RowsAffected)
}
