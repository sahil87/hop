package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/config"
	"github.com/sahil87/hop/internal/proc"
	"github.com/sahil87/hop/internal/scan"
	"github.com/sahil87/hop/internal/yamled"
)

// addLong is the cobra Long help for `hop add <dir>` and its hidden alias
// `hop config add <dir>`.
const addLong = `Register on-disk repos into hop.yaml.

By default, classifies just <dir> and — when it is a normal git repo with a
remote — merges its URL into hop.yaml using the group convention (convention
layout → the 'default' group; otherwise an invented group keyed off the parent
dir basename). add writes by default.

With -r/--recursive, walks <dir> for git repos (DFS, depth-bounded via --depth,
symlink-following) and registers every one it finds. With -p/--print, renders
the merge plan to stdout instead of writing (a dry-run, valid at both breadths).
With -g/--group <name>, forces all discovered repos into the named group,
auto-creating it if absent.

A non-git directory is a no-op (a clear message, exit 0), not an error.

Examples:
  hop add ~/code/acme/widget    register one existing repo into hop.yaml
  hop add -r ~/code             walk ~/code and register every repo found
  hop add -r -p ~/code          preview the recursive plan without writing
  hop add -g vendor ~/forks/x   register into a forced (auto-created) group`

// addOpts carries the parsed flag values into runAdd, keeping the function
// signature stable as the flag surface grows.
type addOpts struct {
	recursive bool
	print     bool
	depth     int
	group     string
}

// newAddCmd returns the cobra factory for the canonical top-level `hop add <dir>`.
func newAddCmd() *cobra.Command {
	var opts addOpts
	cmd := &cobra.Command{
		Use:   "add <dir>",
		Short: "register on-disk repos into hop.yaml (single dir, or -r to walk a tree)",
		Long:  addLong,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd, "hop add", args[0], opts)
		},
	}
	bindAddFlags(cmd, &opts)
	return cmd
}

// newConfigAddCmd returns the cobra factory for the hidden alias
// `hop config add <dir>`. It shares runAdd with the canonical top-level command
// but is Hidden (disappears from --help and self-filters from help-dump) and
// keeps emitting its historical "hop config add:" stderr prefix.
func newConfigAddCmd() *cobra.Command {
	var opts addOpts
	cmd := &cobra.Command{
		Use:    "add <dir>",
		Short:  "register on-disk repos into hop.yaml (single dir, or -r to walk a tree)",
		Long:   addLong,
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd, "hop config add", args[0], opts)
		},
	}
	bindAddFlags(cmd, &opts)
	return cmd
}

// bindAddFlags wires the shared -r/-p/--depth/-g flag set onto an add command.
// Both the canonical command and the hidden alias expose the identical surface.
func bindAddFlags(cmd *cobra.Command, opts *addOpts) {
	cmd.Flags().BoolVarP(&opts.recursive, "recursive", "r", false, "walk <dir> for git repos (DFS) instead of classifying just <dir>")
	cmd.Flags().BoolVarP(&opts.print, "print", "p", false, "render the merge plan to stdout instead of writing to hop.yaml (a dry-run)")
	cmd.Flags().IntVar(&opts.depth, "depth", 3, "maximum DFS depth (only meaningful with -r; root counts as depth 0; must be >= 1)")
	cmd.Flags().StringVarP(&opts.group, "group", "g", "", "force all discovered repos into the named group, auto-creating it if absent")
}

// minAddDepth is the smallest valid value for --depth (matches scan's
// historical minimum).
const minAddDepth = 1

