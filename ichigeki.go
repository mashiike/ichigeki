package ichigeki

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Songmu/flextime"
)

type LogDestination interface {
	fmt.Stringer
	SetName(name string)
	AlreadyExists(ctx context.Context) (exists bool, err error)
	NewWriter(ctx context.Context) (stdout io.Writer, stderr io.Writer, err error)
	Cleanup(ctx context.Context)
}

type Hissatsu struct {
	Name           string
	Description    string
	ExecDate       time.Time
	ConfirmDialog  *bool
	LogDestination LogDestination
	Script         func(ctx context.Context, stdout io.Writer, stderr io.Writer) error
	DialogMessage  string
	PromptInput    io.Reader

	inCompilation bool
}

func (h *Hissatsu) Validate() error {
	if h.Script == nil {
		return errors.New("Script is required")
	}
	if h.Name == "" {
		h.Name = filepath.Base(os.Args[0])
	}
	if h.ExecDate.IsZero() {
		h.ExecDate = flextime.Now().In(time.Local)
	}
	h.ExecDate.Local().Truncate(24 * time.Hour)
	if h.ConfirmDialog == nil {
		h.ConfirmDialog = Bool(true)
	}
	if h.LogDestination == nil {
		h.LogDestination = &LocalFile{}
		fmt.Fprintln(os.Stderr, "[warn] LogDestination is not specified. use default LocalFile")
	}
	h.LogDestination.SetName(h.Name)

	if h.DialogMessage == "" {
		h.DialogMessage = "Do you really execute `%s` ?"
	}
	if cnt := strings.Count(h.DialogMessage, "%s"); cnt != 1 {
		return fmt.Errorf("DialogMessage must always contain one string format specifier %%s: string format specifier count is %d", cnt)
	}
	if h.PromptInput == nil {
		h.PromptInput = os.Stdin
	}
	return nil
}

func (h *Hissatsu) Execute() error {
	return h.ExecuteWithContext(context.Background())
}

func (h *Hissatsu) ExecuteWithContext(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			return
		}
		if rec := recover(); rec != nil {
			switch {
			case h.inCompilation == false:
				fmt.Fprintln(os.Stderr, "script is not complete, but panicked")
				panic(rec)
			default:
				fmt.Fprintf(os.Stderr, "[error] %s", rec)
			}
		}
	}()
	if verr := h.Validate(); verr != nil {
		err = fmt.Errorf("Hissatsu.Validate(): %w", verr)
		return
	}
	today := flextime.Now().In(time.Local).Truncate(24 * time.Hour)
	if h.ExecDate.Format("2006-01-02") != today.Format("2006-01-02") {
		err = fmt.Errorf("exec_date: %s is not today! (today: %s)", h.ExecDate.Format("2006-01-02"), today.Format("2006-01-02"))
		return
	}
	if exists, checkErr := h.LogDestination.AlreadyExists(ctx); checkErr != nil {
		err = fmt.Errorf("Can't execute! Execution log destination [%s] check failed: %w", h.LogDestination.String(), checkErr)
		return
	} else if exists {
		err = fmt.Errorf("Can't execute! Execution log destination [%s] already exists", h.LogDestination.String())
		return
	}

	if *h.ConfirmDialog {
		fmt.Fprintf(os.Stderr, h.DialogMessage+" [y/n]:", h.Name)
		reader := bufio.NewReader(h.PromptInput)
		response, promptErr := reader.ReadString('\n')
		if promptErr != nil {
			err = fmt.Errorf("prompt error: %w", promptErr)
			return
		}
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			err = errors.New("canceled.")
			return
		}
	}
	fmt.Fprintf(os.Stderr, "log output to `%s`\n", h.LogDestination.String())
	err = h.running(ctx)
	return
}

func (h *Hissatsu) running(ctx context.Context) error {
	stdout, stderr, newErr := h.LogDestination.NewWriter(ctx)
	if newErr != nil {
		return fmt.Errorf("Can't execute! Execution log destination [%s] initialize failed: %w", h.LogDestination.String(), newErr)
	}
	var w io.Writer
	if stdout == stderr {
		w = stdout
	} else {
		w = io.MultiWriter(stdout, stderr)
	}

	fmt.Fprintln(w, "# This log is generated dy github.com/mashiike/ichigeki.Hissatsu")
	fmt.Fprintf(w, "name: %s\n", h.Name)
	fmt.Fprintf(w, "start: %s\n", flextime.Now().In(time.Local).Format(time.RFC3339))
	fmt.Fprint(w, "---\n")
	defer func() {
		fmt.Fprint(w, "\n---\n")
		fmt.Fprintf(w, "end: %s\n", flextime.Now().In(time.Local).Format(time.RFC3339))
		h.LogDestination.Cleanup(ctx)
	}()
	if err := h.Script(
		ctx,
		io.MultiWriter(stdout, os.Stdout),
		io.MultiWriter(stderr, os.Stderr),
	); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "")
	h.inCompilation = true
	return nil
}

func Bool(b bool) *bool {
	return &b
}

type LocalFile struct {
	Path           string
	LogFilePostfix string
	name           string
	fp             *os.File
	writer         *bufio.Writer
}

func (f *LocalFile) AlreadyExists(_ context.Context) (bool, error) {
	_, err := os.Stat(f.String())
	return err == nil, nil

}
func (f *LocalFile) NewWriter(ctx context.Context) (io.Writer, io.Writer, error) {
	var err error
	f.fp, err = os.Create(f.String())
	f.writer = bufio.NewWriter(f.fp)
	return f.writer, f.writer, err
}

func (f *LocalFile) Cleanup(ctx context.Context) {
	if f.fp != nil {
		f.writer.Flush()
		f.fp.Close()
	}
}

func (f *LocalFile) logFilePostfix() string {
	if f.LogFilePostfix == "" {
		return ".log"
	}
	return f.LogFilePostfix
}

func (f *LocalFile) path() string {
	if f.Path == "" {
		p, _ := os.Getwd()
		return p
	}
	return f.Path
}

func (f *LocalFile) SetName(name string) {
	f.name = name
}

func (f *LocalFile) String() string {
	return filepath.Join(f.path(), f.name+f.logFilePostfix())
}
