package cli

import (
	"context"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"orion/agent/internal/config"
	"orion/agent/internal/registration"
	agentstate "orion/agent/internal/state"
	"orion/agent/internal/transport"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type enrollResult struct {
	AgentID string `json:"agent_id"`
	CoreURL string `json:"core_url"`
	Config  string `json:"config"`
	State   string `json:"state"`
}

func newEnrollCommand(ctx context.Context, opts *Options) *cobra.Command {
	var coreURL, token, tokenFile string
	cmd := &cobra.Command{
		Use:   "enroll",
		Short: "Register this host with Core using a one-time enrollment token",
		Long: "Writes a starter config if none exists, registers the host with the one-time " +
			"token from Console (Add server), and prepares state for the service user. " +
			"The installer runs this for you.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnroll(ctx, opts, coreURL, token, tokenFile)
		},
	}
	cmd.Flags().StringVar(&coreURL, "core-url", "", "Core URL (or "+config.EnvCoreURL+")")
	cmd.Flags().StringVar(&token, "token", "", "One-time enrollment token (prefer --token-file or "+config.EnvEnrollment+")")
	cmd.Flags().StringVar(&tokenFile, "token-file", "", "File containing the enrollment token")
	return cmd
}

func runEnroll(_ context.Context, opts *Options, coreURL, token, tokenFile string) error {
	if tokenFile != "" {
		data, err := os.ReadFile(tokenFile)
		if err != nil {
			return NewCommandError("could not read token file", err)
		}
		token = strings.TrimSpace(string(data))
	}
	if token == "" {
		var err error
		if token, err = config.Env(config.EnvEnrollment); err != nil {
			return NewCommandError("could not read enrollment token", err)
		}
	}
	if token == "" {
		return NewCommandError("enrollment token is required", errors.New("none given"),
			"copy the command from Console > Add server, or pass --token-file")
	}
	if coreURL == "" {
		coreURL, _ = config.Env(config.EnvCoreURL)
	}
	if coreURL == "" {
		return NewCommandError("Core URL is required", errors.New("none given"), "pass --core-url https://your-core")
	}

	if !opts.JSON {
		PrintHeader("enroll")
		PrintInfo("core_url", coreURL)
	}
	if err := ensureStarterConfig(opts.ConfigPath, coreURL); err != nil {
		return NewCommandError("could not write config", err)
	}
	userConfig, err := config.LoadUserConfig(opts.ConfigPath)
	if err != nil {
		return NewCommandError("config is invalid", err)
	}
	userConfig.CoreURL = coreURL

	store, err := agentstate.Open(opts.StatePath)
	if err != nil {
		return NewCommandError("could not open state database", err)
	}
	defer store.Close()

	prev, hadPrev := os.LookupEnv(config.EnvEnrollment)
	os.Setenv(config.EnvEnrollment, token)
	defer func() {
		if hadPrev {
			os.Setenv(config.EnvEnrollment, prev)
		} else {
			os.Unsetenv(config.EnvEnrollment)
		}
	}()
	if err := registration.New(userConfig, opts.ConfigPath, store).RegisterAgentIfNeeded(); err != nil {
		if errors.Is(err, transport.ErrEnrollmentRejected) {
			return NewCommandError("Core rejected the enrollment token", err)
		}
		return NewCommandError("could not register with Core", err, "check connectivity with: orion-agent doctor")
	}
	st, err := store.Get()
	if err != nil {
		return NewCommandError("could not read state", err)
	}
	handOverToServiceUser(opts.StatePath, opts.ConfigPath)

	res := enrollResult{AgentID: st.AgentID, CoreURL: st.CoreURL, Config: opts.ConfigPath, State: opts.StatePath}
	if opts.JSON {
		return renderJSON(outputWriter, res)
	}
	PrintOK("host enrolled")
	PrintInfo("agent_id", res.AgentID)
	PrintInfo("next", "orion-agent start")
	return nil
}

// ensureStarterConfig writes a minimal config only when none exists.
func ensureStarterConfig(path, coreURL string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	cfg := config.UserConfig{CoreURL: coreURL, Interval: config.DefaultInterval, Monitors: []config.UserMonitor{}}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o640)
}

// handOverToServiceUser gives the unprivileged service user ownership of the
// state files when enrollment ran as root.
func handOverToServiceUser(paths ...string) {
	if os.Geteuid() != 0 {
		return
	}
	u, err := user.Lookup("orion")
	if err != nil {
		return
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)
	for _, p := range paths {
		_ = os.Chown(p, uid, gid)
	}
	if len(paths) > 0 {
		_ = os.Chown(filepath.Dir(paths[0]), uid, gid)
		for _, suffix := range []string{"-wal", "-shm", "-journal"} {
			_ = os.Chown(paths[0]+suffix, uid, gid)
		}
	}
}