// runAdd validates <dir>, discovers repos (single-dir or recursive), resolves
// hop.yaml, and either writes (default) or prints (-p) the merge plan. cmdName
// is the per-path stderr prefix ("hop add" for the canonical top-level command,
// "hop config add" for the hidden alias). Returns errSilent / *errExitCode on
// user-visible failures; nil on success and on every forgiving no-op (non-git
// dir, worktree/bare skip, already-registered).
func runAdd(cmd *cobra.Command, cmdName, userArg string, opts addOpts) error {
	stderr := cmd.ErrOrStderr()

	// 1. Validate --depth (only meaningful with -r, but validated whenever
	//    supplied so a bad value never silently no-ops). Mirrors scan's gate.
	if opts.depth < minAddDepth {
		fmt.Fprintf(stderr, "%s: --depth must be >= %d.\n", cmdName, minAddDepth)
		return &errExitCode{code: 2}
	}

	// 2. Validate <dir>: filepath.Abs → EvalSymlinks → os.Stat (directory).
	canonicalDir, ok := validateConfigDir(userArg, cmdName, stderr)
	if !ok {
		return &errExitCode{code: 2}
	}

	// 3. Resolve hop.yaml. Print mode (-p) never touches the file, so it keeps
	//    erroring on a missing config (config.Resolve). Write mode (default)
	//    carries the user's intent, so a missing config is auto-bootstrapped
	//    with a minimal skeleton (the only ResolveWriteTarget error is
	//    $HOME-unset, an environment failure).
	configPath, err := resolveAddConfig(stderr, cmdName, opts.print)
	if err != nil {
		return err
	}

	// 4. Load existing config (used for the convention check + dedup, and for
	//    deciding whether -g's group already exists).
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmdName, err)
		return errSilent
	}

	// 5. Discover repos. -r walks the tree; otherwise classify just <dir>.
	found, skips, err := discoverRepos(canonicalDir, opts, stderr, cmdName, userArg)
	if err != nil {
		return err
	}

	// 6. Build the merge plan. -g forces all repos into the named group
	//    (auto-created in write mode); otherwise the shared convention/invented
	//    logic applies. The summary carries the counters for the stderr block.
	plan, planSummary, err := buildAddPlan(cfg, found, opts, configPath, userArg, stderr, cmdName)
	if err != nil {
		return err
	}

	// 7. Render (print) or write (default).
	if opts.print {
		bytes, err := yamled.RenderScan(configPath, plan)
		if err != nil {
			fmt.Fprintf(stderr, "%s: render: %v\n", cmdName, err)
			return errSilent
		}
		fmt.Fprint(cmd.OutOrStdout(), addPrintHeader(userArg, configPath))
		_, _ = cmd.OutOrStdout().Write(bytes)
	} else if !planIsEmpty(plan) {
		if err := yamled.MergeScan(configPath, plan); err != nil {
			fmt.Fprintf(stderr, "%s: write %s: %v\n", cmdName, configPath, err)
			return errSilent
		}
	}

	// 8. Stderr summary (last, after stdout in print mode).
	emitAddSummary(stderr, cmdName, userArg, opts, found, skips, planSummary, configPath)
	return nil
}

// resolveAddConfig resolves hop.yaml for the chosen sink. Print mode resolves
// read-only via config.Resolve (errors on absence — nothing to bootstrap).
// Write mode resolves the write target and auto-inits a missing skeleton,
// announcing `created: <path>`.
func resolveAddConfig(stderr io.Writer, cmdName string, print bool) (string, error) {
	if print {
		path, err := config.Resolve()
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", cmdName, err)
			return "", errSilent
		}
		return path, nil
	}

	path, err := config.ResolveWriteTarget()
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmdName, err)
		return "", errSilent
	}
	created, err := config.EnsureSkeleton(path)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmdName, err)
		return "", errSilent
	}
	if created {
		fmt.Fprintf(stderr, "created: %s\n", path)
	}
	return path, nil
}

// discoverRepos returns the Found/Skip slices for the chosen breadth. Recursive
// mode delegates to scan.Walk (depth-bounded DFS); single-dir mode classifies
// just canonicalDir via scan.ClassifyOne and, for a non-registrable dir, emits
// the forgiving no-op message and returns empty slices. A fatal git failure is
// surfaced as errSilent (with the git-missing hint when applicable).
func discoverRepos(canonicalDir string, opts addOpts, stderr io.Writer, cmdName, userArg string) ([]scan.Found, []scan.Skip, error) {
	ctx := context.Background()
	scanOpts := scan.Options{Depth: opts.depth, GitRunner: gitRunner}

	if opts.recursive {
		found, skips, err := scan.Walk(ctx, canonicalDir, scanOpts)
		if err != nil {
			return nil, nil, addGitError(stderr, cmdName, err)
		}
		return found, skips, nil
	}

	found, skipReason, isRepo, err := scan.ClassifyOne(ctx, canonicalDir, scanOpts)
	if err != nil {
		return nil, nil, addGitError(stderr, cmdName, err)
	}
	if !isRepo {
		// Forgiving: plain dir (no skip reason) or a worktree/bare/no-remote
		// candidate. Message + exit 0. No repos discovered.
		fmt.Fprintln(stderr, addSkipMessage(cmdName, userArg, skipReason))
		return nil, nil, nil
	}
	return []scan.Found{found}, nil, nil
}

// addGitError maps a discovery error to the right stderr line + errSilent: a
// missing-git failure surfaces the shared gitMissingHint; anything else is
// reported under the command prefix.
func addGitError(stderr io.Writer, cmdName string, err error) error {
	if errors.Is(err, proc.ErrNotFound) {
		fmt.Fprintln(stderr, gitMissingHint)
		return errSilent
	}
	fmt.Fprintf(stderr, "%s: %v\n", cmdName, err)
	return errSilent
}

