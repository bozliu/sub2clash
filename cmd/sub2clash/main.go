package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/bozliu/sub2clash/internal/sub2clash"
)

func main() {
	log.SetFlags(0)
	if err := run(os.Args[1:]); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printRootHelp()
		return nil
	}

	switch args[0] {
	case "serve":
		return runServe(args[1:])
	case "convert":
		return runConvert(args[1:])
	case "profile":
		return runProfile(args[1:])
	case "help", "-h", "--help":
		printRootHelp()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	dataDir := fs.String("data-dir", "", "data directory")
	addr := fs.String("addr", "", "http listen address")
	baseURL := fs.String("base-url", "", "public base URL")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := sub2clash.NewConfig(*dataDir, *addr, *baseURL)
	if err != nil {
		return err
	}
	store, err := sub2clash.OpenStore(context.Background(), cfg.DataDir)
	if err != nil {
		return err
	}
	defer store.Close()
	service, err := sub2clash.NewService(cfg, store)
	if err != nil {
		return err
	}
	server := sub2clash.NewServer(cfg, service)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return server.Run(ctx)
}

func runConvert(args []string) error {
	fs := flag.NewFlagSet("convert", flag.ContinueOnError)
	sourceURL := fs.String("url", "", "subscription url")
	outPath := fs.String("out", "", "output yaml file")
	dataDir := fs.String("data-dir", "", "data directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *sourceURL == "" || *outPath == "" {
		return errors.New("convert requires --url and --out")
	}

	cfg, err := sub2clash.NewConfig(*dataDir, "", "")
	if err != nil {
		return err
	}
	fetcher := sub2clash.NewFetcher(cfg)
	body, headers, _, err := fetcher.Fetch(context.Background(), *sourceURL, sub2clash.FetchStrategy{})
	if err != nil {
		return err
	}
	result, err := sub2clash.ConvertRawSubscription(body, headers, cfg.AutoTestURL)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(*outPath, result.YAML, 0o644); err != nil {
		return err
	}
	fmt.Printf("Wrote Clash YAML to %s\n", *outPath)
	for _, warning := range result.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
	if result.SubscriptionInfo != "" {
		fmt.Printf("subscription-userinfo: %s\n", result.SubscriptionInfo)
	}
	return nil
}

func runProfile(args []string) error {
	if len(args) == 0 {
		return errors.New("profile requires a subcommand: add, refresh, list")
	}
	switch args[0] {
	case "add":
		return runProfileAdd(args[1:])
	case "refresh":
		return runProfileRefresh(args[1:])
	case "list":
		return runProfileList(args[1:])
	default:
		return fmt.Errorf("unknown profile subcommand %q", args[0])
	}
}

func runProfileAdd(args []string) error {
	fs := flag.NewFlagSet("profile add", flag.ContinueOnError)
	name := fs.String("name", "", "profile name")
	sourceURL := fs.String("url", "", "subscription url")
	refresh := fs.String("refresh", "24h", "refresh interval")
	dataDir := fs.String("data-dir", "", "data directory")
	baseURL := fs.String("base-url", "", "public base URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *sourceURL == "" {
		return errors.New("profile add requires --url")
	}
	refreshInterval, err := sub2clash.ParseRefreshInterval(*refresh)
	if err != nil {
		return err
	}
	cfg, err := sub2clash.NewConfig(*dataDir, "", *baseURL)
	if err != nil {
		return err
	}
	store, err := sub2clash.OpenStore(context.Background(), cfg.DataDir)
	if err != nil {
		return err
	}
	defer store.Close()
	service, err := sub2clash.NewService(cfg, store)
	if err != nil {
		return err
	}
	managed, err := service.AddProfile(context.Background(), *name, *sourceURL, refreshInterval)
	if err != nil {
		return err
	}
	fmt.Printf("Created profile %s\n", managed.ID)
	fmt.Printf("Managed URL: %s\n", managed.ManagedURL)
	fmt.Printf("Download URL: %s\n", managed.DownloadURL)
	for _, warning := range managed.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
	return nil
}

func runProfileRefresh(args []string) error {
	fs := flag.NewFlagSet("profile refresh", flag.ContinueOnError)
	dataDir := fs.String("data-dir", "", "data directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("profile refresh requires <id>")
	}
	id := fs.Arg(0)
	cfg, err := sub2clash.NewConfig(*dataDir, "", "")
	if err != nil {
		return err
	}
	store, err := sub2clash.OpenStore(context.Background(), cfg.DataDir)
	if err != nil {
		return err
	}
	defer store.Close()
	service, err := sub2clash.NewService(cfg, store)
	if err != nil {
		return err
	}
	result, err := service.RefreshProfile(context.Background(), id)
	if err != nil {
		return err
	}
	fmt.Printf("Refreshed %s\n", id)
	for _, warning := range result.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
	return nil
}

func runProfileList(args []string) error {
	fs := flag.NewFlagSet("profile list", flag.ContinueOnError)
	dataDir := fs.String("data-dir", "", "data directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := sub2clash.NewConfig(*dataDir, "", "")
	if err != nil {
		return err
	}
	store, err := sub2clash.OpenStore(context.Background(), cfg.DataDir)
	if err != nil {
		return err
	}
	defer store.Close()
	service, err := sub2clash.NewService(cfg, store)
	if err != nil {
		return err
	}
	items, err := service.ListProfiles(context.Background())
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tREFRESH\tLAST_SUCCESS\tLAST_ERROR")
	for _, item := range items {
		lastSuccess := "-"
		if !item.LastSuccessAt.IsZero() {
			lastSuccess = item.LastSuccessAt.Format(time.RFC3339)
		}
		lastError := item.LastError
		if lastError == "" {
			lastError = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", item.ID, item.Name, item.RefreshInterval, lastSuccess, lastError)
	}
	return tw.Flush()
}

func printRootHelp() {
	fmt.Println(`Sub2Clash

Usage:
  sub2clash serve [--data-dir PATH] [--addr :8080] [--base-url URL]
  sub2clash convert --url <source> --out <file>
  sub2clash profile add --name <name> --url <source> [--refresh 24h]
  sub2clash profile refresh <id>
  sub2clash profile list`)
}
