// Generated Go code from defaults.dingo
// Null coalescing becomes nil checks with fallback
package main

import (
	"fmt"
	"os"
)

type AppConfig struct {
	Host     *string
	Port     *int
	Timeout  *int
	LogLevel *string
	Debug    *bool
}

type EnvConfig struct {
	DatabaseURL *string
	APIKey      *string
	Region      *string
}

// Option type for demonstration
type Option[T any] struct {
	value   T
	present bool
}

func (o Option[T]) IsSome() bool { return o.present }
func (o Option[T]) Unwrap() T    { return o.value }

// GetHost returns configured host or default
func GetHost(config *AppConfig) string {
	var result *string
	if config != nil {
		result = config.Host
	}
	if result != nil {
		return *result
	}
	return "localhost"
}

// GetPort returns configured port or default
func GetPort(config *AppConfig) int {
	var result *int
	if config != nil {
		result = config.Port
	}
	if result != nil {
		return *result
	}
	return 8080
}

// GetTimeout returns timeout with environment override
func GetTimeout(config *AppConfig) int {
	envTimeout := os.Getenv("APP_TIMEOUT")
	if envTimeout != "" {
		return 30
	}
	var result *int
	if config != nil {
		result = config.Timeout
	}
	if result != nil {
		return *result
	}
	return 60
}

// GetLogLevel with chained defaults
func GetLogLevel(config *AppConfig) string {
	var configLevel *string
	if config != nil {
		configLevel = config.LogLevel
	}
	if configLevel != nil {
		return *configLevel
	}
	envLevel := os.Getenv("LOG_LEVEL")
	if envLevel != "" {
		return envLevel
	}
	return "info"
}

// GetDatabaseURL with environment fallback
func GetDatabaseURL(env *EnvConfig) string {
	var configURL *string
	if env != nil {
		configURL = env.DatabaseURL
	}
	if configURL != nil {
		return *configURL
	}
	envURL := os.Getenv("DATABASE_URL")
	if envURL != "" {
		return envURL
	}
	return "postgres://localhost:5432/app"
}

// GetRegion with multi-level fallback
func GetRegion(env *EnvConfig) string {
	var configRegion *string
	if env != nil {
		configRegion = env.Region
	}
	if configRegion != nil {
		return *configRegion
	}
	awsRegion := os.Getenv("AWS_REGION")
	if awsRegion != "" {
		return awsRegion
	}
	region := os.Getenv("REGION")
	if region != "" {
		return region
	}
	return "us-east-1"
}

// LoadConfig demonstrates building config with defaults
func LoadConfig(appConfig *AppConfig, envConfig *EnvConfig) map[string]interface{} {
	return map[string]interface{}{
		"host":        GetHost(appConfig),
		"port":        GetPort(appConfig),
		"timeout":     GetTimeout(appConfig),
		"logLevel":    GetLogLevel(appConfig),
		"databaseURL": GetDatabaseURL(envConfig),
		"region":      GetRegion(envConfig),
	}
}

// GetOptionalSetting example with Option type
func GetOptionalSetting(setting Option[string]) string {
	if setting.IsSome() {
		return setting.Unwrap()
	}
	return "default"
}

// GetAPIEndpoint with chained coalescing
func GetAPIEndpoint(primary *string, secondary *string, tertiary *string) string {
	if primary != nil {
		return *primary
	}
	if secondary != nil {
		return *secondary
	}
	if tertiary != nil {
		return *tertiary
	}
	return "https://api.default.com"
}

func main() {
	// Fully configured
	host := "api.example.com"
	port := 3000
	timeout := 30
	logLevel := "debug"
	fullConfig := &AppConfig{
		Host:     &host,
		Port:     &port,
		Timeout:  &timeout,
		LogLevel: &logLevel,
	}

	// Partially configured
	partialConfig := &AppConfig{
		Host: &host,
	}

	// No configuration (all defaults)
	var emptyConfig *AppConfig = nil

	fmt.Println("=== Full Config ===")
	fmt.Printf("Host: %s\n", GetHost(fullConfig))
	fmt.Printf("Port: %d\n", GetPort(fullConfig))
	fmt.Printf("Timeout: %d\n", GetTimeout(fullConfig))
	fmt.Printf("LogLevel: %s\n", GetLogLevel(fullConfig))

	fmt.Println("\n=== Partial Config ===")
	fmt.Printf("Host: %s\n", GetHost(partialConfig))
	fmt.Printf("Port: %d (default)\n", GetPort(partialConfig))
	fmt.Printf("Timeout: %d (default)\n", GetTimeout(partialConfig))
	fmt.Printf("LogLevel: %s (default)\n", GetLogLevel(partialConfig))

	fmt.Println("\n=== Empty Config ===")
	fmt.Printf("Host: %s (default)\n", GetHost(emptyConfig))
	fmt.Printf("Port: %d (default)\n", GetPort(emptyConfig))

	// Chained example
	fmt.Println("\n=== Chained Fallback ===")
	primary := "https://primary.api.com"
	endpoint1 := GetAPIEndpoint(&primary, nil, nil)
	endpoint2 := GetAPIEndpoint(nil, nil, nil)
	fmt.Printf("With primary: %s\n", endpoint1)
	fmt.Printf("All nil: %s\n", endpoint2)
}
