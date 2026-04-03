package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const defaultSystemPrompt = `
<system-reminder>This user asked a single quick question. Respond directly with one complete answer.

IMPORTANT CONTEXT:

- You are a newly created, lightweight agent instantiated only for this question.
- You do NOT have access to any earlier conversation, shared context, or workspace state. This interaction is entirely isolated and starts from scratch.
CRITICAL CONSTRAINTS:
- You have NO tools available except web or online search. You cannot inspect files, execute commands, or perform any other action.
- Respond in one turn, no follow up conversations.
- NEVER say things such as "Let me try...", "I'll now...", "Let me check...", or otherwise imply that you will take action.
- If the answer is unknown to you, say that plainly. Do not offer to look it up, verify it, or investigate further.

Now answer the question to your best knowledge.</system-reminder>
<user-question>
%s
</user-question>
`

const spinnerMessage = "Waiting for response..."

type backendConfig struct {
	binary string
	args   []string
	mode   backendMode
}

type backendMode string

const (
	backendModeStreaming backendMode = "streaming"
	backendModeCodexJSON backendMode = "codex_json"
)

type configuredBackend struct {
	Path string   `json:"path"`
	Args []string `json:"args"`
}

type userConfig struct {
	DefaultBackend string                       `json:"default_backend"`
	BackendPaths   map[string]string            `json:"backend_paths"`
	Backends       map[string]configuredBackend `json:"backends"`
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	cfg, err := loadUserConfig(defaultConfigPath())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	defaultBackend := "codex"
	if cfg.DefaultBackend != "" {
		defaultBackend = cfg.DefaultBackend
	}

	flags := flag.NewFlagSet("qq", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	backendName := flags.String("backend", defaultBackend, "Backend name to use")

	if err := flags.Parse(args); err != nil {
		return 2
	}

	question := strings.TrimSpace(strings.Join(flags.Args(), " "))
	if question == "" {
		fmt.Fprintln(os.Stderr, `usage: qq [--backend name] "your question"`)
		return 2
	}
	question = wrapQuestion(question)

	backendCfg, err := resolveBackend(*backendName, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	if _, err := resolveBinaryPath(backendCfg.binary); err != nil {
		fmt.Fprintf(os.Stderr, "%s is not installed, not executable, or not on PATH\n", backendCfg.binary)
		return 127
	}

	if backendCfg.mode == backendModeCodexJSON {
		return runCodex(backendCfg, question)
	}

	return runStreamingBackend(backendCfg, question)
}

func runStreamingBackend(cfg backendConfig, question string) int {
	cmd := exec.Command(cfg.binary, append(cfg.args, question)...)
	cmd.Stdin = os.Stdin

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	spinner := startSpinner(os.Stderr, shouldShowSpinner(os.Stderr), spinnerMessage)

	var stopOnce sync.Once
	stopSpinner := func() {
		stopOnce.Do(func() {
			spinner.Stop()
		})
	}

	results := make(chan copyResult, 2)
	var firstOutput sync.Once
	go func() {
		results <- copyResult{
			stream: "stdout",
			err:    proxyCommandOutput(os.Stdout, stdout, &firstOutput, stopSpinner),
		}
	}()
	go func() {
		results <- copyResult{
			stream: "stderr",
			err:    proxyCommandOutput(os.Stderr, stderr, &firstOutput, stopSpinner),
		}
	}()

	waitErr := cmd.Wait()
	stopSpinner()

	for i := 0; i < 2; i++ {
		result := <-results
		if result.err != nil {
			fmt.Fprintf(os.Stderr, "failed to stream backend %s: %v\n", result.stream, result.err)
			return 1
		}
	}

	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			return exitErr.ExitCode()
		}

		fmt.Fprintln(os.Stderr, waitErr)
		return 1
	}

	return 0
}

func wrapQuestion(question string) string {
	return fmt.Sprintf(defaultSystemPrompt, question)
}

func defaultConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}

	return filepath.Join(configDir, "qq", "config.json")
}

func loadUserConfig(path string) (userConfig, error) {
	if path == "" {
		return userConfig{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return userConfig{}, nil
		}
		return userConfig{}, fmt.Errorf("failed to read config file %q: %w", path, err)
	}

	var cfg userConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return userConfig{}, fmt.Errorf("failed to parse config file %q: %w", path, err)
	}

	return cfg, nil
}

func applyLegacyPathOverride(cfg backendConfig, backendName string, userCfg userConfig) backendConfig {
	if userCfg.BackendPaths == nil {
		return cfg
	}

	configuredPath := strings.TrimSpace(userCfg.BackendPaths[normalizeBackendName(backendName)])
	if configuredPath == "" {
		return cfg
	}

	cfg.binary = configuredPath
	return cfg
}

func resolveBackend(name string, userCfg userConfig) (backendConfig, error) {
	normalizedName := normalizeBackendName(name)
	if normalizedName == "" {
		return backendConfig{}, fmt.Errorf("backend name cannot be empty")
	}

	cfg, builtIn := buildBuiltinBackend(normalizedName)
	customCfg, hasCustom := lookupConfiguredBackend(userCfg.Backends, normalizedName)
	if !builtIn && !hasCustom {
		return backendConfig{}, fmt.Errorf("unsupported backend %q", name)
	}

	if hasCustom {
		cfg = mergeConfiguredBackend(cfg, customCfg)
		if !builtIn {
			cfg.mode = backendModeStreaming
		}
	}

	cfg = applyLegacyPathOverride(cfg, normalizedName, userCfg)

	if strings.TrimSpace(cfg.binary) == "" {
		return backendConfig{}, fmt.Errorf("backend %q must define a path", name)
	}

	if cfg.mode == backendModeCodexJSON && !hasArg(cfg.args, "--json") {
		return backendConfig{}, fmt.Errorf("backend %q must include --json in args", name)
	}

	return cfg, nil
}

