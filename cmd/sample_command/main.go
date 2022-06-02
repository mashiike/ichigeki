package main

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/mashiike/ichigeki"
)

func main() {
	h := &ichigeki.Hissatsu{
		Script: func(_ context.Context, stdout io.Writer, _ io.Writer) error {
			fmt.Fprintf(stdout, "run!")
			return nil
		},
	}
	if err := h.Execute(); err != nil {
		log.Fatal(err)
	}
}
