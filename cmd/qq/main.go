package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	cfg, err := loadUserConfig(defaultConfigPath())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	flags := flag.NewFlagSet("qq", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	backendName := flags.String("backend", defaultBackendName(cfg), "Backend name to use")
	cwdContext := flags.Bool("cwd-context", false, "Use current working directory as backend context")

	if err := flags.Parse(args); err != nil {
		return 2
	}

	question := strings.TrimSpace(strings.Join(flags.Args(), " "))
	if question == "" {
		fmt.Fprintln(os.Stderr, `usage: qq [--backend name] [--cwd-context] "your question"`)
		return 2
	}

	backend, err := resolveBackend(*backendName, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	if _, err := resolveBinaryPath(backend.Path); err != nil {
		fmt.Fprintf(os.Stderr, "%s is not installed, not executable, or not on PATH\n", backend.Path)
		return 127
	}

	backend = applyCurrentDirContext(backend, *cwdContext)

	return runBackend(backend, wrapQuestion(question))
}

func defaultBackendName(cfg userConfig) string {
	if strings.TrimSpace(cfg.DefaultBackend) == "" {
		return "codex"
	}

	return cfg.DefaultBackend
}
