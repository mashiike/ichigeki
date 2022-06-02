package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"

	"github.com/mashiike/ichigeki"
)

func main() {
	var (
		name string
	)
	flag.StringVar(&name, name, "", "ichigeki name")
	flag.Parse()

	originalArgs := flag.Args()
	if flag.Arg(0) == "--" {
		originalArgs = originalArgs[1:]
	}
	if len(originalArgs) == 0 {
		flag.PrintDefaults()
		log.Fatal("command not found")
	}
	if name == "" {
		name = filepath.Base(originalArgs[0])
	}

	h := &ichigeki.Hissatsu{
		Name: name,
		Script: func(ctx context.Context, stdout io.Writer, stderr io.Writer) error {
			cmd := exec.CommandContext(ctx, originalArgs[0], originalArgs[1:]...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = stdout
			cmd.Stderr = stderr
			return cmd.Run()
		},
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := h.ExecuteWithContext(ctx); err != nil {
		log.Fatal(err)
	}
}
