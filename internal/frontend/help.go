package frontend

import (
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var basicHelpCommandOrder = []string{
	"gentic",
	"exec",
	"desktop",
	"list",
	"profile",
	"runtime",
	"image",
	"storage",
}

var basicHelpExamples = []string{
	"ned profile install --overwrite",
	"ned gentic personal default",
	"ned exec <vm> -- /bin/bash",
	"ned runtime status",
}

func init() {
	rootCmd.SetHelpFunc(nedHelp)
	rootCmd.SetHelpCommand(newHelpCmd())
}

func nedHelp(cmd *cobra.Command, args []string) {
	writeHelp(cmd.OutOrStdout(), cmd, shouldRenderVerboseHelp())
}

func newHelpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "help [command...]",
		Short: "Show help for a command",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := resolveCommandPath(rootCmd, args)
			if err != nil {
				return err
			}
			writeHelp(commandStdout(cmd), target, shouldRenderVerboseHelp())
			return nil
		},
	}
}

func shouldRenderVerboseHelp() bool {
	return !flagBrevity
}

func writeHelp(out io.Writer, cmd *cobra.Command, verbose bool) {
	if verbose {
		fmt.Fprintln(out, "Verbose command tree:")
		fmt.Fprint(out, renderCommandTree(cmd.Root()))
		return
	}
	if isRootCommand(cmd) {
		writeBasicRootHelp(out, cmd.Root())
		return
	}
	fmt.Fprint(out, cmd.UsageString())
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Help:")
	fmt.Fprintf(out, "  Use %s [command] --help without --brevity to show the full command tree with flags and descriptions.\n", cmd.Root().CommandPath())
}

func isRootCommand(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	return cmd == cmd.Root()
}

func writeBasicRootHelp(out io.Writer, root *cobra.Command) {
	if root == nil {
		return
	}
	fmt.Fprintln(out, strings.TrimSpace(root.Short))
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintf(out, "  %s [command]\n", root.CommandPath())
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Common Commands:")
	for _, name := range basicHelpCommandOrder {
		command := findDirectSubcommand(root, name)
		if command == nil {
			continue
		}
		fmt.Fprintf(out, "  %-10s %s\n", command.Name(), commandSummary(command))
	}
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Common Examples:")
	for _, example := range basicHelpExamples {
		fmt.Fprintf(out, "  %s\n", example)
	}
	fmt.Fprintln(out)

	fmt.Fprintln(out, "More Help:")
	fmt.Fprintf(out, "  %s [command] --help\n", root.CommandPath())
}

func findDirectSubcommand(root *cobra.Command, name string) *cobra.Command {
	if root == nil {
		return nil
	}
	for _, command := range root.Commands() {
		if command.Hidden || command.Name() != name {
			continue
		}
		return command
	}
	return nil
}

func commandSummary(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}
	summary := strings.TrimSpace(cmd.Short)
	if summary != "" {
		return summary
	}
	long := strings.TrimSpace(cmd.Long)
	if long == "" {
		return ""
	}
	if before, _, ok := strings.Cut(long, "\n"); ok {
		return strings.TrimSpace(before)
	}
	return long
}

func renderCommandTree(root *cobra.Command) string {
	if root == nil {
		return ""
	}
	var builder strings.Builder
	writeCommandTreeNode(&builder, root, 0)
	globalFlags := commandPersistentFlags(root)
	if len(globalFlags) > 0 {
		builder.WriteString("Global flags:\n")
		for _, flag := range globalFlags {
			builder.WriteString(fmt.Sprintf("  %s\n", flag))
		}
	}
	if len(root.Commands()) > 0 {
		builder.WriteString("\n")
	}
	for _, child := range root.Commands() {
		writeCommandTree(&builder, child, 1)
	}
	return builder.String()
}

func writeCommandTree(builder *strings.Builder, cmd *cobra.Command, depth int) {
	if cmd.Hidden {
		return
	}
	writeCommandTreeNode(builder, cmd, depth)
	for _, child := range cmd.Commands() {
		writeCommandTree(builder, child, depth+1)
	}
}

func writeCommandTreeNode(builder *strings.Builder, cmd *cobra.Command, depth int) {
	indent := strings.Repeat("  ", depth)
	line := fmt.Sprintf("%s%s", indent, cmd.Use)
	if cmd.Short != "" {
		line = fmt.Sprintf("%s — %s", line, cmd.Short)
	}
	builder.WriteString(line)
	builder.WriteString("\n")

	flags := commandUniqueFlags(cmd)
	if len(flags) > 0 {
		for _, flag := range flags {
			builder.WriteString(fmt.Sprintf("%s  %s\n", indent, flag))
		}
	}
}

func commandUniqueFlags(cmd *cobra.Command) []string {
	if cmd == nil {
		return nil
	}
	if cmd.Parent() == nil {
		return visibleFlags(cmd.LocalNonPersistentFlags())
	}
	return visibleFlags(cmd.NonInheritedFlags())
}

func commandPersistentFlags(cmd *cobra.Command) []string {
	if cmd == nil {
		return nil
	}
	return visibleFlags(cmd.PersistentFlags())
}

func visibleFlags(flagSet *pflag.FlagSet) []string {
	if flagSet == nil {
		return nil
	}
	type flagEntry struct {
		key  string
		text string
	}
	flags := make([]flagEntry, 0)
	flagSet.VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden || flag.Name == "help" {
			return
		}
		flags = append(flags, flagEntry{
			key:  flag.Name,
			text: formatFlag(flag),
		})
	})
	sort.Slice(flags, func(i, j int) bool {
		return flags[i].key < flags[j].key
	})
	labels := make([]string, 0, len(flags))
	for _, entry := range flags {
		labels = append(labels, entry.text)
	}
	return labels
}

func formatFlag(flag *pflag.Flag) string {
	usage := strings.TrimSpace(flag.Usage)
	if flag.Name == "rm-container-on-exit" {
		if usage == "" {
			return "-rm, --rm-container-on-exit"
		}
		return fmt.Sprintf("-rm, --rm-container-on-exit: %s", usage)
	}
	if flag.Shorthand != "" {
		if usage == "" {
			return fmt.Sprintf("-%s, --%s", flag.Shorthand, flag.Name)
		}
		return fmt.Sprintf("-%s, --%s: %s", flag.Shorthand, flag.Name, usage)
	}
	if usage == "" {
		return fmt.Sprintf("--%s", flag.Name)
	}
	return fmt.Sprintf("--%s: %s", flag.Name, usage)
}

func resolveCommandPath(root *cobra.Command, args []string) (*cobra.Command, error) {
	current := root
	for _, arg := range args {
		next := findNamedSubcommand(current, arg)
		if next == nil {
			return nil, fmt.Errorf("unknown help topic %q under %s", arg, current.CommandPath())
		}
		current = next
	}
	return current, nil
}

func findNamedSubcommand(cmd *cobra.Command, name string) *cobra.Command {
	for _, child := range cmd.Commands() {
		if child.Name() == name || slices.Contains(child.Aliases, name) {
			return child
		}
	}
	return nil
}
