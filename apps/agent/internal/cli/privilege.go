package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const privilegeEscalatedEnv = "ORION_AGENT_PRIVILEGE_ESCALATED"

// EnsureCommandPrivilege re-executes interactive installed-service commands via
// sudo when they need access to service management, installed state, or binary
// replacement paths.
func EnsureCommandPrivilege(command string, args []string) error {
	if !commandNeedsElevation(command, args) || os.Geteuid() == 0 {
		return nil
	}
	if os.Getenv(privilegeEscalatedEnv) == "1" {
		return nil
	}

	sudoPath, err := exec.LookPath("sudo")
	if err != nil {
		return fmt.Errorf("%s requires elevated privileges, but sudo was not found", command)
	}

	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable for privilege escalation: %w", err)
	}
	if resolvedPath, err := filepath.EvalSymlinks(executablePath); err == nil {
		executablePath = resolvedPath
	}

	PrintStep("elevating privileges")
	PrintInfo("command", command)

	sudoArgs := append([]string{executablePath, command}, args...)
	cmd := exec.Command(sudoPath, sudoArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), privilegeEscalatedEnv+"=1")

	err = cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); ok {
		os.Exit(exitErr.ExitCode())
	}
	if err != nil {
		return fmt.Errorf("elevate %s with sudo: %w", command, err)
	}
	os.Exit(0)
	return nil
}

func commandNeedsElevation(command string, args []string) bool {
	switch command {
	case "start", "stop", "restart", "update":
		return true
	case "reconfigure":
		return !argsSetPath(args, "state")
	case "setup", "enroll":
		return !pathWritable(argValue(args, "config", defaultConfigPath()))
	case "maintenance", "state", "token":
		return !argsSetPath(args, "state")
	case "run":
		return argsContainFlag(args, "once") && (!argsSetPath(args, "config") || !argsSetPath(args, "state"))
	default:
		return false
	}
}

func argsContainFlag(args []string, name string) bool {
	short := "-" + name
	long := "--" + name
	for _, arg := range args {
		if arg == short || arg == long || strings.HasPrefix(arg, short+"=") || strings.HasPrefix(arg, long+"=") {
			return true
		}
	}
	return false
}

func argsSetPath(args []string, name string) bool {
	short := "-" + name
	long := "--" + name
	for i, arg := range args {
		if arg == short || arg == long {
			return i+1 < len(args)
		}
		if strings.HasPrefix(arg, short+"=") || strings.HasPrefix(arg, long+"=") {
			return true
		}
	}
	return false
}

// argValue returns the value of --name from args, or def.
func argValue(args []string, name, def string) string {
	short, long := "-"+name, "--"+name
	for i, arg := range args {
		if (arg == short || arg == long) && i+1 < len(args) {
			return args[i+1]
		}
		for _, p := range []string{short + "=", long + "="} {
			if strings.HasPrefix(arg, p) {
				return strings.TrimPrefix(arg, p)
			}
		}
	}
	return def
}

// pathWritable reports whether the current user can write the file, or create
// it in its directory.
func pathWritable(path string) bool {
	if f, err := os.OpenFile(path, os.O_WRONLY, 0); err == nil {
		f.Close()
		return true
	}
	if _, err := os.Stat(path); err == nil {
		return false
	}
	dir := filepath.Dir(path)
	for {
		if _, err := os.Stat(dir); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return false
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
	tmp, err := os.CreateTemp(dir, ".orion-write-test-*")
	if err != nil {
		return false
	}
	tmp.Close()
	os.Remove(tmp.Name())
	return true
}
