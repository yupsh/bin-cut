package main

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	command "github.com/gloo-foo/cmd-cut"
	gloo "github.com/gloo-foo/framework"
	"github.com/spf13/afero"
	"github.com/urfave/cli/v3"
)

const name = "cut"

// Error is the sentinel error type emitted by this package.
type Error string

func (e Error) Error() string { return string(e) }

const (
	// ErrInvalidFields reports a malformed -f/--fields list part.
	ErrInvalidFields Error = "invalid field list"
	// ErrOpenEndedField reports a "-f N-" open-ended-to-end field range. The
	// pinned cmd-cut field API selects fields by explicit 1-based position and
	// cannot express an unbounded upper end; -c/-b accept "N-" since they take
	// a spec string. See cmd-cut COMPATIBILITY.md.
	ErrOpenEndedField Error = "open-ended field range (N-) is not supported; list the fields explicitly or use -c/-b"
)

const (
	flagDelimiter  = "delimiter"
	flagFields     = "fields"
	flagChars      = "characters"
	flagBytes      = "bytes"
	flagComplement = "complement"
)

// usageText is the command's multi-line usage synopsis, shown in --help.
// cli/v3 indents the whole block by 3 spaces, so these lines are flush-left to
// stay aligned in the rendered output.
const usageText = `cut OPTION... [FILE...]

print selected parts of lines from each FILE to standard output.
With no FILE, or when FILE is -, read standard input.`

// init replaces urfave/cli's default --version/-v flag with a --version-only
// flag, freeing the single-letter -v for command flags (e.g. grep -v) while
// still exposing the injected build version.
func init() {
	cli.VersionFlag = &cli.BoolFlag{Name: "version", Usage: "print version information and exit"}
}

// run builds and executes the cut CLI against the injected version, I/O, and
// filesystem, returning the process exit code.
func run(version string, args []string, stdin io.Reader, stdout, stderr io.Writer, fs afero.Fs) int {
	cmd := newApp(version, stdin, stdout, fs)
	cmd.Writer = stdout
	cmd.ErrWriter = stderr
	if err := cmd.Run(context.Background(), args); err != nil {
		_, _ = fmt.Fprintf(stderr, name+": %v\n", err)
		return 1
	}
	return 0
}

func newApp(version string, stdin io.Reader, stdout io.Writer, fs afero.Fs) *cli.Command {
	return &cli.Command{
		Name:            name,
		Version:         version,
		Usage:           "remove sections from each line of files",
		UsageText:       usageText,
		HideHelpCommand: true,
		// Keep exit handling in run() rather than letting urfave/cli call
		// os.Exit, so the exit code stays testable.
		ExitErrHandler: func(context.Context, *cli.Command, error) {},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    flagDelimiter,
				Aliases: []string{"d"},
				Usage:   "use DELIM instead of TAB for field delimiter",
			},
			&cli.StringFlag{
				Name:    flagFields,
				Aliases: []string{"f"},
				Usage:   "select only these fields (comma-separated list)",
			},
			&cli.StringFlag{
				Name:    flagChars,
				Aliases: []string{"c"},
				Usage:   "select only these characters (e.g. 1-3,5)",
			},
			&cli.StringFlag{Name: flagBytes, Aliases: []string{"b"}, Usage: "select only these bytes (e.g. 1-3,5)"},
			&cli.BoolFlag{Name: flagComplement, Usage: "complement the set of selected bytes, characters or fields"},
		},
		Action: action(stdin, stdout, fs),
	}
}

func action(stdin io.Reader, stdout io.Writer, fs afero.Fs) cli.ActionFunc {
	return func(_ context.Context, c *cli.Command) error {
		opts, err := options(c)
		if err != nil {
			return err
		}
		_, err = gloo.Run(source(c, stdin, fs), gloo.ByteWriteTo(stdout), command.Cut(opts...))
		return err
	}
}

func source(c *cli.Command, stdin io.Reader, fs afero.Fs) any {
	if c.NArg() == 0 {
		return gloo.ByteReaderSource([]io.Reader{stdin})
	}
	files := make([]gloo.File, c.NArg())
	for i := range files {
		files[i] = gloo.File(c.Args().Get(i))
	}
	return gloo.ByteFileSource(fs, files)
}

// options translates the selected flags into Cut option values. The byte (-b)
// and character (-c) specs are passed through verbatim, since the command
// package parses their full grammar (including open-ended ranges). Fields (-f)
// must be expanded to explicit 1-based positions here, because the command
// package's field API selects by explicit position.
func options(c *cli.Command) ([]any, error) {
	opts := positionOpts(c)
	if c.Bool(flagComplement) {
		opts = append(opts, command.CutComplement)
	}
	return appendFields(c, opts)
}

// positionOpts collects the delimiter and the verbatim byte/character specs.
func positionOpts(c *cli.Command) []any {
	var opts []any
	if c.IsSet(flagDelimiter) {
		opts = append(opts, command.CutDelimiter(c.String(flagDelimiter)))
	}
	if c.IsSet(flagChars) {
		opts = append(opts, command.CutChars(c.String(flagChars)))
	}
	if c.IsSet(flagBytes) {
		opts = append(opts, command.CutBytes(c.String(flagBytes)))
	}
	return opts
}

// appendFields expands the -f spec into explicit field positions and appends
// the resulting option, or returns an error for a malformed/open-ended spec.
func appendFields(c *cli.Command, opts []any) ([]any, error) {
	if !c.IsSet(flagFields) {
		return opts, nil
	}
	fields, err := parseFields(c.String(flagFields))
	if err != nil {
		return nil, err
	}
	return append(opts, command.CutFields(fields...)), nil
}

// parseFields parses a comma-separated list of 1-based positions and ranges
// ("1,3,5-7", "-3") into explicit field positions. Empty parts are skipped, as
// GNU cut tolerates them. An open-ended "N-" range is rejected (ErrOpenEndedField).
func parseFields(s string) ([]int, error) {
	var result []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		nums, err := parsePart(part)
		if err != nil {
			return nil, err
		}
		result = append(result, nums...)
	}
	return result, nil
}

// parsePart parses one comma-free spec part: "N", "N-M", or "-M".
func parsePart(part string) ([]int, error) {
	start, end, found := strings.Cut(part, "-")
	if !found {
		n, err := atoi(start)
		return []int{n}, err
	}
	return parseRange(start, end)
}

// parseRange expands a "lo-hi" range into explicit positions. A missing lo
// defaults to 1 ("-M"); a missing hi ("N-") is open-ended and unsupported.
func parseRange(start, end string) ([]int, error) {
	if end == "" {
		return nil, ErrOpenEndedField
	}
	lo := 1
	if start != "" {
		parsed, err := atoi(start)
		if err != nil {
			return nil, err
		}
		lo = parsed
	}
	hi, err := atoi(end)
	if err != nil {
		return nil, err
	}
	return rangePositions(lo, hi), nil
}

// rangePositions lists the inclusive positions from lo to hi.
func rangePositions(lo, hi int) []int {
	var result []int
	for i := lo; i <= hi; i++ {
		result = append(result, i)
	}
	return result
}

func atoi(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrInvalidFields, s)
	}
	return n, nil
}
