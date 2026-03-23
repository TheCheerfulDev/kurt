package cmd

import (
	"bytes"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for kurt.

To load completions:

Bash:
  $ source <(kurt completion bash)
  # To load completions for each session, execute once:
  # Linux:
  $ kurt completion bash > /etc/bash_completion.d/kurt
  # macOS:
  $ kurt completion bash > $(brew --prefix)/etc/bash_completion.d/kurt

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc
  # To load completions for each session, execute once:
  $ kurt completion zsh > "${fpath[1]}/_kurt"
  # You will need to start a new shell for this setup to take effect.

Fish:
  $ kurt completion fish | source
  # To load completions for each session, execute once:
  $ kurt completion fish > ~/.config/fish/completions/kurt.fish

PowerShell:
  PS> kurt completion powershell | Out-String | Invoke-Expression
  # To load completions for every new session, run:
  PS> kurt completion powershell > kurt.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			genBashCompletionPatched(rootCmd)
		case "zsh":
			rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}

// genBashCompletionPatched generates bash completion V2 with a self-contained
// __kurt_init_completion function that does not depend on the bash-completion
// package (_get_comp_words_by_ref). This allows tab completion to work on
// systems without bash-completion installed.
func genBashCompletionPatched(cmd *cobra.Command) {
	var buf bytes.Buffer
	cmd.GenBashCompletionV2(&buf, true)

	// Cobra's V2 __<prog>_init_completion still calls _get_comp_words_by_ref
	// which requires the bash-completion package. Replace it with a pure-bash
	// implementation that parses COMP_WORDS/COMP_CWORD directly.
	old := `__kurt_init_completion()
{
    COMPREPLY=()
    _get_comp_words_by_ref "$@" cur prev words cword
}`

	replacement := `__kurt_init_completion()
{
    COMPREPLY=()
    if declare -F _get_comp_words_by_ref >/dev/null 2>&1; then
        _get_comp_words_by_ref "$@" cur prev words cword
    else
        cur="${COMP_WORDS[COMP_CWORD]}"
        prev="${COMP_WORDS[COMP_CWORD-1]}"
        words=("${COMP_WORDS[@]}")
        cword=$COMP_CWORD
    fi
}`

	patched := strings.Replace(buf.String(), old, replacement, 1)
	os.Stdout.WriteString(patched)
}