// buildAddPlan converts the discovered repos into a yamled.ScanPlan. With
// -g <name>, every repo is forced into the named group (auto-created via
// EnsureGroup in write mode, announcing `created group:` on first creation);
// otherwise the shared buildScanPlan convention/invented logic applies. For the
// single-dir, non-forced, non-recursive case it preserves the historical
// explicit "already registered" vs. "could not be registered" distinction.
func buildAddPlan(cfg *config.Config, found []scan.Found, opts addOpts, configPath, userArg string, stderr io.Writer, cmdName string) (yamled.ScanPlan, scanPlanSummary, error) {
	if opts.group != "" {
		plan, summary, err := buildForcedGroupPlan(cfg, found, opts, configPath, stderr, cmdName)
		return plan, summary, err
	}

	// Single-dir, non-recursive add preserves the explicit idempotency
	// reporting: a genuine URL duplicate says "already registered"; a candidate
	// skipped for a non-dedup reason says "could not be registered".
	if !opts.recursive && len(found) == 1 {
		if urlAlreadyRegistered(cfg, found[0].URL) {
			fmt.Fprintf(stderr, "%s: %s already registered in %s. Nothing to add.\n", cmdName, found[0].URL, configPath)
			return yamled.ScanPlan{}, scanPlanSummary{}, nil
		}
		plan, summary := buildScanPlan(cfg, found, stderr)
		if planIsEmpty(plan) {
			fmt.Fprintf(stderr, "%s: '%s' could not be registered (see skip above). Nothing to add.\n", cmdName, userArg)
			return yamled.ScanPlan{}, scanPlanSummary{}, nil
		}
		return plan, summary, nil
	}

	plan, summary := buildScanPlan(cfg, found, stderr)
	return plan, summary, nil
}

// buildForcedGroupPlan assigns every discovered repo to opts.group, bypassing
// the convention/invented logic entirely. URLs already present anywhere in
// hop.yaml are dropped (and counted as already-registered skips) so the plan
// and summary agree with what MergeScan would actually change. In write mode a
// missing group is created via yamled.EnsureGroup (announced once) only when
// there is at least one new URL to append — a discovery that yields nothing new
// leaves hop.yaml untouched and prints no `created group:` line.
func buildForcedGroupPlan(cfg *config.Config, found []scan.Found, opts addOpts, configPath string, stderr io.Writer, cmdName string) (yamled.ScanPlan, scanPlanSummary, error) {
	existingURLs := make(map[string]struct{})
	groupExists := false
	for _, g := range cfg.Groups {
		if g.Name == opts.group {
			groupExists = true
		}
		for _, u := range g.URLs {
			existingURLs[u] = struct{}{}
		}
	}

	var urls []string
	var summary scanPlanSummary
	for _, f := range found {
		if _, dup := existingURLs[f.URL]; dup {
			fmt.Fprintf(stderr, "skip: %s: %s already registered in hop.yaml\n", f.Path, f.URL)
			summary.skipAlreadyRegistered++
			continue
		}
		urls = append(urls, f.URL)
	}

	plan := yamled.ScanPlan{}
	if len(urls) > 0 {
		// Auto-create the group only when we are actually writing, it does not
		// yet exist, and there is at least one new URL to append. Gating on
		// len(urls) avoids materializing an empty group (and announcing
		// `created group:`) when discovery found nothing new — the target dir
		// isn't a repo, the recursive walk found zero repos, or everything is
		// already registered. Print mode never mutates the file. EnsureGroup is
		// idempotent, but we also gate on groupExists so the announcement only
		// fires when we genuinely created it.
		if !opts.print && !groupExists {
			if err := yamled.EnsureGroup(configPath, opts.group); err != nil {
				fmt.Fprintf(stderr, "%s: %v\n", cmdName, err)
				return yamled.ScanPlan{}, scanPlanSummary{}, errSilent
			}
			fmt.Fprintf(stderr, "created group: %s\n", opts.group)
		}

		// A forced group with an explicit user-named target renders as a
		// flat-list group with no dir override (Flat). This keeps write mode
		// (where EnsureGroup pre-seeds a flat node) and print mode (where it does
		// not) byte-identical: RenderScan creates the same flat `<name>: [...]`
		// shape either way, instead of a map-shaped `dir: ""` that fails to load.
		plan.InventedGroups = []yamled.InventedGroup{{Name: opts.group, Flat: true, URLs: urls}}
		summary.forcedGroup = opts.group
		summary.inventedURLCount = len(urls)
	}
	return plan, summary, nil
}

