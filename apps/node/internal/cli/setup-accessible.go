package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/term"

	"orion/node/internal/config"
	"orion/node/internal/discover"
)

// accessibleRequested says whether prompts should be plain lines: ACCESSIBLE=1
// (or ORION_ACCESSIBLE) for screen readers and dumb terminals.
func accessibleRequested() bool {
	if os.Getenv("ACCESSIBLE") != "" || os.Getenv("ORION_ACCESSIBLE") != "" {
		return true
	}
	// A dumb terminal only asks for line prompts when a person is typing; a
	// cron job or CI step that sets TERM=dumb must keep running from flags.
	return os.Getenv("TERM") == "dumb" && stdinIsTerminal()
}

// readSecretLine reads a hidden line from a terminal, or a plain line when
// stdin is a pipe (scripts feed answers that way).
func readSecretLine(in *os.File, r *bufio.Reader) (string, error) {
	if term.IsTerminal(in.Fd()) {
		b, err := term.ReadPassword(in.Fd())
		fmt.Fprintln(outputWriter)
		return strings.TrimSpace(string(b)), err
	}
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// accessibleSetup asks the setup questions as plain lines: no colour, no
// cursor movement, one question at a time.
func accessibleSetup(ctx context.Context, opts *Options, in *os.File, out io.Writer) (setupPlan, error) {
	r := bufio.NewReader(in)
	plan := setupPlan{Force: opts.SetupForce}
	ask := func(prompt string) (string, error) {
		fmt.Fprint(out, prompt)
		line, err := r.ReadString('\n')
		if err != nil && line == "" {
			return "", errors.New("no answer on standard input")
		}
		return strings.TrimSpace(line), nil
	}
	if cur := currentSetupState(opts.StatePath); cur != nil {
		plan.CoreURL = cur.CoreURL
		fmt.Fprintf(out, "This host is already connected to %s.\n", cur.CoreURL)
	} else {
		def := firstNonEmpty(opts.SetupCoreURL, envOrEmpty(config.EnvCoreURL))
		fmt.Fprintln(out, "Where is your Core? The address you open the Console at.")
		url, err := ask("Input: ")
		if err != nil {
			return plan, err
		}
		if url == "" {
			url = def
		}
		if err := validateCoreURL(url); err != nil {
			return plan, NewCommandError("that is not a Core address", err)
		}
		fmt.Fprint(out, "One-time token (input is hidden): ")
		token, err := readSecretLine(in, r)
		if err != nil || token == "" {
			return plan, NewCommandError("a one-time token is required", errors.New("create one in the Console under Servers, Add server"))
		}
		conn, err := checkSetupConnection(ctx, strings.TrimSpace(url), token)
		if err != nil {
			return plan, err
		}
		fmt.Fprintf(out, "Reachable. %d ms.\n", conn.Latency.Milliseconds())
		path, err := stageEnrollmentToken(token)
		if err != nil {
			return plan, err
		}
		plan.CoreURL, plan.TokenFile, plan.RemoveTokenFile = strings.TrimSpace(url), path, true
	}

	found := discoverHost(ctx)
	plan.Found = &found
	var picks []discover.Candidate
	for _, c := range found.Candidates {
		if c.Status != discover.StatusMonitored && (c.Monitor != nil || c.StoragePath != "") {
			picks = append(picks, c)
		}
	}
	if len(picks) == 0 {
		fmt.Fprintln(out, "Nothing to suggest. Host metrics report without any setup.")
		return plan, nil
	}
	fmt.Fprintln(out, "\nWhich of these should Orion watch? Enter numbers separated by commas, all, or none.")
	for i, c := range picks {
		fmt.Fprintf(out, "  %d. %s (%s)\n", i+1, c.Name, c.Confidence)
	}
	answer, err := ask("Choice: ")
	if err != nil {
		return plan, err
	}
	var ids []string
	switch strings.ToLower(answer) {
	case "none", "":
	case "all":
		for _, c := range picks {
			ids = append(ids, c.ID)
		}
	default:
		for _, f := range strings.Split(answer, ",") {
			n, err := strconv.Atoi(strings.TrimSpace(f))
			if err != nil || n < 1 || n > len(picks) {
				return plan, NewCommandError("that is not a choice", fmt.Errorf("%q: enter numbers from 1 to %d", strings.TrimSpace(f), len(picks)))
			}
			ids = append(ids, picks[n-1].ID)
		}
	}
	sel, err := discover.Select(found.Candidates, ids)
	if err != nil {
		return plan, NewCommandError("could not select monitors", err)
	}
	plan.Selected = sel
	plan.Label = "chosen"
	return plan, nil
}
