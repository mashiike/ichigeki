package s3log

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type S3Client interface {
	s3.HeadObjectAPIClient
	manager.UploadAPIClient
}
type Config struct {
	Bucket        string
	ObjectPrefix  string
	ObjectPostfix string
}

type LogDestination struct {
	name   string
	cfg    *Config
	client S3Client
	w      *s3Writer
}

func New(ctx context.Context, cfg *Config) (*LogDestination, error) {
	opts := make([]func(*config.LoadOptions) error, 0)
	if region := os.Getenv("AWS_DEFAULT_REGION"); region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}
	ld := &LogDestination{
		cfg:    cfg,
		client: s3.NewFromConfig(awsCfg),
	}
	return ld, nil
}

func (ld LogDestination) object() string {
	if ld.cfg.ObjectPostfix == "" {
		ld.cfg.ObjectPostfix = ".log"
	}
	key := fmt.Sprintf("%s%s%s", ld.cfg.ObjectPrefix, ld.name, ld.cfg.ObjectPostfix)
	return strings.TrimLeft(key, "/")
}

func (ld *LogDestination) AlreadyExists(ctx context.Context) (bool, error) {
	_, err := ld.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(ld.cfg.Bucket),
		Key:    aws.String(ld.object()),
	})
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "NotFound" {
				return false, nil
			}
		}
		return false, err
	}
	return true, nil

}
func (ld *LogDestination) NewWriter(ctx context.Context) (io.Writer, io.Writer, error) {
	ld.w = newS3Writer(ld.client, &s3.PutObjectInput{
		Bucket: aws.String(ld.cfg.Bucket),
		Key:    aws.String(ld.object()),
	})
	return ld.w, ld.w, nil
}

func (ld *LogDestination) Cleanup(_ context.Context) {
	if ld.w != nil {
		ld.w.Close()
	}
}

func (ld *LogDestination) SetName(name string) {
	ld.name = name
}

func (ld *LogDestination) String() string {
	return fmt.Sprintf("s3://%s/%s", ld.cfg.Bucket, ld.object())
}

type s3Writer struct {
	w   *io.PipeWriter
	err error
	wg  sync.WaitGroup
}

func newS3Writer(client manager.UploadAPIClient, input *s3.PutObjectInput) *s3Writer {
	uploader := manager.NewUploader(client)
	pr, pw := io.Pipe()
	w := &s3Writer{
		w: pw,
	}
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		input.Body = pr
		_, w.err = uploader.Upload(context.Background(), input)
		pr.Close()
	}()
	return w
}

func (w *s3Writer) Write(p []byte) (int, error) {
	if w.err != nil {
		err := w.err
		w.err = nil
		return 0, err
	}
	return w.w.Write(p)
}

func (w *s3Writer) Close() {
	if err := w.w.Close(); err != nil {
		log.Printf("[error] pipe writer close failed: %s", err.Error())
	}
	w.wg.Wait()
	if w.err != nil {
		log.Printf("[error] upload finish failed: %s", w.err.Error())
	}
}
