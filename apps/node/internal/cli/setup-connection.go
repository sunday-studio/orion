package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"orion/node/internal/config"
	nodestate "orion/node/internal/state"
	"orion/node/internal/transport"
)

type setupConnection struct {
	Latency time.Duration
	Current bool
}

func currentSetupState(path string) *config.InternalState {
	st, err := nodestate.InspectReadOnly(path)
	if err != nil || st == nil || !st.IsRegistered() {
		return nil
	}
	return st
}

func checkCurrentSetupConnection(ctx context.Context, st *config.InternalState, coreURL string) (setupConnection, error) {
	if strings.TrimRight(coreURL, "/") != strings.TrimRight(st.CoreURL, "/") {
		return setupConnection{}, NewCommandError(
			"this host is already connected to another Core",
			fmt.Errorf("registered with %s", st.CoreURL),
			"use the current Core URL, or deliberately reset registration with orion-node reconfigure",
		)
	}
	started := time.Now()
	code, err := probeToken(ctx, st.CoreURL, st.NodeID, st.Token)
	if err != nil {
		return setupConnection{}, NewCommandError("could not connect to Core", err, "check the Core URL and network connection")
	}
	if code == 401 {
		return setupConnection{}, NewCommandError(
			"Core rejected this host's saved token",
			errors.New("the local registration needs a replacement Server token"),
			"in Console, open this Server, rotate or reissue its token, then run: orion-node token apply --token-file <file>",
		)
	}
	if code < 200 || code >= 300 {
		return setupConnection{}, NewCommandError("Core could not validate this host", fmt.Errorf("HTTP %d", code), "check Core health, then try again")
	}
	return setupConnection{Latency: time.Since(started), Current: true}, nil
}

func checkSetupConnection(ctx context.Context, coreURL, token string) (setupConnection, error) {
	if err := validateCoreURL(coreURL); err != nil {
		return setupConnection{}, err
	}
	if looksLikeNodeToken(token) {
		return setupConnection{}, NewCommandError(
			"this is a Server replacement token, not an Add server token",
			errors.New("replacement tokens update an existing local registration"),
			"run: orion-node token apply --token-file <file>",
			"for a new setup, create a token in Console under Servers, Add server",
		)
	}
	client := transport.NewClient(strings.TrimRight(coreURL, "/"), "").WithContext(ctx)
	client.SetEnrollmentToken(token)
	started := time.Now()
	if err := client.PreflightEnrollment(); err != nil {
		if errors.Is(err, transport.ErrEnrollmentRejected) {
			return setupConnection{}, NewCommandError(
				"Core rejected the Add server token",
				err,
				"create a fresh token in Console under Servers, Add server",
			)
		}
		return setupConnection{}, NewCommandError("could not connect to Core", err, "check the Core URL and network connection")
	}
	return setupConnection{Latency: time.Since(started)}, nil
}

func looksLikeNodeToken(token string) bool {
	token = strings.TrimSpace(token)
	if len(token) != 64 {
		return false
	}
	for _, r := range token {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return false
		}
	}
	return true
}

func setupConnectionSummary(result setupConnection) string {
	return fmt.Sprintf("Core reachable %s   %s", setupConnectionLatency(result), setupConnectionTokenLabel(result))
}

func setupConnectionLatency(result setupConnection) time.Duration {
	latency := result.Latency.Round(time.Millisecond)
	if latency < time.Millisecond {
		return time.Millisecond
	}
	return latency
}

func setupConnectionTokenLabel(result setupConnection) string {
	if result.Current {
		return "Server token valid"
	}
	return "Add server token valid"
}
