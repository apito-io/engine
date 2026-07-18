package models

import (
	"context"

	"github.com/apito-io/types"
	"github.com/labstack/echo/v4"
)

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
	// SQLiteDataDir is the persistent root for local SQLite replicas (sqlite/ subtree).
	// Default ./db is relative to the engine working directory; production mounts a volume at this path.
	SQLiteDataDir string `env:"SQLITE_DATA_DIR" env-default:"./db"`
	// LibsqlSyncDataDir is deprecated; use SQLITE_DATA_DIR. Kept for one release of backward compatibility.
	LibsqlSyncDataDir string `env:"LIBSQL_SYNC_DATA_DIR" env-default:""`

	// System Database Information
	SystemDatabaseEngine string `env:"SYSTEM_DB_ENGINE" env-default:"coredb"`
	SystemDBUser         string `env:"SYSTEM_DB_USER" env-default:""`
	SystemDBPassword     string `env:"SYSTEM_DB_PASSWORD" env-default:""`
	SystemDBHost         string `env:"SYSTEM_DB_HOST" env-default:""`
	SystemDBPort         string `env:"SYSTEM_DB_PORT" env-default:""`
	SystemDBName         string `env:"SYSTEM_DB_NAME" env-default:"apito_system.db"`

	// GeneralPostgresIsolation: "database" (default, CREATE DATABASE per project) or "schema" (CREATE SCHEMA + search_path on shared database name stored on the project driver).
	GeneralPostgresIsolation string `env:"GENERAL_POSTGRES_ISOLATION" env-default:"database"`
	// GeneralSQLiteFilePerProject uses utility.SQLiteProjectFileName(project_id) under DefaultDatabaseDir for new SQLite general projects using default template credentials.
	GeneralSQLiteFilePerProject bool `env:"GENERAL_SQLITE_FILE_PER_PROJECT" env-default:"false"`
	// GeneralMySQLIsolation is only "database" supported: MySQL/MariaDB use CREATE DATABASE per project (no PG-style shared-schema mode in this engine).
	GeneralMySQLIsolation string `env:"GENERAL_MYSQL_ISOLATION" env-default:"database"`

	KVStorageEngine         string `env:"KV_ENGINE" env-default:"coredb"`
	KVStorageEngineHost     string `env:"KV_HOST" env-default:""`
	KVStorageEnginePort     string `env:"KV_PORT" env-default:""`
	KVStorageEngineUser     string `env:"KV_USER" env-default:""`
	KVStorageEnginePassword string `env:"KV_PASSWORD" env-default:""`
	KVStorageEngineDatabase string `env:"KV_DATABASE" env-default:"apito_kv.db"`

	// Realtime bus: unified NATS JetStream fan-out (subscriptions + console notify).
	// "nats" = embedded or external NATS with JetStream (production default).
	// "memory" = in-process fan-out (single node, tests/local).
	RealtimeEngine string `env:"REALTIME_ENGINE" env-default:"nats"`
	// RealtimeNatsURL, when set, connects to an external/clustered NATS instead of
	// embedding an in-process server (e.g. "nats://nats:4222").
	RealtimeNatsURL string `env:"REALTIME_NATS_URL" env-default:""`
	// RealtimeNatsPort exposes the embedded NATS server on a TCP port for
	// clustering/leaf-node connections. -1 (default) keeps it in-process only.
	RealtimeNatsPort int `env:"REALTIME_NATS_PORT" env-default:"-1"`
	// RealtimeNatsJetStream enables JetStream durable streams for replay and
	// cross-instance fan-out (default on for production NATS backend).
	RealtimeNatsJetStream bool `env:"REALTIME_NATS_JETSTREAM" env-default:"true"`
	// RealtimeNatsStoreDir is the JetStream file store directory for embedded NATS.
	RealtimeNatsStoreDir string `env:"REALTIME_NATS_STORE_DIR" env-default:""`

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

	// Platform free-cloud object storage (R2/S3-compatible). Used when project storage_settings.use_free_cloud_storage=true.
	FreeCloudDefaultS3AccessKey      string  `env:"FREE_CLOUD_DEFAULT_S3_ACCESS_KEY" env-default:""`
	FreeCloudDefaultS3SecretKey      string  `env:"FREE_CLOUD_DEFAULT_S3_SECRET_KEY" env-default:""`
	FreeCloudDefaultS3Endpoint       string  `env:"FREE_CLOUD_DEFAULT_S3_ENDPOINT" env-default:""`
	FreeCloudDefaultS3BucketName     string  `env:"FREE_CLOUD_DEFAULT_S3_BUCKET_NAME" env-default:""`
	FreeCloudDefaultS3PublicBaseURL  string  `env:"FREE_CLOUD_DEFAULT_S3_PUBLIC_BASE_URL" env-default:""`
	FreeCloudDefaultS3ForcePathStyle bool    `env:"FREE_CLOUD_DEFAULT_S3_FORCE_PATH_STYLE" env-default:"true"`
	FreeCloudStorageLimitGB          float64 `env:"FREE_CLOUD_STORAGE_LIMIT_GB" env-default:"0.5"`

	// Resend API key for transactional email (team invites, etc.).
	ResendAPIKey string `env:"RESEND_API_KEY" env-default:""`

	// Admin password reset: secret required to call POST /admin/reset-password (e.g. set in ~/.apito/bin/.env)
	AdminResetSecret string `env:"APITO_ADMIN_RESET_SECRET" env-default:""`

	// Optional driver factory for dependency injection. Required at runtime.
	DriverFactory interface{} `env:"-"` // type-asserted to interfaces.DatabaseDriverFactory

	// RealtimeBusFactory creates the realtime fan-out bus (NATS, memory, cloudflare stub, etc.).
	RealtimeBusFactory interface{} `env:"-"` // interfaces.RealtimeBusFactory

	// CacheFactory creates the project/app cache driver.
	CacheFactory interface{} `env:"-"` // interfaces.CacheFactory

	// KVFactory creates the key-value service driver.
	KVFactory interface{} `env:"-"` // interfaces.KVFactory

	// PluginHost loads and supervises third-party plugins.
	PluginHost interface{} `env:"-"` // interfaces.PluginHost

	// DatabaseCheckWrapper optionally wraps the system database check HTTP handler.
	// Type: func(auth any) echo.HandlerFunc (router type-asserts; avoids importing echo here).
	DatabaseCheckWrapper interface{} `env:"-"`

	// --- Extension hooks (pro layer registers implementations at startup) ---

	// ConnectionRoutingHook returns a scope key for sub-project connection isolation.
	// If it returns (key, true) with a non-empty key, the executor uses a scoped connection.
	ConnectionRoutingHook func(ctx context.Context, projectID string) (scopeKey string, ok bool) `env:"-"`

	// QueryFilterHook returns additional filters to apply before every query (e.g. row-level isolation).
	QueryFilterHook func(ctx context.Context, params *CommonSystemParams) []*QueryFilter `env:"-"`

	// DocumentPreInsertHook is called before a document is inserted; can mutate the document or return an error.
	DocumentPreInsertHook func(ctx context.Context, params *CommonSystemParams, doc map[string]interface{}) error `env:"-"`

	// DocumentPreInsertDocHook is called after the driver builds *types.DefaultDocumentStructure and before persist.
	// Pro may mutate the struct; open-core must not interpret field semantics.
	DocumentPreInsertDocHook func(ctx context.Context, params *CommonSystemParams, doc *types.DefaultDocumentStructure) error `env:"-"`

	// PostTokenValidateHook runs after token validation succeeds (bearer or cookie path).
	// Pro may read scopes/headers/cookies and stamp echo context; open-core stays policy-free.
	PostTokenValidateHook func(ctx echo.Context, claims *TokenClaims) `env:"-"`

	// DDLPostCreateHook is called after a model table/collection is created; can add columns or indexes.
	DDLPostCreateHook func(ctx context.Context, project *Project, model *ModelType, dbHandle interface{}) error `env:"-"`

	// PreCreateModelHook lets extensions mutate ModelType before create-model DDL (pro: tenant/common flags).
	PreCreateModelHook func(model *ModelType, args map[string]interface{}) `env:"-"`

	// ApplyModelUpdateHook lets extensions mutate ModelType during updateModel; return true when Ext changed.
	ApplyModelUpdateHook func(model *ModelType, args map[string]interface{}) bool `env:"-"`

	// PostDocumentInsertHook is called after a document is successfully inserted.
	PostDocumentInsertHook func(ctx context.Context, params *CommonSystemParams, docID string) error `env:"-"`

	// SchemaIterateHook is called when a schema change needs to propagate to sub-project databases.
	SchemaIterateHook func(ctx context.Context, project *Project, fn func(ctx context.Context, driver interface{}) error) error `env:"-"`

	// SkipSchemaBaseDDLHook lets extensions skip base project physical DDL for tenant-only storage layouts.
	SkipSchemaBaseDDLHook func(ctx context.Context, project *Project) bool `env:"-"`

	// PostSchemaChangeHook runs after a successful schema orchestration commit (pro: Turso Sync flush).
	PostSchemaChangeHook func(ctx context.Context, baseDriver interface{}, project *Project) `env:"-"`

	// SchemaMutationHook runs before schema DDL orchestration; handled=true skips runSchemaChange.
	SchemaMutationHook SchemaMutationHook `env:"-"`

	// SchemaVersioningEnabled stages schema mutations for review (pro registers the hook).
	SchemaVersioningEnabled bool `env:"PRO_SCHEMA_VERSIONING_ENABLED" env-default:"true"`
	// SchemaVersioningBypass applies schema mutations immediately through orchestration.
	SchemaVersioningBypass bool `env:"PRO_SCHEMA_VERSIONING_BYPASS" env-default:"false"`

	// TokenClaimsHook allows the pro layer to inject additional claims into JWT/token payloads.
	TokenClaimsHook func(project *Project, claims map[string]interface{}) `env:"-"`

	// ProjectAPITokenClaimsHook allows optional claim enrichment before project API key issuance.
	// Open-core stays policy-free and simply invokes this hook when set.
	ProjectAPITokenClaimsHook func(ctx echo.Context, project *Project, claims *TokenClaims) `env:"-"`

	// ProjectUserGraphQLHooks allows the host to override project end-user GraphQL resolvers before the open-core default.
	// Type: *resolver.ProjectUserGraphQLHooks (set by pro at boot).
	ProjectUserGraphQLHooks interface{} `env:"-"`

	// ProjectUserItemFieldsHook lets the host extend the project end-user GraphQL object.
	// Type: resolver.ProjectUserItemFieldsHook (set by pro at boot). Open-core does not name host fields.
	ProjectUserItemFieldsHook interface{} `env:"-"`

	// ProjectUserGraphQLOperationFieldHook lets the host extend Args (or other field config) on named
	// project end-user operations (createUser, searchUsers, …). Type: resolver.ProjectUserGraphQLOperationFieldHook.
	ProjectUserGraphQLOperationFieldHook interface{} `env:"-"`

	// ProjectUserAPITokenHook lets the host adjust API token type/scopes for app end-user login.
	// Open-core default: tokenType "user", scopes ["project:<projectID>"].
	ProjectUserAPITokenHook func(cache *ApplicationCache, userID, role string) (tokenType string, scopes []string) `env:"-"`

	// EnsureScopedDatabaseHook runs before default scoped DB creation (e.g. Postgres/MySQL per-scope isolation).
	EnsureScopedDatabaseHook func(ctx context.Context, projectID string, base, derived *DriverCredentials) error `env:"-"`

	// LoadProjectCacheHook allows the pro layer to modify a project after loading from the system DB.
	LoadProjectCacheHook func(ctx context.Context, project *Project) `env:"-"`

	// RealtimeTopicHook lets the host rewrite a realtime subscription topic before
	// publish/subscribe (e.g. inject a tenant scope prefix). Open-core builds a
	// neutral base topic and applies this hook identically on both the emit and
	// subscribe sides so they match. Open-core does not encode tenant semantics.
	RealtimeTopicHook func(ctx context.Context, baseTopic string) string `env:"-"`

	// NamingV2ArangoPerModelCollections is used when applying Arango naming V2 physical migration:
	// true means one document collection per model layout; false uses a single p_{projectId} bucket.
	NamingV2ArangoPerModelCollections func(ctx context.Context, project *Project) bool `env:"-"`

	// NamingV2RelationTenantModel returns the tenant root model name for relation edges (e.g. "restaurant").
	// When set, Arango naming migration moves legacy root-level tenant_id into ext and sets ext.tenant_model.
	NamingV2RelationTenantModel func(ctx context.Context, project *Project) string `env:"-"`

	// BuildSystemParamHook allows the pro layer to enrich CommonSystemParams after the base build.
	BuildSystemParamHook func(ctx context.Context, project *Project, param *CommonSystemParams) `env:"-"`

	// InitProjectBaseHook is for extending the project base initialization.
	// driver is the concrete ProjectDBInterface implementation. Default when nil: type-assert and call InitProjectBase.
	InitProjectBaseHook func(ctx context.Context, driver interface{}, param *CommonSystemParams) error `env:"-"`

	// ProjectTypeForClaims maps open-core Project to a JWT "project_type" value (e.g. int32). OSS leaves nil and JWT uses "general".
	ProjectTypeForClaims func(*Project) interface{} `env:"-"`

	// PostApplicationCacheHook runs after GetApplicationCache assembles cache (param, ctx, plugins).
	// Pro may enrich cache.Ctx / cache.Param here so resolvers using the embedded *GraphQLServer see the same context
	// as the outer server. Type: func(echo.Context, *ApplicationCache).
	PostApplicationCacheHook interface{} `env:"-"`

	// SchemaObjectsExtensionHook runs inside BuildServerQueriesAndMutations right after
	// InitPrivateObjects(). The pro layer uses it to AddFieldConfig on core schema objects
	// (e.g. ModelType, ProjectModel) before the schema is sent through channels.
	SchemaObjectsExtensionHook func(objs interface{}) `env:"-"`

	// --- Public GraphQL schema builder (per-request dynamic schema) ---

	// MaxModelsPerProject caps models processed by publicSchemaBuilder (0 = no limit).
	MaxModelsPerProject int `env:"MAX_MODELS_PER_PROJECT" env-default:"0"`

	// SQLite / connection pool tunables (SaaS capacity).
	// SQLITE_CACHE_SIZE_KB is the PRAGMA cache_size magnitude in KiB (negative form applied).
	SQLiteCacheSizeKB int `env:"SQLITE_CACHE_SIZE_KB" env-default:"20000"`
	// SQLITE_MMAP_BYTES caps mmap_size on local SQLite files.
	SQLiteMmapBytes int64 `env:"SQLITE_MMAP_BYTES" env-default:"134217728"`
	// MAX_HOT_CONNECTIONS caps ConnectionManager pooled project/tenant drivers (0 = engine default 1000).
	MaxHotConnections int `env:"MAX_HOT_CONNECTIONS" env-default:"0"`
	// CONN_TTL_MINUTES is the ConnectionManager entry TTL in minutes (0 = engine default 120).
	ConnTTLMinutes int `env:"CONN_TTL_MINUTES" env-default:"0"`

	// FunctionRuntimeMode: local (in-process worker via LocalTransport) or nats (distributed).
	FunctionRuntimeMode string `env:"FUNCTION_RUNTIME_MODE" env-default:"local"`
	// FunctionGlobalConcurrency caps concurrent Apito Function invocations process-wide.
	FunctionGlobalConcurrency int `env:"FUNCTION_GLOBAL_CONCURRENCY" env-default:"16"`
	// FunctionCallableAuthMode: secret (X-Fn-Hash must match RestAPISecretURLKey) or disabled.
	FunctionCallableAuthMode string `env:"FUNCTION_CALLABLE_AUTH_MODE" env-default:"secret"`

	// FunctionLimitsHook lets pro supply plan/tier quotas for Apito Functions.
	FunctionLimitsHook func(ctx context.Context, projectID string, fn *ApitoFunction) (memoryBytes int64, timeoutMs int64, maxConcurrency int, err error) `env:"-"`

	// FunctionTenantScopeMode selects draft-test vs live callable tenant policy.
	// Values: "draft_test" | "live" (see FunctionTenantScope* constants below).

	// FunctionTenantScopeHook resolves and validates tenant scope for function
	// invocation without importing Pro types into open-core. It must inject
	// routing keys onto cache.Ctx (typed Pro keys) and cache.Param.Ext so
	// ConnectionRoutingHook and row filters share one validated scope.
	// Returns the resolved tenant id (may be empty for non-SaaS / shared-DB).
	FunctionTenantScopeHook func(
		ctx context.Context,
		cache *ApplicationCache,
		mode FunctionTenantScopeMode,
		explicitTenantID string,
	) (tenantID string, err error) `env:"-"`

	// FunctionCallableAuthHook runs on REST /function for SaaS projects before
	// invoke. Pro uses it to require a verified app-user Bearer JWT (in addition
	// to X-Fn-Hash). Non-SaaS may leave this nil (hash-only remains valid).
	FunctionCallableAuthHook func(c echo.Context, project *Project) error `env:"-"`

	// EnableCompiledSchemaCache caches pre-connection GraphQL shape (fingerprint: project + role + schema).
	EnableCompiledSchemaCache bool `env:"ENABLE_COMPILED_SCHEMA_CACHE" env-default:"false"`

	// EnableClosureFreeResolvers reserved for future resolver refactors (relation fields).
	EnableClosureFreeResolvers bool `env:"ENABLE_CLOSURE_FREE_RESOLVERS" env-default:"false"`

	// RoleAgnosticSchemaCache builds one superset schema per project (pre-connection cache key omits role); resolvers enforce real role.
	RoleAgnosticSchemaCache bool `env:"ROLE_AGNOSTIC_SCHEMA_CACHE" env-default:"false"`

	// AdjustPublicSchemaForRequestHook runs after collectFilteredModelsForPublicSchema and may
	// mutate permissions and filteredModels (e.g. remove SaaS tenant control-plane model roots).
	// When set, the compiled public schema cache fingerprint includes the effective API permission map.
	AdjustPublicSchemaForRequestHook func(ctx context.Context, cache *ApplicationCache, project *Project, permissions map[string]*APIPermission, filteredModels *[]*PublicSchemaModelFilter) error `env:"-"`

	// SchemaBuildTelemetry emits OTel spans around publicSchemaBuilder when true.
	SchemaBuildTelemetry bool `env:"SCHEMA_BUILD_TELEMETRY" env-default:"true"`

	// SchemaBuildMetrics registers OTel counter schema_build_total and histogram schema_build_duration_seconds.
	SchemaBuildMetrics bool `env:"SCHEMA_BUILD_METRICS" env-default:"false"`

	// MetricsEnabled gates apito_* OpenTelemetry instruments (HTTP, GraphQL, pool, DB decorator, cache, KV, queue, session).
	// When false, telemetry helpers no-op. When true, instruments record if a global MeterProvider is registered (OSS or extended builds).
	MetricsEnabled bool `env:"METRICS_ENABLED" env-default:"true"`

	// SystemMetricsToken is an optional Bearer secret compared by the router when a build exposes a protected metrics scrape path.
	// Open-core does not mount that route; deployments that add one should set this in production.
	SystemMetricsToken string `env:"SYSTEM_METRICS_TOKEN" env-default:""`

	// OTELExporterOTLPEndpoint optional OTLP HTTP endpoint for traces. Empty disables OTLP trace export for builds that wire a TracerProvider.
	OTELExporterOTLPEndpoint string `env:"OTEL_EXPORTER_OTLP_ENDPOINT" env-default:""`
}

// FunctionTenantScopeMode selects draft-test vs live callable tenant policy.
type FunctionTenantScopeMode string

const (
	FunctionTenantScopeDraftTest FunctionTenantScopeMode = "draft_test"
	FunctionTenantScopeLive      FunctionTenantScopeMode = "live"
)
