package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigLoadBoth(t *testing.T) {
	cfg, err := loadConfig("testdata/config.toml")
	require.NoError(t, err)
	expected := &config{
		S3: &s3Config{
			Bucket:       "example-com",
			ObjectPrefix: "hoge/",
		},
		File: &fileConfig{
			Dir: "./",
		},
	}
	require.EqualValues(t, expected, cfg)
}

func TestConfigLoadOnlyS3(t *testing.T) {
	cfg, err := loadConfig("testdata/s3_only.toml")
	require.NoError(t, err)
	expected := &config{
		S3: &s3Config{
			Bucket:       "example-com",
			ObjectPrefix: "hoge/",
		},
	}
	require.EqualValues(t, expected, cfg)
}
