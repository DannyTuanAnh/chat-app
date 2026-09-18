package config

import (
	"fmt"
	"os"
	"time"

	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/utils"
)

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type ServerConfig struct {
	Port              string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration

	MaxHeaderBytes int
}

type WebsocketServerConfig struct {
	Port              string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration

	MaxHeaderBytes int
}

type RedisOptions struct {
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	MaxRetries      int
	MinRetryBackOff time.Duration
	MaxRetryBackOff time.Duration

	PoolSize     int
	MinIdleConns int
	PoolTimeout  time.Duration
}
type RedisConfig struct {
	Addr     string
	Password string
	DB       int

	Options RedisOptions
}

type ServiceConfig struct {
	AuthServiceAddr       string
	AuthServiceListenAddr string

	UserServiceAddr       string
	UserServiceListenAddr string

	FriendServiceAddr       string
	FriendServiceListenAddr string

	ChatServiceAddr       string
	ChatServiceListenAddr string
}

type RedisGCPConfig struct {
	Addr     string
	Password string
	DB       int

	Options RedisOptions
}

type Config struct {
	DB              DatabaseConfig
	WebsocketServer WebsocketServerConfig
	Server          ServerConfig
	Service         ServiceConfig
	Redis           RedisConfig

	RedisGCP RedisGCPConfig
}

func NewConfig() *Config {
	cfg := &Config{}

	// set server
	serverCfg := NewConfigServer()
	cfg.Server = serverCfg.Server

	// set auth service
	authCfg := NewConfigAuthService()
	cfg.Service.AuthServiceAddr = authCfg.Service.AuthServiceAddr

	// set user service
	userCfg := NewConfigUserService()
	cfg.Service.UserServiceAddr = userCfg.Service.UserServiceAddr

	// set friend service
	friendCfg := NewConfigFriendService()
	cfg.Service.FriendServiceAddr = friendCfg.Service.FriendServiceAddr

	// set chat service
	chatCfg := NewConfigChatService()
	cfg.Service.ChatServiceAddr = chatCfg.Service.ChatServiceAddr

	return cfg
}

func NewConfigWebsocket() *Config {
	cfg := &Config{}

	// set websocket server
	websocketCfg := newConfigWebsocketServer()
	cfg.WebsocketServer = websocketCfg.WebsocketServer

	// set user service
	userCfg := NewConfigUserService()
	cfg.Service.UserServiceAddr = userCfg.Service.UserServiceAddr

	return cfg
}

func newConfigWebsocketServer() *Config {
	port := os.Getenv("PORT_WS")
	if port == "" {
		port = "8081" // default port if not set
	}

	return &Config{
		WebsocketServer: WebsocketServerConfig{
			Port:              port,                                                      // cổng mà server sẽ lắng nghe
			ReadTimeout:       utils.GetEnvTime("SV_READTIMEOUT", 5) * time.Second,       // thời gian tối đa để đọc yêu cầu từ client
			ReadHeaderTimeout: utils.GetEnvTime("SV_READHEADERTIMEOUT", 3) * time.Second, // thời gian tối đa để đọc header của yêu cầu từ client
			WriteTimeout:      utils.GetEnvTime("SV_WRITETIMEOUT", 10) * time.Second,     // thời gian tối đa để gửi phản hồi cho một yêu cầu
			IdleTimeout:       utils.GetEnvTime("SV_IDLETIMEOUT", 120) * time.Second,     // thời gian chờ tối đa cho một kết nối không hoạt động (giữ kết nối tối đa 2 phút)
			ShutdownTimeout:   utils.GetEnvTime("SV_SHUTDOWNTIMEOUT", 5) * time.Second,   // thời gian tối đa để server hoàn thành các yêu cầu đang xử lý trước khi tắt

			MaxHeaderBytes: utils.GetEnvInt("SV_MAXHEADERBYTES", 16) << 10, // giới hạn kích thước header của yêu cầu (16KB)
		},
	}
}

func NewConfigRedisOptions() RedisOptions {
	return RedisOptions{
		DialTimeout:     utils.GetEnvTime("REDIS_DIALTIMEOUT", 5) * time.Second,
		ReadTimeout:     utils.GetEnvTime("REDIS_READTIMEOUT", 3) * time.Second,
		WriteTimeout:    utils.GetEnvTime("REDIS_WRITETIMEOUT", 3) * time.Second,
		MaxRetries:      utils.GetEnvInt("REDIS_MAXRETRIES", 3),
		MinRetryBackOff: utils.GetEnvTime("REDIS_MINRETRYBACKOFF", 50) * time.Millisecond,
		MaxRetryBackOff: utils.GetEnvTime("REDIS_MAXRETRYBACKOFF", 500) * time.Millisecond,
		PoolSize:        utils.GetEnvInt("REDIS_POOLSIZE", 100),
		MinIdleConns:    utils.GetEnvInt("REDIS_MINIDLECONNS", 50),
		PoolTimeout:     utils.GetEnvTime("REDIS_POOLTIMEOUT", 6) * time.Second,
	}
}

func NewConfigGCPRedis() *Config {
	host := utils.GetEnv("REDIS_GCP_HOST", "")
	port := utils.GetEnv("REDIS_GCP_PORT", "")
	addr := fmt.Sprintf("%s:%s", host, port)

	return &Config{
		RedisGCP: RedisGCPConfig{
			Addr:     addr,
			Password: utils.GetEnv("REDIS_GCP_PASSWORD", ""),
			DB:       utils.GetEnvInt("REDIS_GCP_DB", 0),
			Options:  NewConfigRedisOptions(),
		},
	}
}

