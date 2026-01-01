// basar manages Volatility3 ISF symbol cache.
//
// It fetches, merges, and caches ISF banner files from multiple upstream
// sources, providing a unified cache for use with volatility3's -u flag.
//
// Usage:
//
//	basar [flags]
//
// Flags:
//
//	-p, --path           print cache file path
//	-u, --uri            print file:// URI (default output)
//	-s, --stats          print cache statistics as JSON
//	-c, --check          check if cache is valid (exit 0=valid, 2=invalid)
//	    --update         force cache update
//	    --smart-update   update only if sources changed (uses ETag/Last-Modified)
//	    --clear          remove cache file
//	    --init           create default config file
//	    --setup          complete setup (config, update, vol3 config, systemd)
//	    --install-service install systemd timer for auto-updates
//	    --configure-vol3  configure volatility3 to use basar
//	-v, --verbose        enable verbose output
//	-h, --help           show help
//
// Environment:
//
//	BASAR_TTL       cache TTL in seconds (default: 86400)
//	BASAR_VERBOSE   set to "1" for verbose output
//	XDG_CACHE_HOME     cache directory base (default: ~/.cache)
//	XDG_CONFIG_HOME    config directory base (default: ~/.config)
//
// Examples:
//
//	basar                          # ensure cache & print URI
//	basar --setup                  # complete setup (recommended for first run)
//	basar --update                 # force update
//	volatility3 -u $(basar) ...    # use with volatility3
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/calilkhalil/basar/internal/cache"
	"github.com/calilkhalil/basar/internal/config"
)

const (
	exitOK      = 0
	exitError   = 1
	exitInvalid = 2
)

// Flags holds parsed command-line flags.
type Flags struct {
	Path           bool
	URI            bool
	Stats          bool
	Check          bool
	Update         bool
	SmartUpdate    bool
	Clear          bool
	Init           bool
	Setup          bool
	InstallService bool
	ConfigureVol3  bool
	Verbose        bool
	Help           bool
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags, err := parseFlags(args)
	if err != nil {
		fmt.Fprintf(stderr, "basar: %v\n", err)
		return exitError
	}

	if flags.Help {
		printUsage(stdout)
		return exitOK
	}

	// Setup context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg := config.New()
	c := cache.New(cfg)

	// Handle verbose from env if not set via flag
	verbose := flags.Verbose || os.Getenv("BASAR_VERBOSE") == "1"

	// --setup: complete setup
	if flags.Setup {
		if err := c.Setup(ctx, verbose); err != nil {
			fmt.Fprintf(stderr, "basar: %v\n", err)
			return exitError
		}
		fmt.Fprintln(stdout, "setup complete")
		return exitOK
	}

	// --init: create config file
	if flags.Init {
		if err := cfg.InitConfig(); err != nil {
			fmt.Fprintf(stderr, "basar: %v\n", err)
			return exitError
		}
		fmt.Fprintln(stdout, cfg.ConfigFile)
		return exitOK
	}

	// --install-service: install systemd timer
	if flags.InstallService {
		if err := c.InstallService(); err != nil {
			fmt.Fprintf(stderr, "basar: %v\n", err)
			return exitError
		}
		fmt.Fprintln(stdout, "systemd timer installed")
		return exitOK
	}

	// --configure-vol3: configure volatility3
	if flags.ConfigureVol3 {
		if err := c.ConfigureVolatility3(); err != nil {
			fmt.Fprintf(stderr, "basar: %v\n", err)
			return exitError
		}
		fmt.Fprintln(stdout, "volatility3 configured")
		return exitOK
	}

	// --clear: remove cache
	if flags.Clear {
		if err := c.Clear(); err != nil {
			fmt.Fprintf(stderr, "basar: %v\n", err)
			return exitError
		}
		return exitOK
	}

	// --smart-update: update only if changed
	if flags.SmartUpdate {
		if verbose {
			fmt.Fprintf(stderr, "checking %d sources for updates\n", len(cfg.Sources))
		}
		updated, err := c.SmartUpdate(ctx, verbose)
		if err != nil {
			fmt.Fprintf(stderr, "basar: %v\n", err)
			return exitError
		}
		if verbose {
			if updated {
				stats := c.Stats()
				fmt.Fprintf(stderr, "updated: %d banners cached\n", stats.Entries)
			} else {
				fmt.Fprintln(stderr, "no changes")
			}
		}
		return exitOK
	}

	// --update: force update
	if flags.Update {
		if verbose {
			fmt.Fprintf(stderr, "updating from %d sources\n", len(cfg.Sources))
		}
		if err := c.Update(ctx, true); err != nil {
			fmt.Fprintf(stderr, "basar: %v\n", err)
			return exitError
		}
		if verbose {
			stats := c.Stats()
			fmt.Fprintf(stderr, "cached %d banners\n", stats.Entries)
		}
		return exitOK
	}

	// --check: verify cache validity
	if flags.Check {
		if c.IsValid() {
			return exitOK
		}
		return exitInvalid
	}

	// --stats: print statistics
	if flags.Stats {
		stats := c.Stats()
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(stats)
		return exitOK
	}

	// Ensure cache is valid for path/uri output
	if err := c.Ensure(ctx); err != nil {
		fmt.Fprintf(stderr, "basar: %v\n", err)
		return exitError
	}

	// --path: print file path
	if flags.Path {
		path, ok := c.Path()
		if !ok {
			return exitInvalid
		}
		fmt.Fprintln(stdout, path)
		return exitOK
	}

	// Default (or --uri): print file:// URI
	uri, ok := c.URI()
	if !ok {
		return exitInvalid
	}
	fmt.Fprintln(stdout, uri)
	return exitOK
}

