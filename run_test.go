package main

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

func TestRun(t *testing.T) {
	cases := []struct {
		files      map[string]string
		name       string
		version    string
		stdin      string
		wantOut    string
		wantErrSub string
		args       []string
		wantCode   int
	}{
		{
			name:    "stdin passthrough no flags",
			args:    []string{"cut"},
			stdin:   "alpha\nbeta\n",
			wantOut: "alpha\nbeta\n",
		},
		{
			name:    "fields with delimiter",
			args:    []string{"cut", "-d", ":", "-f", "2"},
			stdin:   "one:two:three\n",
			wantOut: "two\n",
		},
		{
			name:    "fields range and list",
			args:    []string{"cut", "-d", ",", "-f", "1,3-4"},
			stdin:   "a,b,c,d,e\n",
			wantOut: "a,c,d\n",
		},
		{
			name:    "fields bounded from start",
			args:    []string{"cut", "-d", ":", "-f", "-2"},
			stdin:   "one:two:three:four\n",
			wantOut: "one:two\n",
		},
		{
			name:    "characters selection",
			args:    []string{"cut", "-c", "1-5"},
			stdin:   "Hello World\n",
			wantOut: "Hello\n",
		},
		{
			name:    "bytes selection",
			args:    []string{"cut", "-b", "1,3,5"},
			stdin:   "abcdefgh\n",
			wantOut: "ace\n",
		},
		{
			name:    "fields with empty segment",
			args:    []string{"cut", "-d", ",", "-f", "1,,3"},
			stdin:   "a,b,c\n",
			wantOut: "a,c\n",
		},
		{
			name:    "complement fields",
			args:    []string{"cut", "-d", ",", "-f", "2", "--complement"},
			stdin:   "a,b,c\n",
			wantOut: "a,c\n",
		},
		{
			name:    "file source",
			args:    []string{"cut", "-d", ":", "-f", "1", "/in.txt"},
			files:   map[string]string{"/in.txt": "x:y\nz:w\n"},
			wantOut: "x\nz\n",
		},
		{
			name:    "version flag reports injected version",
			version: "1.2.3",
			args:    []string{"cut", "--version"},
			wantOut: "cut version 1.2.3\n",
		},
		{
			name:       "invalid field list errors",
			args:       []string{"cut", "-f", "a"},
			wantCode:   1,
			wantErrSub: "cut: invalid field list: a",
		},
		{
			name:       "invalid field range high errors",
			args:       []string{"cut", "-f", "1-z"},
			wantCode:   1,
			wantErrSub: "cut: invalid field list: z",
		},
		{
			name:       "invalid field range low errors",
			args:       []string{"cut", "-f", "z-3"},
			wantCode:   1,
			wantErrSub: "cut: invalid field list: z",
		},
		{
			name:       "open-ended field range errors",
			args:       []string{"cut", "-d", ":", "-f", "2-"},
			stdin:      "one:two:three\n",
			wantCode:   1,
			wantErrSub: "cut: open-ended field range",
		},
		{
			name:       "unknown flag errors",
			args:       []string{"cut", "--nope"},
			wantCode:   1,
			wantErrSub: "cut:",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			for path, content := range tc.files {
				if err := afero.WriteFile(fs, path, []byte(content), 0o644); err != nil {
					t.Fatalf("write fixture %s: %v", path, err)
				}
			}

			var out, errOut bytes.Buffer
			code := run(tc.version, tc.args, strings.NewReader(tc.stdin), &out, &errOut, fs)

			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d (stderr=%q)", code, tc.wantCode, errOut.String())
			}
			if tc.wantErrSub == "" && out.String() != tc.wantOut {
				t.Fatalf("stdout = %q, want %q", out.String(), tc.wantOut)
			}
			if tc.wantErrSub != "" && !strings.Contains(errOut.String(), tc.wantErrSub) {
				t.Fatalf("stderr = %q, want substring %q", errOut.String(), tc.wantErrSub)
			}
		})
	}
}

func Test_main(t *testing.T) {
	origExit, origRun := osExit, runCLI
	t.Cleanup(func() { osExit, runCLI = origExit, origRun })

	gotCode := -1
	osExit = func(code int) { gotCode = code }
	runCLI = func(string, []string, io.Reader, io.Writer, io.Writer, afero.Fs) int { return 7 }

	main()

	if gotCode != 7 {
		t.Fatalf("main propagated exit code %d, want 7", gotCode)
	}
}