func NewConfigRedis() *Config {
	return &Config{
		Redis: RedisConfig{
			Addr:     utils.GetEnv("REDIS_ADDR", ""),
			Password: utils.GetEnv("REDIS_PASSWORD", ""),
			DB:       utils.GetEnvInt("REDIS_DB", 0),
			Options:  NewConfigRedisOptions(),
		},
	}
}

func NewConfigServer() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // default port if not set
	}

	return &Config{
		Server: ServerConfig{
			Port:              port,                                                      // cổng mà server sẽ lắng nghe
			ReadTimeout:       utils.GetEnvTime("SV_READTIMEOUT", 5) * time.Second,       // thời gian tối đa để đọc yêu cầu từ client
			ReadHeaderTimeout: utils.GetEnvTime("SV_READHEADERTIMEOUT", 3) * time.Second, // thời gian tối đa để đọc header của yêu cầu từ client
			WriteTimeout:      utils.GetEnvTime("SV_WRITETIMEOUT", 10) * time.Second,     // thời gian tối đa để gửi phản hồi cho một yêu cầu
			IdleTimeout:       utils.GetEnvTime("SV_IDLETIMEOUT", 120) * time.Second,     // thời gian chờ tối đa cho một kết nối không hoạt động (giữ kết nối tối đa 2 phút)
			ShutdownTimeout:   utils.GetEnvTime("SV_SHUTDOWNTIMEOUT", 5) * time.Second,   // thời gian tối đa để server hoàn thành các yêu cầu đang xử lý trước khi tắt

			MaxHeaderBytes: utils.GetEnvInt("SV_MAXHEADERBYTES", 16) << 10, // giới hạn kích thước header của yêu cầu (16KB)
		},
	}
}

func NewConfigAuthService() *Config {
	return &Config{
		Service: ServiceConfig{
			AuthServiceAddr:       utils.GetEnv("AUTH_SERVICE_ADDR", ":50051"),
			AuthServiceListenAddr: utils.GetEnv("AUTH_SERVICE_LISTEN_ADDR", ":50051"),
		},
	}
}

func NewConfigUserService() *Config {
	return &Config{
		Service: ServiceConfig{
			UserServiceAddr:       utils.GetEnv("USER_SERVICE_ADDR", ":50052"),
			UserServiceListenAddr: utils.GetEnv("USER_SERVICE_LISTEN_ADDR", ":50052"),
		},
	}
}

func NewConfigFriendService() *Config {
	return &Config{
		Service: ServiceConfig{
			FriendServiceAddr:       utils.GetEnv("FRIEND_SERVICE_ADDR", ":50053"),
			FriendServiceListenAddr: utils.GetEnv("FRIEND_SERVICE_LISTEN_ADDR", ":50053"),
		},
	}
}

func NewConfigChatService() *Config {
	return &Config{
		Service: ServiceConfig{
			ChatServiceAddr:       utils.GetEnv("CHAT_SERVICE_ADDR", ":50054"),
			ChatServiceListenAddr: utils.GetEnv("CHAT_SERVICE_LISTEN_ADDR", ":50054"),
		},
	}
}

func NewConfigUserDB() *Config {
	return &Config{
		DB: DatabaseConfig{
			Host:     utils.GetEnv("DB_HOST", "localhost"),
			Port:     utils.GetEnv("USER_DB_PORT", "5432"),
			User:     utils.GetEnv("DB_USER", "postgres"),
			Password: utils.GetEnv("USER_DB_PASSWORD", "postgres"),
			DBName:   utils.GetEnv("USER_DB_NAME", "myapp"),
			SSLMode:  utils.GetEnv("DB_SSLMODE", "disable"),
		},
	}
}

func NewConfigAuthDB() *Config {
	return &Config{
		DB: DatabaseConfig{
			Host:     utils.GetEnv("DB_HOST", "localhost"),
			Port:     utils.GetEnv("AUTH_DB_PORT", "5432"),
			User:     utils.GetEnv("DB_USER", "postgres"),
			Password: utils.GetEnv("AUTH_DB_PASSWORD", "postgres"),
			DBName:   utils.GetEnv("AUTH_DB_NAME", "myapp"),
			SSLMode:  utils.GetEnv("DB_SSLMODE", "disable"),
		},
	}
}

func NewConfigFriendDB() *Config {
	return &Config{
		DB: DatabaseConfig{
			Host:     utils.GetEnv("DB_HOST", "localhost"),
			Port:     utils.GetEnv("FRIEND_DB_PORT", "5432"),
			User:     utils.GetEnv("DB_USER", "postgres"),
			Password: utils.GetEnv("FRIEND_DB_PASSWORD", "postgres"),
			DBName:   utils.GetEnv("FRIEND_DB_NAME", "myapp"),
			SSLMode:  utils.GetEnv("DB_SSLMODE", "disable"),
		},
	}
}

func NewConfigChatDB() *Config {
	return &Config{
		DB: DatabaseConfig{
			Host:     utils.GetEnv("DB_HOST", "localhost"),
			Port:     utils.GetEnv("CHAT_DB_PORT", "5432"),
			User:     utils.GetEnv("DB_USER", "postgres"),
			Password: utils.GetEnv("CHAT_DB_PASSWORD", "postgres"),
			DBName:   utils.GetEnv("CHAT_DB_NAME", "myapp"),
			SSLMode:  utils.GetEnv("DB_SSLMODE", "disable"),
		},
	}
}

func (c *Config) DB_DNS() string {
	//local test
	return fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=%s", c.DB.User, c.DB.Password, c.DB.Host, c.DB.Port, c.DB.DBName, c.DB.SSLMode)
}
