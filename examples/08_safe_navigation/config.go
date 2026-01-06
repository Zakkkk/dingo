// Real-world example: Safely navigating nested configuration
// Safe navigation (?.) prevents nil pointer panics in deep object access
//
// === Design Decision: Safe Navigation Operator ===
//
// The ?. operator short-circuits nil pointer chains:
//
//	config?.ssl?.certPath
//	→ if config == nil || config.ssl == nil { return "" }
//	   return config.ssl.certPath
//
// This prevents runtime panics from nil dereferences.
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
// Without safe nav: if config.Database != nil { return config.Database.Host }
func GetDatabaseHost(config *ServerConfig) string {
	//line examples/08_safe_navigation//config.dingo:63:9
	if config != nil && config.Database != nil {
		return config.Database.Host
	}
	return "localhost"
}

// GetSSLCertPath navigates 3 levels deep safely
func GetSSLCertPath(config *ServerConfig) string {
	//line examples/08_safe_navigation//config.dingo:68:9
	if config != nil && config.Database != nil && config.Database.SSL != nil {
		return config.Database.SSL.CertPath
	}
	return "/etc/ssl/cert.pem"
}

// GetCAPath handles optional *string at the end
func GetCAPath(config *ServerConfig) string {
	// Chain ends with *string, need to dereference if present
	//line examples/08_safe_navigation//config.dingo:74:10
	var path *string
	if config != nil && config.Database != nil && config.Database.SSL != nil {
		path = config.Database.SSL.CAPath
	}
	if path != nil {
		return *path
	}
	return "/etc/ssl/ca.pem"
}

// GetRedisPassword safely accesses deeply nested optional password
func GetRedisPassword(config *ServerConfig) string {
	//line examples/08_safe_navigation//config.dingo:83:14
	var password *string
	if config != nil && config.Cache != nil && config.Cache.Redis != nil {
		password = config.Cache.Redis.Password
	}
	if password != nil {
		return *password
	}
	return ""
}

// GetLogFile combines safe navigation with null coalescing
func GetLogFile(config *ServerConfig) string {
	//line examples/08_safe_navigation//config.dingo:92:10
	var file *string
	if config != nil && config.Logging != nil && config.Logging.Output != nil {
		file = config.Logging.Output.File
	}
	if file != nil {
		return *file
	}
	return "/var/log/app.log"
}

// IsSSLEnabled shows safe navigation with method-like checks
func IsSSLEnabled(config *ServerConfig) bool {
	// If any part is nil, returns false (zero value)
	//line examples/08_safe_navigation//config.dingo:102:9
	if config != nil && config.Database != nil && config.Database.SSL != nil {
		return config.Database.SSL.Enabled
	}
	return false
}

// GetReplicaCount safely accesses array length
func GetReplicaCount(config *ServerConfig) int {
	//line examples/08_safe_navigation//config.dingo:107:14
	var replicas []*DatabaseConfig
	if config != nil && config.Database != nil {
		replicas = config.Database.Replicas
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
		// No database, cache, or logging configured
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
	fmt.Printf("DB Host: %s\n", GetDatabaseHost(minimal))  // "localhost"
	fmt.Printf("SSL Cert: %s\n", GetSSLCertPath(minimal))  // default
	fmt.Printf("SSL Enabled: %v\n", IsSSLEnabled(minimal)) // false

	fmt.Println("\n=== Nil Config ===")
	fmt.Printf("DB Host: %s\n", GetDatabaseHost(empty)) // "localhost"
	fmt.Printf("SSL Cert: %s\n", GetSSLCertPath(empty)) // default
}
