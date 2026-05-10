package appsetup

import (
	"context"
	"encoding/json"
	"fmt"
)

// ghAuthStatus is the JSON shape returned by `gh auth status -a
// --json hosts`. We only need the state field for the github.com
// host. Lifted from the gh CLI's documented schema; future schema
// changes might break parsing.
type ghAuthStatus struct {
	Hosts map[string][]struct {
		State string `json:"state"`
	} `json:"hosts"`
}

// SetupGitHub probes gh auth status and dispatches:
//   - If state == "success" for github.com: return (true, nil). No
//     prompt, no login.
//   - Otherwise: prompter.AskYesNo("Setup Github"). On false, return
//     (false, nil) — user opted out.
//   - On true: invoke `gh auth login -w` interactively (browser flow,
//     user uploads the SSH key from SetupSSH inside the gh flow).
//     After it completes (regardless of internal state), return
//     (true, nil). No re-probe — once the user finishes the login
//     flow we trust the result.
//
// gh missing on PATH propagates as an error. gh is in the brew
// packages list, so this should never happen in practice.
func SetupGitHub(ctx context.Context, prompter YesNoPrompter, runner CmdRunner) (bool, error) {
	output, err := runner.Capture(ctx, "gh", "auth", "status", "-a", "--json", "hosts")
	if err != nil {
		return false, fmt.Errorf("gh auth status: %w", err)
	}

	var status ghAuthStatus
	if err := json.Unmarshal(output, &status); err != nil {
		return false, fmt.Errorf("parse gh auth status JSON: %w", err)
	}

	if hosts, ok := status.Hosts["github.com"]; ok && len(hosts) > 0 && hosts[0].State == "success" {
		return true, nil
	}

	accepted, err := prompter.AskYesNo("Setup Github")
	if err != nil {
		return false, fmt.Errorf("ask yes-no: %w", err)
	}
	if !accepted {
		return false, nil
	}

	if err := runner.Run(ctx, "gh", "auth", "login", "-w"); err != nil {
		return false, fmt.Errorf("gh auth login -w: %w", err)
	}
	return true, nil
}
