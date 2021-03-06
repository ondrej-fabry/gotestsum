package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/gotestsum/internal/text"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
	"gotest.tools/v3/fs"
	"gotest.tools/v3/golden"
)

func TestE2E_RerunFails(t *testing.T) {
	type testCase struct {
		name        string
		args        []string
		expectedErr string
	}
	fn := func(t *testing.T, tc testCase) {
		tmpFile := fs.NewFile(t, t.Name()+"-seedfile", fs.WithContent("0"))
		defer tmpFile.Remove()

		envVars := osEnviron()
		envVars["TEST_SEEDFILE"] = tmpFile.Path()
		defer env.PatchAll(t, envVars)()

		flags, opts := setupFlags("gotestsum")
		assert.NilError(t, flags.Parse(tc.args))
		opts.args = flags.Args()

		bufStdout := new(bytes.Buffer)
		opts.stdout = bufStdout
		bufStderr := new(bytes.Buffer)
		opts.stderr = bufStderr

		err := run(opts)
		if tc.expectedErr != "" {
			assert.Error(t, err, tc.expectedErr)
		} else {
			assert.NilError(t, err)
		}
		out := text.ProcessLines(t, bufStdout,
			text.OpRemoveSummaryLineElapsedTime,
			text.OpRemoveTestElapsedTime,
			filepath.ToSlash, // for windows
		)
		golden.Assert(t, out, "e2e/expected/"+expectedFilename(t.Name()))
	}
	var testCases = []testCase{
		{
			name: "reruns until success",
			args: []string{
				"-f=testname",
				"--rerun-fails=4",
				"--packages=./testdata/e2e/flaky/",
				"--", "-count=1", "-tags=testdata",
			},
		},
		{
			name: "reruns continues to fail",
			args: []string{
				"-f=testname",
				"--rerun-fails=2",
				"--packages=./testdata/e2e/flaky/",
				"--", "-count=1", "-tags=testdata",
			},
			expectedErr: "exit status 1",
		},
		{
			name: "first run has errors, abort rerun",
			args: []string{
				"-f=testname",
				"--rerun-fails=2",
				"--packages=./testjson/internal/broken",
				"--", "-count=1", "-tags=stubpkg",
			},
			expectedErr: "rerun aborted because previous run had errors",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if testing.Short() {
				t.Skip("too slow for short run")
			}
			fn(t, tc)
		})
	}
}

// osEnviron returns os.Environ() as a map, with any GOTESTSUM_ env vars removed
// so that they do not alter the test results.
func osEnviron() map[string]string {
	e := env.ToMap(os.Environ())
	for k := range e {
		if strings.HasPrefix(k, "GOTESTSUM_") {
			delete(e, k)
		}
	}
	return e
}

func expectedFilename(name string) string {
	ver := runtime.Version()
	switch {
	case isPreGo114(ver):
		return name + "-go1.13"
	default:
		return name
	}
}

// go1.14.6 changed how it prints messages from tests. go1.14.{0-5} used a format
// that was different from both go1.14.6 and previous versions of Go. These tests
// no longer support that format.
func isPreGo114(ver string) bool {
	prefix := "go1.1"
	if !strings.HasPrefix(ver, prefix) || len(ver) < len(prefix)+1 {
		return false
	}
	switch ver[len(prefix)] {
	case '0', '1', '2', '3':
		return true
	}
	return false
}
