package models

type Config struct {
	Environment string `env:"ENVIRONMENT" env-default:"local"`

	BrankaKey string `env:"BRANKA_KEY" env-default:""`

	CORSOrigin   string `env:"CORS_ORIGIN" env-default:""`
	CookieDomain string `env:"COOKIE_DOMAIN" env-default:""`

	UptraceDNS string `env:"UPTRACE_DNS" env-default:""`

	PluginPath string `env:"PLUGIN_PATH" env-default:"plugins"`

	CacheDriver string `env:"CACHE_DRIVER" env-default:"memory"`
	CacheTTL    string `env:"CACHE_TTL" env-default:"600"`

	// Server Information
	ServePort string `env:"SERVE_PORT" env-default:"5050"` // Server Listening Port

	SystemDatabaseEngine string `env:"APITO_SYSTEM_DB_ENGINE" env-default:"embedded"`

	// System Database Information
	SystemDBUser     string `env:"SYSTEM_DB_USER" env-default:""`
	SystemDBPassword string `env:"SYSTEM_DB_PASSWORD" env-default:""`
	SystemDBHost     string `env:"SYSTEM_DB_HOST" env-default:""`
	SystemDBPort     string `env:"SYSTEM_DB_PORT" env-default:""`
	SystemDBName     string `env:"SYSTEM_DB_NAME" env-default:""`

	DefaultProjectDatabaseEngine string `env:"APITO_PROJECT_DB_ENGINE" env-default:"embedded"`

	DefaultProjectDBUser     string `env:"PROJECT_DB_USER" env-default:""`
	DefaultProjectDBPassword string `env:"PROJECT_DB_PASSWORD" env-default:""`
	DefaultProjectDBHost     string `env:"PROJECT_DB_HOST" env-default:""`
	DefaultProjectDBPort     string `env:"PROJECT_DB_PORT" env-default:""`
	DefaultProjectDBName     string `env:"PROJECT_DB_NAME" env-default:""`

	DefaultSaaSProjectDBName string `env:"DEFAULT_SAAS_PROJECT_DB_NAME" env-default:""`

	KVStorageEngine         string `env:"KV_ENGINE" env-default:"embedded"`
	KVStorageEngineHost     string `env:"KV_HOST" env-default:""`
	KVStorageEnginePort     string `env:"KV_PORT" env-default:""`
	KVStorageEngineUser     string `env:"KV_USER" env-default:""`
	KVStorageEnginePassword string `env:"KV_PASSWORD" env-default:""`
	KVStorageEngineDatabase string `env:"KV_DATABASE" env-default:"0"`

	// Token Encryption Credential
	PublicKeyPath  string `env:"PUBLIC_KEY_PATH" env-default:"keys/public.key"`
	PrivateKeyPath string `env:"PRIVATE_KEY_PATH" env-default:"keys/private.key"`

	// Sentry Credential
	SentryKey     string `env:"SENTRY_KEY" env-default:""`
	SentryProject string `env:"SENTRY_PROJECT" env-default:""`
	SentryAPI     string `env:"SENTRY_API" env-default:""`

	AuthServiceProvider string `env:"AUTH_SERVICE_PROVIDER" env-default:"local"`

	GoogleOauthClientID     string `env:"GOOGLE_OAUTH_CLIENT_ID" env-default:""`
	GoogleOauthClientSecret string `env:"GOOGLE_OAUTH_CLIENT_SECRET" env-default:""`
	GoogleOauthRedirectURL  string `env:"GOOGLE_OAUTH_REDIRECT_URL" env-default:""`

	GithubOauthClientID     string `env:"GITHUB_OAUTH_CLIENT_ID" env-default:""`
	GithubOauthClientSecret string `env:"GITHUB_OAUTH_CLIENT_SECRET" env-default:""`
	GithubOauthRedirectURL  string `env:"GITHUB_OAUTH_REDIRECT_URL" env-default:""`

	TokenTTL string `env:"TOKEN_TTL" env-default:"60"`

	AWSRegion string `env:"AWS_REGION" env-default:""`
	AWSSecret string `env:"AWS_SECRET" env-default:""`
	AWSKey    string `env:"AWS_KEY" env-default:""`

	// Optional driver factory for dependency injection (used by pro version)
	// If nil, falls back to default core drivers
	DriverFactory interface{} `env:"-"` // Will be type-asserted to DatabaseDriverFactory
}
