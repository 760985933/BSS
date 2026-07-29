package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
)

// Config 应用配置，全部支持环境变量覆盖
type Config struct {
	Addr      string // 监听地址，默认 127.0.0.1:8080
	DataDir   string // 数据目录（SQLite + uploads），默认 ./data
	JWTSecret string // JWT HMAC 密钥，生产必须显式配置 BSS_JWT_SECRET
}

func Load() *Config {
	c := &Config{
		Addr:    envOr("BSS_ADDR", "127.0.0.1:8080"),
		DataDir: envOr("BSS_DATA", "./data"),
	}
	c.JWTSecret = os.Getenv("BSS_JWT_SECRET")
	if c.JWTSecret == "" {
		// 未配置时随机生成：重启后所有 token 失效，仅适合开发
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			log.Fatalf("生成随机 JWT 密钥失败: %v", err)
		}
		c.JWTSecret = hex.EncodeToString(b)
		log.Println("[警告] 未配置 BSS_JWT_SECRET，已生成随机密钥（重启后所有会话失效）")
	}
	return c
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
