package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"mr-queue/internal/app"
	"mr-queue/internal/config"
	"mr-queue/internal/server"
)

var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "version", "--version", "-version":
		printVersion()
	case "serve":
		serve(os.Args[2:])
	case "sync-queue":
		syncQueue(os.Args[2:])
	case "run":
		run(os.Args[2:])
	case "dry-run":
		dryRun(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func serve(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", "mr-queue.yml", "path to YAML config")
	envPath := fs.String("env", "", "path to .env file")
	statePath := fs.String("state", "", "path to state JSON file")
	addr := fs.String("addr", "127.0.0.1:8787", "listen address")
	_ = fs.Parse(args)

	runtime, err := app.Build(*configPath, *envPath, *statePath)
	if err != nil {
		log.Fatal(err)
	}
	srv := server.New(runtime)
	fmt.Printf("mr-queue web panel: http://%s/\n", *addr)
	log.Fatal(http.ListenAndServe(*addr, srv.Handler()))
}

func syncQueue(args []string) {
	fs := flag.NewFlagSet("sync-queue", flag.ExitOnError)
	configPath := fs.String("config", "mr-queue.yml", "path to YAML config")
	envPath := fs.String("env", "", "path to .env file")
	statePath := fs.String("state", "", "path to state JSON file")
	skipFetch := fs.Bool("skip-fetch", false, "use existing local refs without fetching remotes")
	_ = fs.Parse(args)

	runtime, err := app.Build(*configPath, *envPath, *statePath)
	if err != nil {
		log.Fatal(err)
	}
	runtime.Runner.SetSkipFetch(*skipFetch)
	count, err := runtime.Runner.SyncQueue()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("synced %d queue commits\n", count)
}

func run(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	configPath := fs.String("config", "mr-queue.yml", "path to YAML config")
	envPath := fs.String("env", "", "path to .env file")
	statePath := fs.String("state", "", "path to state JSON file")
	_ = fs.Parse(args)

	runtime, err := app.Build(*configPath, *envPath, *statePath)
	if err != nil {
		log.Fatal(err)
	}
	if err := runtime.Runner.RunOnce(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("run complete")
}

func dryRun(args []string) {
	fs := flag.NewFlagSet("dry-run", flag.ExitOnError)
	configPath := fs.String("config", "mr-queue.yml", "path to YAML config")
	envPath := fs.String("env", "", "path to .env file")
	_ = fs.Parse(args)

	cfg, err := config.Load(*configPath, *envPath)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(cfg.Safe())
}

func printVersion() {
	fmt.Printf("mr-queue %s\ncommit: %s\nbuilt: %s\n", version, commit, buildDate)
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  mr-queue version")
	fmt.Fprintln(os.Stderr, "  mr-queue serve --config mr-queue.yml [--env .env] [--addr 127.0.0.1:8787]")
	fmt.Fprintln(os.Stderr, "  mr-queue sync-queue --config mr-queue.yml [--env .env] [--skip-fetch]")
	fmt.Fprintln(os.Stderr, "  mr-queue run --config mr-queue.yml [--env .env]")
	fmt.Fprintln(os.Stderr, "  mr-queue dry-run --config mr-queue.yml [--env .env]")
}
