package _const

const (
	// Apito DefaultCloud Database
	ApitoDB string = "apitoDB"

	// SQL driver
	MySQLDriver   string = "mysql"
	MariaDBDriver string = "mariadb"
	SQLiteDriver  string = "sqlite"

	PostgreSQLDriver string = "postgresql"
	SQLServerDriver  string = "sqlServer"
	OracleDriver     string = "oracle"

	// NoSQL driver
	MongoDBDriver   string = "mongodb"
	CouchbaseDriver string = "couchbase"

	// Cloud Driver
	DynamoDB  string = "dynamoDB"
	FireStore string = "firestore"

	// EmbeddedDB Embedded Database
	CoreDB string = "coredb"
	BoltDB string = "bbolt"

	// RedisDriver KeyValue database
	RedisDriver string = "redis"

	MemoryDB string = "memory" // usually its sync.Map{}
)
