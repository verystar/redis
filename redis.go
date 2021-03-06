package redis

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/go-redis/redis/v7"
)

var (
	redisList map[string]*redis.Client
	errs      []string
)

type Config struct {
	Server       string
	Password     string
	DB           int
	MaxRetries   int
	DialTimeout  int `json:"dial_timeout" toml:"dial_timeout"`
	ReadTimeout  int `json:"read_timeout" toml:"read_timeout"`
	WriteTimeout int `json:"write_timeout" toml:"write_timeout"`

	// sentinel
	MasterName       string `json:"master_name" toml:"master_name"`
	SentinelAddrs    string `json:"sentinel_addrs" toml:"sentinel_addrs"`
	SentinelUsername string `json:"sentinel_username" toml:"sentinel_username"`
	SentinelPassword string `json:"sentinel_password" toml:"sentinel_password"`
	CaCert           string `json:"ca_cert" toml:"ca_cert"`
	CertFile         string `json:"cert_file" toml:"cert_file"`
	CertKey          string `json:"cert_key" toml:"cert_key"`
}

func Client(name ...string) *redis.Client {
	key := "default"
	if name != nil {
		key = name[0]
	}

	client, ok := redisList[key]
	if !ok {
		panic(fmt.Sprintf("[redis] the redis client `%s` is not configured", key))
	}

	return client
}

// Open redis client
func Open(addr string, options ...func(options *redis.Options)) *redis.Client {

	redisOption := &redis.Options{
		Addr: addr,
	}

	for _, option := range options {
		option(redisOption)
	}

	return redis.NewClient(redisOption)
}

// Open redis client
func OpenSentinel(options func(options *redis.FailoverOptions)) *redis.Client {
	redisOption := &redis.FailoverOptions{}
	options(redisOption)
	return redis.NewFailoverClient(redisOption)
}

func Connect(configs map[string]Config) {
	defer func() {
		if len(errs) > 0 {
			panic("[redis] " + strings.Join(errs, "\n"))
		}
	}()

	redisList = make(map[string]*redis.Client)
	for name, conf := range configs {
		r := newRedis(&conf)
		log.Println("[redis] connect:" + conf.Server)

		_, err := r.Ping().Result()
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}

		client := newRedis(&conf)

		if r, ok := redisList[name]; ok {
			redisList[name] = client
			_ = r.Close()
		} else {
			redisList[name] = client
		}
	}
}

// 创建 redis for config
func newRedis(conf *Config) *redis.Client {

	if conf.MasterName != "" {
		return OpenSentinel(func(options *redis.FailoverOptions) {
			options.MasterName = conf.MasterName
			options.SentinelAddrs = strings.Split(conf.SentinelAddrs, ",")
			options.SentinelPassword = conf.SentinelPassword
			options.SentinelUsername = conf.SentinelUsername
			options.Password = conf.Password
			options.DB = conf.DB

			if conf.MaxRetries > 0 {
				options.MaxRetries = conf.MaxRetries
			}

			if conf.DialTimeout > 0 {
				options.DialTimeout = time.Duration(conf.DialTimeout) * time.Second
			}

			if conf.ReadTimeout > 0 {
				options.ReadTimeout = time.Duration(conf.ReadTimeout) * time.Second
			}

			if conf.WriteTimeout > 0 {
				options.WriteTimeout = time.Duration(conf.WriteTimeout) * time.Second
			}

			// 开启TLS连接模式
			if len(conf.CertKey) > 0 && len(conf.CertFile) > 0 && len(conf.CaCert) > 0 {
				cert, err := tls.X509KeyPair([]byte(conf.CertFile), []byte(conf.CertKey))
				if err != nil {
					panic(fmt.Sprintf("Unable to load key pair: %s", err))
				}

				pool := x509.NewCertPool()
				ok := pool.AppendCertsFromPEM([]byte(conf.CaCert))
				if !ok {
					panic("failed to parse root certificate")
				}

				options.TLSConfig = &tls.Config{
					ClientAuth:   tls.RequireAndVerifyClientCert,
					Certificates: []tls.Certificate{cert},
					MinVersion:   tls.VersionTLS12,
					RootCAs:      pool,
				}
			}

		})
	}

	return Open(conf.Server, func(options *redis.Options) {
		options.Password = conf.Password
		options.DB = conf.DB

		if conf.MaxRetries > 0 {
			options.MaxRetries = conf.MaxRetries
		}

		if conf.DialTimeout > 0 {
			options.DialTimeout = time.Duration(conf.DialTimeout) * time.Second
		}

		if conf.ReadTimeout > 0 {
			options.ReadTimeout = time.Duration(conf.ReadTimeout) * time.Second
		}

		if conf.WriteTimeout > 0 {
			options.WriteTimeout = time.Duration(conf.WriteTimeout) * time.Second
		}
	})
}
