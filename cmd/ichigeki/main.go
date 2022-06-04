package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/mashiike/ichigeki"
	"github.com/mashiike/ichigeki/s3log"
	"github.com/pelletier/go-toml"
)

const (
	Version       = "current"
	defaultConfig = ".config/ichigeki/default.toml"
)

func main() {
	flag.CommandLine.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "ichigeki [options] -- (commands)")
		fmt.Fprintln(flag.CommandLine.Output(), "version:", Version)
		flag.CommandLine.PrintDefaults()
	}
	var (
		cfg             *config
		name            string
		s3URLPrefix     string
		dir             string
		noConfirmDialog bool
		execDate        string
	)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("can not get user config dir: ", err)
	}
	defaultConfigPath := filepath.Join(homeDir, defaultConfig)
	if configExists(defaultConfigPath) {
		var err error
		cfg, err = loadConfig(defaultConfigPath)
		if err != nil {
			log.Fatal("default config load failed:", err)
		}
	} else {
		cfg = &config{}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	flag.StringVar(&name, "name", "", "ichigeki name")
	flag.StringVar(&s3URLPrefix, "s3-url-prefix", "", "log destination for s3")
	flag.StringVar(&dir, "dir", "", "log destination for s3")
	flag.StringVar(&execDate, "exec-date", "", "scheduled execution date")
	flag.BoolVar(&noConfirmDialog, "no-confirm-dialog", false, "do confirm")
	flag.Parse()
	if noConfirmDialog {
		cfg.ConfirmDialog = ichigeki.Bool(false)
	}

	if s3URLPrefix != "" {
		u, err := url.Parse(s3URLPrefix)
		if err != nil {
			log.Fatal("s3-url-prefix can not parse:", err)
		}
		if u.Scheme != "s3" {
			log.Fatal("s3-url-prefix is not s3 url format")
		}
		if cfg.S3 == nil {
			cfg.S3 = &s3Config{}
		}
		cfg.S3.Bucket = u.Host
		cfg.S3.ObjectPrefix = u.Path
	}

	if dir != "" {
		if !filepath.IsAbs(dir) {
			var err error
			dir, err = filepath.Abs(dir)
			if err != nil {
				log.Fatal("can not convert to abs path:", err)
			}
			if cfg.File == nil {
				cfg.File = &fileConfig{}
			}
			cfg.File.Dir = dir
		}
	}

	logDestinations := make([]ichigeki.LogDestination, 0, 2)
	if cfg.S3 != nil {
		ld, err := s3log.New(ctx, &s3log.Config{
			Bucket:       cfg.S3.Bucket,
			ObjectPrefix: cfg.S3.ObjectPrefix,
		})
		if err != nil {
			log.Fatal("s3 log destination:", err)
		}
		logDestinations = append(logDestinations, ld)
	}
	if cfg.File != nil {
		logDestinations = append(logDestinations, &ichigeki.LocalFile{
			Path:           cfg.File.Dir,
			LogFilePostfix: cfg.File.LogFilePostfix,
		})
	}
	if len(logDestinations) == 0 {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatal("can not get working directory:", err)
		}
		logDestinations = append(logDestinations, &ichigeki.LocalFile{
			Path: wd,
		})
	}

	var logDestination ichigeki.LogDestination
	if len(logDestinations) == 1 {
		logDestination = logDestinations[0]
	} else {
		logDestination = ichigeki.MultipleLogDestination(logDestinations)
	}
	var args []string
	if flag.Arg(0) == "--" {
		args = flag.Args()[1:]
	} else {
		args = flag.Args()
	}
	if len(args) == 0 {
		flag.CommandLine.Usage()
		log.Fatal("commands not found")
	}

	h := &ichigeki.Hissatsu{
		Args:           args,
		LogDestination: logDestination,
		ConfirmDialog:  cfg.ConfirmDialog,
		Script: func(ctx context.Context, stdout io.Writer, stderr io.Writer) error {
			env := os.Environ()
			env = append(env, `ICHIGEKI_EXECUTION_ENV=ichigeki `+Version+``)
			cmd := exec.CommandContext(ctx, args[0], args[1:]...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = stdout
			cmd.Stderr = stderr
			cmd.Env = env
			return fmt.Errorf("command runtime error: %w", cmd.Run())
		},
	}

	if execDate != "" {
		t, err := time.Parse("2006-01-02", execDate)
		if err != nil {
			log.Fatal("[error] exec date parse failed: ", err)
		}
		h.ExecDate = t
	}
	if err := h.ExecuteWithContext(ctx); err != nil {
		log.Fatal("[error] ", err)
	}
}

type config struct {
	ConfirmDialog *bool       `toml:"confirm_dialog"`
	File          *fileConfig `toml:"file"`
	S3            *s3Config   `toml:"s3"`
}

type s3Config struct {
	Bucket       string `toml:"bucket"`
	ObjectPrefix string `toml:"object_prefix"`
}

type fileConfig struct {
	Dir            string `toml:"dir"`
	LogFilePostfix string `toml:"log_file_postfix"`
}

func loadConfig(path string) (*config, error) {
	fp, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fp.Close()
	cfg := &config{}
	decoder := toml.NewDecoder(fp)
	if err := decoder.Decode(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func configExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
