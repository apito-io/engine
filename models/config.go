package models

type Config struct {
	Environment string `env:"ENVIRONMENT" env-default:"local"`

	BrankaKey string `env:"BRANKA_KEY" env-default:""`

	CORSOrigin   string `env:"CORS_ORIGIN" env-default:"http://localhost:4000"`
	CookieDomain string `env:"COOKIE_DOMAIN" env-default:"localhost"`

	UptraceDNS string `env:"UPTRACE_DNS" env-default:""`

	PluginPath string `env:"PLUGIN_PATH" env-default:"plugins"`

	CacheEngine     string `env:"CACHE_DB" env-default:"memory"`
	CacheDBHost     string `env:"CACHE_DB_HOST" env-default:""`
	CacheDBPort     string `env:"CACHE_DB_PORT" env-default:""`
	CacheDBUser     string `env:"CACHE_DB_USER" env-default:""`
	CacheDBPassword string `env:"CACHE_DB_PASSWORD" env-default:""`
	CacheDBName     string `env:"CACHE_DB_NAME" env-default:"apito_cache.db"`

	CacheTTL string `env:"CACHE_TTL" env-default:"600"`

	// Server Information
	ServePort string `env:"SERVE_PORT" env-default:"5050"` // Server Listening Port

	DefaultDatabaseDir string `env:"DEFAULT_DATABASE_DIR" env-default:"~/.apito/db"`

	// System Database Information
	SystemDatabaseEngine string `env:"SYSTEM_DB_ENGINE" env-default:"coreDB"`
	SystemDBUser         string `env:"SYSTEM_DB_USER" env-default:""`
	SystemDBPassword     string `env:"SYSTEM_DB_PASSWORD" env-default:""`
	SystemDBHost         string `env:"SYSTEM_DB_HOST" env-default:""`
	SystemDBPort         string `env:"SYSTEM_DB_PORT" env-default:""`
	SystemDBName         string `env:"SYSTEM_DB_NAME" env-default:"apito_system.db"`

	DefaultProjectDatabaseEngine string `env:"PROJECT_DB_ENGINE" env-default:"coreDB"`
	DefaultProjectDBUser         string `env:"PROJECT_DB_USER" env-default:""`
	DefaultProjectDBPassword     string `env:"PROJECT_DB_PASSWORD" env-default:""`
	DefaultProjectDBHost         string `env:"PROJECT_DB_HOST" env-default:""`
	DefaultProjectDBPort         string `env:"PROJECT_DB_PORT" env-default:""`
	DefaultProjectDBName         string `env:"PROJECT_DB_NAME" env-default:"apito_project.db"`

	DefaultSaaSProjectDBName string `env:"DEFAULT_SAAS_PROJECT_DB_NAME" env-default:"apito_saas_project.db"`

	KVStorageEngine         string `env:"KV_ENGINE" env-default:"coreDB"`
	KVStorageEngineHost     string `env:"KV_HOST" env-default:""`
	KVStorageEnginePort     string `env:"KV_PORT" env-default:""`
	KVStorageEngineUser     string `env:"KV_USER" env-default:""`
	KVStorageEnginePassword string `env:"KV_PASSWORD" env-default:""`
	KVStorageEngineDatabase string `env:"KV_DATABASE" env-default:"apito_kv.db"`

	QueueStorageEngine         string `env:"QUEUE_ENGINE" env-default:"coreDB"`
	QueueStorageEngineHost     string `env:"QUEUE_HOST" env-default:""`
	QueueStorageEnginePort     string `env:"QUEUE_PORT" env-default:""`
	QueueStorageEngineUser     string `env:"QUEUE_USER" env-default:""`
	QueueStorageEnginePassword string `env:"QUEUE_PASSWORD" env-default:""`
	QueueStorageEngineDatabase string `env:"QUEUE_DATABASE" env-default:"apito_queue.db"`

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

	// Admin password reset: secret required to call POST /admin/reset-password (e.g. set in ~/.apito/bin/.env)
	AdminResetSecret string `env:"APITO_ADMIN_RESET_SECRET" env-default:""`

	// Optional driver factory for dependency injection (used by pro version)
	// If nil, falls back to default core drivers
	DriverFactory interface{} `env:"-"` // Will be type-asserted to DatabaseDriverFactory
}
