package models

import "github.com/apito-io/buffers/shared"

type Config struct {
	// CORS Configuration
	CORSOrigin   string
	CookieDomain string

	// Database Information
	KeyValueEngine          string
	KeyValueDBConfig        *shared.CommonDatabaseConfig
	CacheDBEngine           string
	CacheDBConfig           *shared.CommonDatabaseConfig
	SystemDatabaseEngine    string
	SystemDatabaseDBConfig  *shared.CommonDatabaseConfig
	ProjectDatabaseEngine   string
	ProjectDatabaseDBConfig *shared.CommonDatabaseConfig

	// Default Args while building golang plugins
	PluginBuildArgs string

	// Server Information
	TLS         string
	TLSPort     string
	Environment string
	ServePort   string

	// SSL Credential
	CertPrivateKey string
	CertPath       string

	// Console Login Provider
	AuthServiceProvider string

	// JWT Token TTL
	TokenTTL string
}
