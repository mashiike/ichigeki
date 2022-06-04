# ichigeki

[![Documentation](https://godoc.org/github.com/mashiike/ichigeki?status.svg)](https://godoc.org/github.com/mashiike/ichigeki)
![Latest GitHub release](https://img.shields.io/github/release/mashiike/ichigeki.svg)
![Github Actions test](https://github.com/mashiike/ichigeki/workflows/Test/badge.svg?branch=main)
[![Go Report Card](https://goreportcard.com/badge/mashiike/ichigeki)](https://goreportcard.com/report/mashiike/ichigeki)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/mashiike/ichigeki/blob/master/LICENSE)

ichigeki is the util tool for one time script for mission critical (especially for preventing rerunning it).
this tool inspired by perl's [Script::Ichigeki](https://github.com/Songmu/p5-Script-Ichigeki)

## Usage as a CLI 

for ECS Task. log output to S3 Bucket.
For example, for a one time script that needs to be executed on 2022-06-01
```shell
$ ichigeki --s3-url-prefix s3://ichigeki-example-com/logs/ --exec-date 2022-06-01 --no-confirm-dialog -- your_command
```

or with default config `~/.config/ichigeki/default.toml`.

```toml
confirm_dialog = false
default_name_template = "{{ .Name }}{{ if gt (len .Args) 1}}-{{ .Args | hash }}{{ end }}"

[s3]
bucket = "ichigeki-example-com"
object_prefix = "logs/"
```

```shell
$ ichigeki --exec-date 2022-06-01 -- your_command
```

### ICHIGEKI_EXECUTION_ENVs 

If you want to check whether the command is started using ichigeki on the side of the command to be started, you can check the environment variable named ICHIGEKI_EXECUTION_ENV. If version information is stored, it is invoked via ichigeki command.

sample.sh
```shell
#!/bin/bash

echo $ICHIGEKI_EXECUTION_ENV
echo $ICHIGEKI_EXECUTION_NAME
echo $ICHIGEKI_EXECUTION_DATE
```

```shell
$ ichigeki --s3-url-prefix s3://ichigeki-example-com/logs/  -- ./sample.sh           
[info] log output to `s3://ichigeki-example-com/logs/sample.sh.log`
ichigeki v0.3.0 
```

### default_name_template in `~/.config/ichigeki/default.toml`

The default configuration file provides a template for dynamically determining the ichigeki name.
This dynamic ichigeki name mechanism works only if you do not explicitly specify `-name` in the options
This template follows the Go template notation [text/template](https://pkg.go.dev/text/template)

The following data is passed to the template:

- `.Name` : Default name if template is not specified
- `.ExecDate`: `-exec-date` or the value of the execution date. format(2016-01-02)
- `.Today`: the value of the execution date. format(2016-01-02)
- `.Args`: A space-separated array of the commands passed. (type []string) 

The following custom functions are passed:

- `sha256` : Given a string or []string, compute the hexadecimal notation of sha256 hash
- `hash` : First 7 characters of the hexadecimal notation of the sha256 hash
- `arg` :  Value of the specified index of .Args. If not present, it will be an empty string. (sample {{ arg 1 }})
- `last_arg` : Last element of .Args
- `env` : Refers to the environment variable at the start of execution. If not set, an empty character will be returned.
- `must_env` : Refers to the environment variable at the start of execution. If it is not set, it will panic.

For example: `default_name_template` = `"{{ .Name }}{{ if gt (len .Args) 1}}-{{ .Args | hash }}{{ end }}"`

`$ ichigeki -- ./sample.sh` => `sample.sh`
`$ ichigeki -- go run cmd/migration/. --debug` => `go-4575533`

### Install 
#### Homebrew (macOS and Linux)

```console
$ brew install mashiike/tap/ichigeki
```

#### Binary packages

[Releases](https://github.com/mashiike/ichigeki/releases)

### Options

```shell
ichigeki [options] -- (commands)
  -dir string
        log destination for s3
  -exec-date string
        scheduled execution date
  -name string
        ichigeki name
  -no-confirm-dialog
        do confirm
  -s3-url-prefix string
        log destination for s3
```
## Usage as a library

for example:

```go
package main

import (
    "fmt"
    "log"

    "github.com/mashiike/ichigeki"
    "github.com/mashiike/ichigeki/s3log"
)


func main() {
    ld, err := s3log.New(context.Background(), &s3log.Config{
        Bucket:       "ichigeki-example-come",
        ObjectPrefix: "logs/",
    })      
    if err != nil {
        log.Fatal("s3 log destination:", err)
    }
    h := &ichigeki.Hissatsu{
        Name:           "hogehoge",
        LogDestination: ld,
        ConfirmDialog:  ichigeki.Bool(true),
        Script: func(_ context.Context, stdout io.Writer, stderr io.Writer) error {
            fmt.Fprintln(stdout, "this message out to stdout") 
            fmt.Fprintln(stderr, "this message out to stderr") 
            return nil 
        }, 
    }

    if err := h.Execute(); err != nil {
        log.Fatal(err)
    }
}
```

## LICENSE

MIT License

Copyright (c) 2022 IKEDA Masashi
