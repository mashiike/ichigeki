package ichigeki_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Songmu/flextime"
	"github.com/mashiike/ichigeki"
	"github.com/stretchr/testify/require"
)

func TestHissatsuNotToday(t *testing.T) {
	restore := flextime.Set(time.Date(2022, 6, 1, 2, 3, 4, 5, time.Local))
	defer restore()

	h := &ichigeki.Hissatsu{
		Name:     "test_run",
		ExecDate: time.Date(2022, 6, 5, 0, 0, 0, 0, time.Local),
		Script: func(_ context.Context, stdout io.Writer, _ io.Writer) error {
			fmt.Fprintf(stdout, "run!")
			return nil
		},
	}
	require.EqualError(t, h.Execute(), "exec_date: 2022-06-05 is not today! (today: 2022-05-31)")
}

func TestHissatsuLogAlreadyExists(t *testing.T) {
	restore := flextime.Set(time.Date(2022, 6, 5, 12, 0, 0, 0, time.Local))
	defer restore()

	h := &ichigeki.Hissatsu{
		Name:     "test_run",
		ExecDate: time.Date(2022, 6, 5, 0, 0, 0, 0, time.Local),
		LogDestination: &ichigeki.LocalFile{
			Path: "testdata/",
		},
		Script: func(_ context.Context, stdout io.Writer, _ io.Writer) error {
			fmt.Fprintf(stdout, "run!")
			return nil
		},
	}
	require.EqualError(t, h.Execute(), "Can't execute! Execution log destination [testdata/test_run.log] already exists")
}

func TestHissatsuPromptNo(t *testing.T) {
	restore := flextime.Set(time.Date(2022, 6, 5, 12, 0, 0, 0, time.Local))
	defer restore()

	h := &ichigeki.Hissatsu{
		Name:     "test_run",
		ExecDate: time.Date(2022, 6, 5, 0, 0, 0, 0, time.Local),
		Script: func(_ context.Context, stdout io.Writer, _ io.Writer) error {
			fmt.Fprintf(stdout, "run!")
			return nil
		},
		PromptInput: strings.NewReader("no\n"),
	}
	require.EqualError(t, h.Execute(), "canceled.")
}

func TestHissatsuDoubleRun(t *testing.T) {
	restore := flextime.Set(time.Date(2022, 6, 5, 12, 0, 0, 0, time.Local))
	defer restore()
	tempDir := t.TempDir()
	h := &ichigeki.Hissatsu{
		Name:     "test_run",
		ExecDate: time.Date(2022, 6, 5, 0, 0, 0, 0, time.Local),
		LogDestination: &ichigeki.LocalFile{
			Path: tempDir,
		},
		Script: func(_ context.Context, stdout io.Writer, _ io.Writer) error {
			fmt.Fprintf(stdout, "run!")
			return nil
		},
		PromptInput: strings.NewReader("yes\n"),
	}
	require.NoError(t, h.Execute())
	logPath := filepath.Join(tempDir, "test_run.log")
	require.EqualValues(
		t,
		readFile(t, "testdata/test_run.log"),
		readFile(t, logPath),
	)
	require.EqualError(t, h.Execute(), fmt.Sprintf("Can't execute! Execution log destination [%s] already exists", logPath))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	bs, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(bs)
}
