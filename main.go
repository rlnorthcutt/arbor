package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/rlnorthcutt/arbor/internal/builder"
	"github.com/rlnorthcutt/arbor/internal/config"
	"github.com/rlnorthcutt/arbor/internal/scaffold"
	"github.com/rlnorthcutt/arbor/internal/server"
	"github.com/rlnorthcutt/cmdkit/logger"
	"github.com/rlnorthcutt/cmdkit/ui"
)

// version is set at build time via -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	root := flag.String("root", "./", "Project root directory")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()

	log := logger.New(*verbose)
	userUI := ui.New(false).
		WithLogger(log).
		WithInterrupt(context.Background())
	defer userUI.StopSignal()

	args := flag.Args()
	if len(args) == 0 {
		printHelp()
		return
	}

	switch args[0] {
	case "init":
		handleInit(*root, args[1:], log)

	case "new":
		handleNew(*root, args[1:], log)

	case "build":
		handleBuild(*root, args[1:], log, userUI.Ctx)

	case "preview":
		handlePreview(*root, args[1:], log, userUI.Ctx)

	case "check":
		handleCheck(*root, log)

	case "version", "--version", "-v":
		fmt.Printf("arbor %s\n", version)

	case "help", "--help", "-h":
		printHelp()

	default:
		log.Error("Unknown command: %s", args[0])
		printHelp()
		os.Exit(1)
	}
}

func handleInit(root string, args []string, log *logger.Logger) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	blueprint := fs.String("blueprint", "blog", "Site blueprint: blog, marketing, docs")
	fs.Parse(args) //nolint

	if err := scaffold.Init(root, *blueprint, log); err != nil {
		log.Fatal("Init failed: %v", err)
	}
}

func handleNew(root string, args []string, log *logger.Logger) {
	if len(args) < 2 {
		log.Error("Usage: arbor new [type] [name]")
		log.Error("Example: arbor new blog my-first-post")
		os.Exit(1)
	}
	contentType := args[0]
	name := args[1]

	if err := scaffold.NewContent(root, contentType, name, log); err != nil {
		log.Fatal("new failed: %v", err)
	}
}

func handleBuild(root string, args []string, log *logger.Logger, ctx context.Context) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	force := fs.Bool("force", false, "Ignore cache, full rebuild")
	noMinify := fs.Bool("no-minify", false, "Disable CSS/JS minification")
	noAggregate := fs.Bool("no-aggregate", false, "Disable CSS/JS aggregation into bundles")
	fs.Parse(args) //nolint

	cfg, err := loadConfig(root, log)
	if err != nil {
		log.Fatal("Build setup failed: %v", err)
	}

	b, err := builder.New(root, log)
	if err != nil {
		log.Fatal("Build setup failed: %v", err)
	}

	opts := builder.BuildOptions{
		Force:           *force,
		MinifyAssets:    effectiveBool(cfg.Assets.Minify, true, *noMinify),
		AggregateAssets: effectiveBool(cfg.Assets.Aggregate, true, *noAggregate),
	}
	if err := b.Build(ctx, opts); err != nil {
		log.Fatal("Build failed: %v", err)
	}
}

func handlePreview(root string, args []string, log *logger.Logger, ctx context.Context) {
	fs := flag.NewFlagSet("preview", flag.ExitOnError)
	port := fs.Int("port", 8080, "Local port for preview server")
	force := fs.Bool("force", false, "Ignore cache, force full rebuild before serving")
	noMinify := fs.Bool("no-minify", false, "Enable minification during preview")
	noAggregate := fs.Bool("no-aggregate", false, "Enable aggregation during preview")
	fs.Parse(args) //nolint

	cfg, err := loadConfig(root, log)
	if err != nil {
		log.Fatal("Preview setup failed: %v", err)
	}

	s, err := server.New(root, *port, log)
	if err != nil {
		log.Fatal("Preview setup failed: %v", err)
	}

	opts := builder.BuildOptions{
		Force:           *force,
		MinifyAssets:    effectiveBool(cfg.Assets.Minify, false, *noMinify),
		AggregateAssets: effectiveBool(cfg.Assets.Aggregate, false, *noAggregate),
	}
	if err := s.Start(ctx, opts); err != nil {
		log.Fatal("Preview server failed: %v", err)
	}
}

func handleCheck(root string, log *logger.Logger) {
	b, err := builder.New(root, log)
	if err != nil {
		log.Fatal("Check failed: %v", err)
	}

	issues := b.Check()
	if issues == 0 {
		log.Success("All checks passed")
	} else {
		log.Error("%d issue(s) found", issues)
		os.Exit(1)
	}
}

// loadConfig loads config.toml from root, logging a warning and returning a
// default config if the file is missing or unparseable.
func loadConfig(root string, log *logger.Logger) (*config.Config, error) {
	cfg, err := config.Load(root)
	if err != nil {
		log.Warn("Could not load config.toml: %v", err)
		return config.Default(), nil
	}
	return cfg, nil
}

// effectiveBool resolves the final bool for a toggle:
//   - If the disableFlag is set (true), returns false.
//   - Else if cfgVal has an explicit pointer value, uses that.
//   - Otherwise returns modeDefault.
func effectiveBool(cfgVal *bool, modeDefault bool, disableFlag bool) bool {
	if disableFlag {
		return false
	}
	if cfgVal != nil {
		return *cfgVal
	}
	return modeDefault
}

func printHelp() {
	fmt.Print(`Arbor - Static Site Generator

Usage:
  arbor [OPTIONS] COMMAND

Commands:
  init          Initialize a new Arbor project in the current directory
                Flags: --blueprint  Site blueprint: blog, marketing, docs (default: blog)
  new           Create a new content file with scaffolded front matter
                Usage: arbor new [TYPE] [NAME]
                Example: arbor new blog my-first-post
  build         Build the site to /public
                Flags: --force         Ignore cache, full rebuild
                       --no-minify     Disable CSS/JS minification (default: enabled)
                       --no-aggregate  Disable CSS/JS bundling (default: enabled)
  preview       Build and serve locally with live reload
                Flags: --port          Local port (default: 8080)
                       --force         Ignore cache, force full rebuild
                       --no-minify     Enable minification during preview
                       --no-aggregate  Enable aggregation during preview
  check         Validate config, templates, and content without building
  version       Print the version
  help          Show this help message

Global Options:
  --root        Project root directory (default: ./)
  --verbose     Enable verbose logging

`)
}
