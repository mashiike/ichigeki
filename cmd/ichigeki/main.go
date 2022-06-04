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
	Version           = "current"
	defaultConfigPath = ".config/ichigeki/default.toml"
)

func main() {
	flag.CommandLine.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "ichigeki [options] -- (commands)")
		fmt.Fprintln(flag.CommandLine.Output(), "version:", Version)
		flag.CommandLine.PrintDefaults()
	}
	cfg, err := defaultConfig()
	if err != nil {
		log.Fatal("[error] ", err)
	}
	cfg.SetFlags(flag.CommandLine)
	flag.Parse()
	if err := cfg.Restrict(); err != nil {
		log.Fatal("[error] ", err)
	}

	var args []string
	if flag.Arg(0) == "--" {
		args = flag.Args()[1:]
	} else {
		args = flag.Args()
	}
	if len(args) == 0 {
		flag.CommandLine.Usage()
		log.Fatal("[error] commands not found")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ld, err := cfg.LogDestination(ctx)
	if err != nil {
		log.Fatal("[error] ", err)
	}
	h := &ichigeki.Hissatsu{
		Args:                args,
		Name:                cfg.Name,
		DefaultNameTemplate: cfg.DefaultNameTemplate,
		LogDestination:      ld,
		ConfirmDialog:       cfg.ConfirmDialog,
		ExecDate:            cfg.ExecDate,
		Script: func(ctx context.Context, stdout io.Writer, stderr io.Writer) error {
			env := os.Environ()
			env = append(env, `ICHIGEKI_EXECUTION_ENV=ichigeki `+Version+``)
			cmd := exec.CommandContext(ctx, args[0], args[1:]...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = stdout
			cmd.Stderr = stderr
			cmd.Env = env
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("command runtime error: %w", err)
			}
			return nil
		},
	}

	if err := h.ExecuteWithContext(ctx); err != nil {
		log.Fatal("[error] ", err)
	}
}

type config struct {
	Name                string      `toml:"-"`
	ConfirmDialog       *bool       `toml:"confirm_dialog"`
	DefaultNameTemplate string      `toml:"default_name_template"`
	File                *fileConfig `toml:"file"`
	S3                  *s3Config   `toml:"s3"`
	ExecDate            time.Time   `toml:"-"`

	optDir             string `toml:"-"`
	optName            string `toml:"-"`
	optS3URLPrefix     string `toml:"-"`
	optNoConfirmDialog bool   `toml:"-"`
	optExecDate        string `toml:"-"`
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

func defaultConfig() (*config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("can not get user config dir: %w", err)
	}
	defaultConfigPath := filepath.Join(homeDir, defaultConfigPath)
	if configExists(defaultConfigPath) {
		if cfg, err := loadConfig(defaultConfigPath); err != nil {
			return nil, fmt.Errorf("default config load failed: %w", err)
		} else {
			return cfg, nil
		}
	}
	return &config{}, nil
}

func (cfg *config) SetFlags(fs *flag.FlagSet) {
	flag.StringVar(&cfg.optDir, "dir", "", "log destination for s3")
	flag.StringVar(&cfg.optName, "name", "", "ichigeki name")
	flag.StringVar(&cfg.optS3URLPrefix, "s3-url-prefix", "", "log destination for s3")
	flag.StringVar(&cfg.optExecDate, "exec-date", "", "scheduled execution date")
	flag.BoolVar(&cfg.optNoConfirmDialog, "no-confirm-dialog", false, "do confirm")
}

func (cfg *config) Restrict() error {
	cfg.Name = cfg.optName
	if cfg.optNoConfirmDialog {
		cfg.ConfirmDialog = ichigeki.Bool(false)
	}

	if cfg.optS3URLPrefix != "" {
		u, err := url.Parse(cfg.optS3URLPrefix)
		if err != nil {
			return fmt.Errorf("s3-url-prefix can not parse: %w", err)
		}
		if u.Scheme != "s3" {
			return fmt.Errorf("s3-url-prefix is not s3 url format")
		}
		if cfg.S3 == nil {
			cfg.S3 = &s3Config{}
		}
		cfg.S3.Bucket = u.Host
		cfg.S3.ObjectPrefix = u.Path
	}

	if cfg.optDir != "" {
		if !filepath.IsAbs(cfg.optDir) {
			var err error
			cfg.optDir, err = filepath.Abs(cfg.optDir)
			if err != nil {
				return fmt.Errorf("can not convert to abs path: %w", err)
			}
			if cfg.File == nil {
				cfg.File = &fileConfig{}
			}
			cfg.File.Dir = cfg.optDir
		}
	}

	if cfg.optExecDate != "" {
		t, err := time.Parse("2006-01-02", cfg.optExecDate)
		if err != nil {
			return fmt.Errorf("exec date parse failed: %w", err)
		}
		cfg.ExecDate = t
	}
	return nil
}

func (cfg *config) LogDestination(ctx context.Context) (ichigeki.LogDestination, error) {
	logDestinations := make([]ichigeki.LogDestination, 0, 2)
	if cfg.S3 != nil && cfg.S3.Bucket != "" {
		ld, err := s3log.New(ctx, &s3log.Config{
			Bucket:       cfg.S3.Bucket,
			ObjectPrefix: cfg.S3.ObjectPrefix,
		})
		if err != nil {
			return nil, fmt.Errorf("s3 log destination: %w", err)
		}
		logDestinations = append(logDestinations, ld)
	}
	if cfg.File != nil && cfg.File.Dir != "" {
		logDestinations = append(logDestinations, &ichigeki.LocalFile{
			Path:           cfg.File.Dir,
			LogFilePostfix: cfg.File.LogFilePostfix,
		})
	}
	if len(logDestinations) == 0 {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("can not get working directory: %w", err)
		}
		logDestinations = append(logDestinations, &ichigeki.LocalFile{
			Path: wd,
		})
	}

	if len(logDestinations) == 1 {
		return logDestinations[0], nil
	}
	return ichigeki.MultipleLogDestination(logDestinations), nil
}
