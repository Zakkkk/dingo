// Real-world example: Configuration defaults with null coalescing
// The ?? operator provides fallback values for nil pointers and zero values
//
// === Design Decision: Null Coalescing Operator ===
//
// The ?? operator provides fallback values:
//
//	config.timeout ?? 30
//	→ if config.timeout != nil { return *config.timeout }
//	   return 30
//
// Works with pointers, Option types, and zero values.
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

// GetHost returns configured host or default
func GetHost(config *AppConfig) string {
	// If config is nil or Host is nil, return default
	//line /Users/jack/mag/dingo/examples/10_null_coalesce/defaults.dingo:37:9
	if config != nil && config.Host != nil {
		return *config.Host
	}
	return "localhost"
}

// GetPort returns configured port or default
func GetPort(config *AppConfig) int {
	//line /Users/jack/mag/dingo/examples/10_null_coalesce/defaults.dingo:42:9
	if config != nil && config.Port != nil {
		return *config.Port
	}
	return 8080
}

// GetTimeout returns timeout with environment override
func GetTimeout(config *AppConfig) int {
	// Check environment first, then config, then default
	envTimeout := os.Getenv("APP_TIMEOUT")
	if envTimeout != "" {
		// Parse would go here in real code
		return 30
	}
	//line /Users/jack/mag/dingo/examples/10_null_coalesce/defaults.dingo:53:9
	if config != nil && config.Timeout != nil {
		return *config.Timeout
	}
	return 60
}

// GetLogLevel - ?? for pointer, ternary for empty string fallback
func GetLogLevel(config *AppConfig) string {
	//line /Users/jack/mag/dingo/examples/10_null_coalesce/defaults.dingo:58:11
	var level string
	if config != nil && config.LogLevel != nil {
		level = *config.LogLevel
	} else {
		level = ""
	}
	envLevel := os.Getenv("LOG_LEVEL")
	//line /Users/jack/mag/dingo/examples/10_null_coalesce/defaults.dingo:60:9
	if level != "" {
		return level
	}
	if envLevel != "" {
		return envLevel
	}
	return "info"
}

// GetDatabaseURL - ?? for pointer, ternary for empty string fallback
func GetDatabaseURL(env *EnvConfig) string {
	//line /Users/jack/mag/dingo/examples/10_null_coalesce/defaults.dingo:65:9
	var url string
	if env != nil && env.DatabaseURL != nil {
		url = *env.DatabaseURL
	} else {
		url = ""
	}
	envUrl := os.Getenv("DATABASE_URL")
	//line /Users/jack/mag/dingo/examples/10_null_coalesce/defaults.dingo:67:9
	if url != "" {
		return url
	}
	if envUrl != "" {
		return envUrl
	}
	return "postgres://localhost:5432/app"
}

// GetRegion - demonstrates len(safe_nav) with ternary fallback and null-state inference
func GetRegion(env *EnvConfig) string {
	// Use len(env?.Region) > 0 to check if region is set and non-empty
	// Null-state inference: env?.Region in true branch becomes *env.Region (no IIFE)
	//line /Users/jack/mag/dingo/examples/10_null_coalesce/defaults.dingo:74:9
	var tmp int
	if env != nil && env.Region != nil {
		tmp = len(*env.Region)
	}
	if tmp > 0 {
		return *env.Region
	}
	if os.Getenv("AWS_REGION") != "" {
		return os.Getenv("AWS_REGION")
	}
	if os.Getenv("REGION") != "" {
		return os.Getenv("REGION")
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

// Simple pointer-to-value coalescing - ?? automatically dereferences
func GetOptionalSetting(setting *string) string {
	return func() string {
		if setting != nil {
			return *setting
		}
		return "default"
	}()
}

// Chained ?? works when ALL sources are nilable (pointers)
func GetAPIEndpoint(primary *string, secondary *string, tertiary *string) string {
	return func() string {
		if func() *string {
			if func() *string {
				if primary != nil {
					return primary
				}
				return secondary
			}() != nil {
				return func() *string {
					if primary != nil {
						return primary
					}
					return secondary
				}()
			}
			return tertiary
		}() != nil {
			return *func() *string {
				if func() *string {
					if primary != nil {
						return primary
					}
					return secondary
				}() != nil {
					return func() *string {
						if primary != nil {
							return primary
						}
						return secondary
					}()
				}
				return tertiary
			}()
		}
		return "https://api.default.com"
	}()
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
		// Port, Timeout, LogLevel are nil - will use defaults
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
