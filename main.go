package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/router"
	"github.com/cloudflare/tableflip"
	"github.com/getsentry/sentry-go"
	"github.com/ilyakaznacheev/cleanenv"
)

var cfg models.Config

func main() {

	// Load configuration from environment variables and .env file
	err := cleanenv.ReadConfig(".env", &cfg)
	if err != nil {
		fmt.Println("No .env file found, using environment variables only")
	}

	// If .env file doesn't exist, try to read from environment variables only
	err = cleanenv.ReadEnv(&cfg)
	if err != nil {
		panic(fmt.Sprintf("Failed to load configuration: %v", err))
	}

	if cfg.SentryKey != "" {
		err := sentry.Init(sentry.ClientOptions{
			Dsn: fmt.Sprintf("https://%s@%s/%s", cfg.SentryKey, cfg.SentryAPI, cfg.SentryProject),
		})
		if err != nil {
			ae.EwP(err, "Sentry Init")
		}
	} else {
		fmt.Println(string(fmtConfig(&cfg)))
	}

	upg, _ := tableflip.New(tableflip.Options{})
	defer upg.Stop()

	// By prefixing PID to log, easy to interrupt from another process.
	fmt.Println(fmt.Sprintf("[PID: %d]", os.Getpid()))

	// Listen for the process signal to trigger the tableflip upgrade.
	go func(_upg *tableflip.Upgrader) {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGHUP)
		for range sig {
			fmt.Println("getting signal to upgrade")
			err := _upg.Upgrade()
			if err != nil {
				fmt.Println(err.Error())
			}
		}
	}(upg)

	// Init Router
	_route, err := router.InitRouter(&cfg)
	if err != nil {
		//ae.EwP(err, "Router Init")
		return
	}

	// Print all registered routes for debugging
	// This will show all routes including plugin routes after they are loaded
	//router.PrintAllEchoRoutes(_route)

	fmt.Println("making tcp connection ready for router")

	// Listen must be called before Ready
	ln, err := upg.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", cfg.ServePort))
	if err != nil {
		log.Fatalln("Can't listen:", err)
	}
	//defer ln.Close()

	_route.Listener = ln
	fmt.Println("starting the router")
	go _route.Start("")

	// tableflip ready
	fmt.Println("initializing tableflip driver")
	if err := upg.Ready(); err != nil {
		panic(err)
	}

	<-upg.Exit()

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Make sure to set a deadline on exiting the process
	// after upg.Exit() is closed. No new upgrades can be
	// performed if the parent doesn't exit.
	time.AfterFunc(30*time.Second, func() {
		log.Println("Graceful shutdown timed out")
		os.Exit(1)
	})

	fmt.Println("Starting graceful shutdown...")

	// Shutdown server gracefully
	if err := _route.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	fmt.Println("Graceful shutdown completed")
}

func fmtConfig(cfg *models.Config) []byte {
	str, err := json.MarshalIndent(cfg, " ", " ")
	if err != nil {
		return nil
	}
	return str
}
