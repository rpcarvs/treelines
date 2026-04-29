package cmd

import "github.com/spf13/cobra"

const onboardText = "## Codebase Exploration\n" +
	"\n" +
	"This project can be explored with **treelines** before reading source files.\n" +
	"Run `treelines recap` for a fuller command reference.\n" +
	"\n" +
	"**Agent quick start:**\n" +
	"- `treelines init` - Create `.treelines/` and schema if needed\n" +
	"- `treelines index` - Build a fresh full snapshot\n" +
	"- `treelines overview` - Get the compact first-pass map\n" +
	"- `treelines module-graph` - See module relationships\n" +
	"- `treelines search <name>` - Find symbols by name or FQName\n" +
	"- `treelines callees <fq_name>` - Show what an element calls\n" +
	"- `treelines uses <fq_name>` - Show who calls an element\n" +
	"\n" +
	"**Rules of use:**\n" +
	"- Run `treelines index` before exploration and wait for it to finish\n" +
	"- Prefer `treelines overview` before targeted file reads\n" +
	"- Use direct file reads only after treelines has narrowed the scope\n" +
	"- Run `treelines index` again when a fresh post-edit snapshot is needed\n"

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Show a quick treelines introduction for agents",
	Long:  "Show a short agent-oriented reminder for starting codebase exploration with treelines.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Print(onboardText)
	},
}

func init() {
	rootCmd.AddCommand(onboardCmd)
}
