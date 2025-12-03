// Generated Go code from config.dingo
// Safe navigation becomes explicit nil checks
package main

import "fmt"

type DatabaseConfig struct {
	Host     string
	Port     int
	SSL      *SSLConfig
	Replicas []*DatabaseConfig
}

type SSLConfig struct {
	Enabled  bool
	CertPath string
	KeyPath  string
	CAPath   *string
}

type ServerConfig struct {
	Name     string
	Database *DatabaseConfig
	Cache    *CacheConfig
	Logging  *LoggingConfig
}

type CacheConfig struct {
	Enabled bool
	TTL     int
	Redis   *RedisConfig
}

type RedisConfig struct {
	Host     string
	Port     int
	Password *string
}

type LoggingConfig struct {
	Level  string
	Output *OutputConfig
}

type OutputConfig struct {
	File   *string
	Stdout bool
}

// GetDatabaseHost safely accesses nested database host
func GetDatabaseHost(config *ServerConfig) string {
	var result string
	if config != nil {
		if config.Database != nil {
			result = config.Database.Host
		}
	}
	if result == "" {
		return "localhost"
	}
	return result
}

// GetSSLCertPath navigates 3 levels deep safely
func GetSSLCertPath(config *ServerConfig) string {
	var result string
	if config != nil {
		if config.Database != nil {
			if config.Database.SSL != nil {
				result = config.Database.SSL.CertPath
			}
		}
	}
	if result == "" {
		return "/etc/ssl/cert.pem"
	}
	return result
}

// GetCAPath handles optional *string at the end
func GetCAPath(config *ServerConfig) string {
	var path *string
	if config != nil {
		if config.Database != nil {
			if config.Database.SSL != nil {
				path = config.Database.SSL.CAPath
			}
		}
	}
	if path != nil {
		return *path
	}
	return "/etc/ssl/ca.pem"
}

// GetRedisPassword safely accesses deeply nested optional password
func GetRedisPassword(config *ServerConfig) string {
	var password *string
	if config != nil {
		if config.Cache != nil {
			if config.Cache.Redis != nil {
				password = config.Cache.Redis.Password
			}
		}
	}
	if password != nil {
		return *password
	}
	return ""
}

// GetLogFile combines safe navigation with null coalescing
func GetLogFile(config *ServerConfig) string {
	var file *string
	if config != nil {
		if config.Logging != nil {
			if config.Logging.Output != nil {
				file = config.Logging.Output.File
			}
		}
	}
	if file != nil {
		return *file
	}
	return "/var/log/app.log"
}

// IsSSLEnabled shows safe navigation with method-like checks
func IsSSLEnabled(config *ServerConfig) bool {
	var result bool
	if config != nil {
		if config.Database != nil {
			if config.Database.SSL != nil {
				result = config.Database.SSL.Enabled
			}
		}
	}
	return result
}

// GetReplicaCount safely accesses array length
func GetReplicaCount(config *ServerConfig) int {
	var replicas []*DatabaseConfig
	if config != nil {
		if config.Database != nil {
			replicas = config.Database.Replicas
		}
	}
	if replicas == nil {
		return 0
	}
	return len(replicas)
}

func main() {
	// Fully configured server
	caPath := "/etc/ssl/custom-ca.pem"
	full := &ServerConfig{
		Name: "production",
		Database: &DatabaseConfig{
			Host: "db.example.com",
			Port: 5432,
			SSL: &SSLConfig{
				Enabled:  true,
				CertPath: "/etc/ssl/server.crt",
				KeyPath:  "/etc/ssl/server.key",
				CAPath:   &caPath,
			},
		},
		Cache: &CacheConfig{
			Enabled: true,
			TTL:     3600,
			Redis: &RedisConfig{
				Host: "redis.example.com",
				Port: 6379,
			},
		},
	}

	// Minimal configuration
	minimal := &ServerConfig{
		Name: "development",
	}

	// Nil configuration
	var empty *ServerConfig = nil

	fmt.Println("=== Full Config ===")
	fmt.Printf("DB Host: %s\n", GetDatabaseHost(full))
	fmt.Printf("SSL Cert: %s\n", GetSSLCertPath(full))
	fmt.Printf("CA Path: %s\n", GetCAPath(full))
	fmt.Printf("SSL Enabled: %v\n", IsSSLEnabled(full))
	fmt.Printf("Replicas: %d\n", GetReplicaCount(full))

	fmt.Println("\n=== Minimal Config ===")
	fmt.Printf("DB Host: %s\n", GetDatabaseHost(minimal))
	fmt.Printf("SSL Cert: %s\n", GetSSLCertPath(minimal))
	fmt.Printf("SSL Enabled: %v\n", IsSSLEnabled(minimal))

	fmt.Println("\n=== Nil Config ===")
	fmt.Printf("DB Host: %s\n", GetDatabaseHost(empty))
	fmt.Printf("SSL Cert: %s\n", GetSSLCertPath(empty))
}