// addPrintHeader returns the two-line print-mode header. Reworded from scan's
// --write phrasing to the new -p spelling (intake §3). Date is UTC.
func addPrintHeader(userArg, configPath string) string {
	date := time.Now().UTC().Format("2006-01-02")
	return fmt.Sprintf("# hop config — generated by 'hop add -r -p %s' on %s (UTC).\n# Run without --print to merge into %s.\n",
		userArg, date, configPath)
}

// emitAddSummary writes the stderr status block. On a real write it ends with
// `added:`/`wrote:`; in print mode it ends with the `Run without --print ...`
// trailer. A no-op (empty plan, write mode) emits only the breadth-appropriate
// "nothing to add" context already surfaced by discovery/plan building, so the
// summary stays quiet to avoid double-reporting the single-dir messages.
func emitAddSummary(stderr io.Writer, cmdName, userArg string, opts addOpts, found []scan.Found, skips []scan.Skip, summary scanPlanSummary, configPath string) {
	// The single-dir, non-recursive, non-print path already printed its own
	// no-op / added lines inline (preserving historical wording), so emit the
	// detailed scan-style summary only for recursive or print breadths.
	if !opts.recursive && !opts.print && opts.group == "" {
		if !summaryAddedNothing(summary) {
			fmt.Fprintf(stderr, "added: %s\nwrote: %s\n", found[0].URL, configPath)
		}
		return
	}

	totalFound := len(found)
	if totalFound == 0 {
		fmt.Fprintf(stderr, "%s: scanned %s, found 0 repos. Nothing to add.\n", cmdName, userArg)
		if opts.print {
			fmt.Fprintf(stderr, "Run without --print to merge into %s.\n", configPath)
		} else {
			fmt.Fprintf(stderr, "wrote: %s\n", configPath)
		}
		return
	}

	fmt.Fprintf(stderr, "%s: scanned %s, found %d %s.\n",
		cmdName, userArg, totalFound, pluralize(totalFound, "repo", "repos"))

	if summary.defaultMatched > 0 {
		fmt.Fprintf(stderr, "  matched convention (default): %d (%d new, %d already registered)\n",
			summary.defaultMatched, summary.defaultNew, summary.defaultExisting)
	}
	if summary.forcedGroup != "" {
		// -g names the group explicitly — it was forced, not invented.
		fmt.Fprintf(stderr, "  forced group: %s (%d new)\n", summary.forcedGroup, summary.inventedURLCount)
	}
	if len(summary.inventedGroups) > 0 {
		fmt.Fprintf(stderr, "  invented groups: %d (%s)\n",
			len(summary.inventedGroups), strings.Join(summary.inventedGroups, ", "))
	}
	skipParts := buildSkipParts(skips, summary.skipNoGroupName, summary.skipAlreadyRegistered)
	if len(skipParts) > 0 {
		fmt.Fprintf(stderr, "  skipped: %s\n", strings.Join(skipParts, ", "))
	}

	if opts.print {
		fmt.Fprintf(stderr, "Run without --print to merge into %s.\n", configPath)
	} else {
		fmt.Fprintf(stderr, "wrote: %s\n", configPath)
	}
}

// summaryAddedNothing reports whether a scan/forced-group plan added no URLs
// (no new convention matches and no invented/forced-group URLs). Used only to
// decide the single-dir non-recursive added/wrote line.
func summaryAddedNothing(summary scanPlanSummary) bool {
	return summary.defaultNew == 0 && summary.inventedURLCount == 0
}

// addSkipMessage returns the forgiving stderr line for a dir that is not a
// registrable normal repo. cmdName is the per-path prefix. A plain directory
// (empty skipReason) gets the explicit "not a git repo" wording; classified
// candidates (worktree, bare repo, no remote) report their reason.
func addSkipMessage(cmdName, userArg, skipReason string) string {
	if skipReason == "" {
		return fmt.Sprintf("%s: '%s' is not a git repo. Nothing to add.", cmdName, userArg)
	}
	return fmt.Sprintf("%s: '%s' is a %s — skipping. Nothing to add.", cmdName, userArg, skipReason)
}

// urlAlreadyRegistered reports whether url is already present in any group's URL
// list. Exact-match (no normalization), mirroring buildScanPlan's existingURLs
// dedup semantics so add and scan stay consistent. Used by runAdd to decide the
// "already registered" message before building the plan.
func urlAlreadyRegistered(cfg *config.Config, url string) bool {
	for _, g := range cfg.Groups {
		for _, u := range g.URLs {
			if u == url {
				return true
			}
		}
	}
	return false
}

// planIsEmpty reports whether a ScanPlan would add no URLs.
func planIsEmpty(plan yamled.ScanPlan) bool {
	if len(plan.DefaultURLs) > 0 {
		return false
	}
	for _, ig := range plan.InventedGroups {
		if len(ig.URLs) > 0 {
			return false
		}
	}
	return true
}
