package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/apito-io/buffers/shared"
	_const "github.com/apito-io/databasedriver"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/router"
	"github.com/joho/godotenv"
)

var configs map[string]string
var cfg *models.Config

func init() {
	cfg = &models.Config{
		CookieDomain: getConfig("COOKIE_DOMAIN", "localhost"),
		CORSOrigin:   getConfig("CORS_ORIGIN", "http://localhost:3005"),

		// auth-provider
		AuthServiceProvider: getConfig("AUTH_SERVICE_PROVIDER", "local"),
		TokenTTL:            getConfig("TOKEN_TTL", "60"),

		KeyValueEngine: getEnv("KV_ENGINE", _const.BoltDriver),
	}
}

func main() {

	var err error
	configs, err = godotenv.Read(".env")
	if err != nil {
		panic(err)
	}

	// System Database Initialization
	if val, ok := configs["SYSTEM_DB_ENGINE"]; val != "" && ok {
		cfg.SystemDatabaseEngine = val
		cfg.SystemDatabaseDBConfig = &shared.CommonDatabaseConfig{
			Host:     checkConfig("SYSTEM_DB_HOST", true),
			Port:     checkConfig("SYSTEM_DB_PORT", true),
			User:     checkConfig("SYSTEM_DB_USER", true),
			Password: checkConfig("SYSTEM_DB_PASSWORD", true),
			Database: checkConfig("SYSTEM_DB_NAME", true),
		}
	} else {
		panic("you have to choose a system database. env variable SYSTEM_DB_ENGINE is missing")
	}

	// Project Database Initialization
	if val, ok := configs["PROJECT_DB_ENGINE"]; val != "" && ok {
		cfg.ProjectDatabaseEngine = val
		cfg.ProjectDatabaseDBConfig = &shared.CommonDatabaseConfig{
			Host:     checkConfig("PROJECT_DB_HOST", true),
			Port:     checkConfig("PROJECT_DB_PORT", true),
			User:     checkConfig("PROJECT_DB_USER", true),
			Password: checkConfig("PROJECT_DB_PASSWORD", true),
			Database: checkConfig("PROJECT_DB_NAME", true),
		}
	} else {
		panic("you have to choose a project database. env variable PROJECT_DB_ENGINE is missing")
	}

	// Init Router
	_route, err := router.InitRouter(cfg)
	if err != nil {
		//ae.EwP(err, "Router Init")
		return
	}

	fmt.Println("making tcp connection ready for router")

	// Listen must be called before Ready
	ln, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.ServePort))
	if err != nil {
		log.Fatalln("Can't listen:", err)
	}
	//defer ln.Close()

	_route.Listener = ln
	fmt.Println("starting the router")

	if cfg.TLS == "true" {
		err = _route.StartTLS("", cfg.CertPath, cfg.CacheDBConfig)
		if err != nil {
			panic(err.Error())
		}
	} else {
		err = _route.Start("")
		if err != nil {
			panic(err.Error())
		}
	}

	// Wait for connections to drain.
	//_route.Shutdown(context.Background())
}

func getEnv(key, defaults string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaults
}

func getConfig(key, defaults string) string {
	if value, ok := configs[key]; ok {
		return value
	}
	return defaults
}

func checkConfig(key string, required bool) string {
	if value, ok := configs[key]; ok {
		return value
	}
	if required {
		panic("missing env variable " + key)
	}
	return ""
}

func fmtConfig(cfg *models.Config) []byte {
	str, err := json.MarshalIndent(cfg, " ", " ")
	if err != nil {
		return nil
	}
	return str
}