func parseFlags(args []string) (*Flags, error) {
	fs := flag.NewFlagSet("basar", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // Handle errors manually

	flags := &Flags{}

	fs.BoolVar(&flags.Path, "p", false, "")
	fs.BoolVar(&flags.Path, "path", false, "")
	fs.BoolVar(&flags.URI, "u", false, "")
	fs.BoolVar(&flags.URI, "uri", false, "")
	fs.BoolVar(&flags.Stats, "s", false, "")
	fs.BoolVar(&flags.Stats, "stats", false, "")
	fs.BoolVar(&flags.Check, "c", false, "")
	fs.BoolVar(&flags.Check, "check", false, "")
	fs.BoolVar(&flags.Update, "update", false, "")
	fs.BoolVar(&flags.SmartUpdate, "smart-update", false, "")
	fs.BoolVar(&flags.Clear, "clear", false, "")
	fs.BoolVar(&flags.Init, "init", false, "")
	fs.BoolVar(&flags.Init, "init-config", false, "")
	fs.BoolVar(&flags.Setup, "setup", false, "")
	fs.BoolVar(&flags.InstallService, "install-service", false, "")
	fs.BoolVar(&flags.ConfigureVol3, "configure-vol3", false, "")
	fs.BoolVar(&flags.Verbose, "v", false, "")
	fs.BoolVar(&flags.Verbose, "verbose", false, "")
	fs.BoolVar(&flags.Help, "h", false, "")
	fs.BoolVar(&flags.Help, "help", false, "")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	return flags, nil
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `basar - Volatility3 ISF symbol cache manager

Usage: basar [options]

Options:
  -p, --path            print cache file path
  -u, --uri             print file:// URI (default output)
  -s, --stats           print cache statistics as JSON
  -c, --check           check if cache is valid (exit 0=valid, 2=invalid)
      --update          force cache update
      --smart-update    update only if sources changed
      --clear           remove cache file
      --init            create default config file
      --setup           complete setup (recommended for first use)
      --install-service install systemd timer for auto-updates
      --configure-vol3  configure volatility3 to use basar
  -v, --verbose         enable verbose output
  -h, --help            show this help

Environment:
  BASAR_TTL      cache TTL in seconds (default: 86400)
  BASAR_VERBOSE  set to "1" for verbose output

First time? Run:
  basar --setup

This will:
  1. Create config file with default sources
  2. Download and cache ISF banners
  3. Configure volatility3 to use basar automatically
  4. Install systemd timer for auto-updates (Linux)

After setup, just run:
  volatility3 -f dump.raw linux.pslist

Config: ~/.config/basar/sources.conf (one URL/path per line)
`)
}