func mergeConfiguredBackend(base backendConfig, custom configuredBackend) backendConfig {
	if path := strings.TrimSpace(custom.Path); path != "" {
		base.binary = path
	}
	if custom.Args != nil {
		base.args = append([]string(nil), custom.Args...)
	}
	return base
}

func lookupConfiguredBackend(backends map[string]configuredBackend, name string) (configuredBackend, bool) {
	for configuredName, cfg := range backends {
		if normalizeBackendName(configuredName) == name {
			return cfg, true
		}
	}

	return configuredBackend{}, false
}

func normalizeBackendName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func hasArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}

	return false
}

func resolveBinaryPath(binary string) (string, error) {
	if strings.ContainsRune(binary, os.PathSeparator) {
		info, err := os.Stat(binary)
		if err != nil {
			return "", err
		}
		if info.IsDir() {
			return "", fmt.Errorf("%q is a directory", binary)
		}
		return binary, nil
	}

	return exec.LookPath(binary)
}

func runCodex(cfg backendConfig, question string) int {
	cmd := exec.Command(cfg.binary, append(cfg.args, question)...)
	cmd.Stdin = os.Stdin

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	spinner := startSpinner(os.Stderr, shouldShowSpinner(os.Stderr), spinnerMessage)
	message, parseErr := extractCodexMessage(stdout)
	waitErr := cmd.Wait()
	spinner.Stop()

	if waitErr != nil {
		if stderr.Len() > 0 {
			_, _ = io.Copy(os.Stderr, &stderr)
			if !bytes.HasSuffix(stderr.Bytes(), []byte("\n")) {
				fmt.Fprintln(os.Stderr)
			}
		}

		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			return exitErr.ExitCode()
		}

		fmt.Fprintln(os.Stderr, waitErr)
		return 1
	}

	if parseErr != nil {
		fmt.Fprintln(os.Stderr, parseErr)
		return 1
	}

	if message == "" {
		fmt.Fprintln(os.Stderr, "codex completed without a final agent message")
		return 1
	}

	fmt.Fprintln(os.Stdout, message)
	return 0
}

type copyResult struct {
	stream string
	err    error
}

func proxyCommandOutput(dst io.Writer, src io.Reader, once *sync.Once, onFirstOutput func()) error {
	_, err := io.Copy(&firstOutputWriter{
		dst:           dst,
		once:          once,
		onFirstOutput: onFirstOutput,
	}, src)
	return err
}

type firstOutputWriter struct {
	dst           io.Writer
	once          *sync.Once
	onFirstOutput func()
}

func (w *firstOutputWriter) Write(p []byte) (int, error) {
	if len(p) > 0 && w.onFirstOutput != nil {
		if w.once != nil {
			w.once.Do(w.onFirstOutput)
		} else {
			w.onFirstOutput()
		}
	}

	return w.dst.Write(p)
}

func shouldShowSpinner(file *os.File) bool {
	if file == nil {
		return false
	}

	info, err := file.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}

type spinner struct {
	out      io.Writer
	enabled  bool
	message  string
	interval time.Duration
	frames   []byte
	done     chan struct{}
	stopped  chan struct{}
	mu       sync.Mutex
	lastLen  int
}

func startSpinner(out io.Writer, enabled bool, message string) *spinner {
	s := &spinner{
		out:      out,
		enabled:  enabled,
		message:  message,
		interval: 120 * time.Millisecond,
		frames:   []byte{'|', '/', '-', '\\'},
		done:     make(chan struct{}),
		stopped:  make(chan struct{}),
	}

	if !enabled {
		close(s.stopped)
		return s
	}

	go s.run()
	return s
}

func (s *spinner) Stop() {
	if !s.enabled {
		return
	}

	select {
	case <-s.done:
	default:
		close(s.done)
	}

	<-s.stopped
	s.clear()
}

func (s *spinner) run() {
	defer close(s.stopped)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for i := 0; ; i++ {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.render(i)
		}
	}
}

func (s *spinner) render(frame int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	text := fmt.Sprintf("\r%c %s", s.frames[frame%len(s.frames)], s.message)
	_, _ = io.WriteString(s.out, text)
	s.lastLen = len(text) - 1
}

func (s *spinner) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.lastLen == 0 {
		return
	}

	_, _ = io.WriteString(s.out, "\r"+strings.Repeat(" ", s.lastLen)+"\r")
	s.lastLen = 0
}

func extractCodexMessage(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var message string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event codexEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return "", fmt.Errorf("failed to parse codex JSON output: %w", err)
		}

		if event.Type == "item.completed" && event.Item.Type == "agent_message" {
			message = strings.TrimSpace(event.Item.Text)
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read codex output: %w", err)
	}

	return message, nil
}

type codexEvent struct {
	Type string `json:"type"`
	Item struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"item"`
}

func buildBuiltinBackend(name string) (backendConfig, bool) {
	switch normalizeBackendName(name) {
	case "codex":
		return backendConfig{
			binary: "codex",
			args: []string{
				"exec",
				"--ephemeral",
				"--skip-git-repo-check",
				"--json",
				"--sandbox",
				"read-only",
			},
			mode: backendModeCodexJSON,
		}, true
	case "claude":
		return backendConfig{
			binary: "claude",
			args:   []string{"-p"},
			mode:   backendModeStreaming,
		}, true
	case "gemini":
		return backendConfig{
			binary: "gemini",
			args:   []string{"-p"},
			mode:   backendModeStreaming,
		}, true
	default:
		return backendConfig{}, false
	}
}
