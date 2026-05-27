// Package packages defines the curated set of homebrew formulae and
// casks installed during bootstrap, plus the orchestrator that drives
// them through a brew.Runner with limp-and-report semantics.
package packages

// Formulae is the core set of CLI tools installed on every machine.
var Formulae = []string{
	"gnupg",
	"diff-so-fancy",
	"emacs",
	"pinentry-mac",
	"jq",
	"less",
	"grep",
	"zsh-syntax-highlighting",
	"shellcheck",
	"shfmt",
	"lsd",
	"gh",
}

// Fonts is the set of font formulae. Separated for grouping in the
// progress output, not because they install differently.
var Fonts = []string{
	"font-fira-code-nerd-font",
}

// DevTools is the set of development-related formulae.
var DevTools = []string{
	"go",
	"golangci-lint",
	"go-task/tap/go-task",
	"nvm",
	"pnpm",
}

// AI is the set of AI-related CLI tools.
var AI = []string{
	"copilot-cli",
	"claude-code",
}

// CommonCasks is the always-installed cask set.
var CommonCasks = []string{
	"zoom",
	"brave-browser",
	"warp",
	"docker",
	"raycast",
	"keka",
	"slack",
	"shottr",
}

// BetaCasks is the always-installed beta-channel cask set.
var BetaCasks = []string{
	"visual-studio-code@insiders",
}

// PersonalCasks is installed only when Opts.Personal is true.
var PersonalCasks = []string{
	"proton-drive",
	"proton-pass",
	"protonvpn",
	"daisydisk",
	"lulu",
	"ente",
	"yaak",
	"discord",
	"enpass",
}
