package oss

import (
	"context"
	"net/http"

	"github.com/go-kit/log"
	"github.com/thanos-io/objstore"
	"github.com/thanos-io/objstore/providers/oss"
)

// NewBucketClient creates a new OSS bucket client
func NewBucketClient(ctx context.Context, cfg Config, hedgedRoundTripper func(rt http.RoundTripper) http.RoundTripper, name string, logger log.Logger) (objstore.Bucket, error) {
	bucketConfig := oss.Config{
		Endpoint:          cfg.Endpoint,
		Region:            cfg.Region,
		Bucket:            cfg.BucketName,
		AccessKeyID:       cfg.AccessKeyID,
		AccessKeySecret:   cfg.SecretAccessKey.Value,
		RoleArn:           cfg.RoleArn,
		OIDCProviderArn:   cfg.OIDCProviderArn,
		OIDCTokenFilePath: cfg.OIDCTokenFilePath,
		RoleSessionName:   cfg.RoleSessionName,
	}

	return oss.NewBucketWithConfig(logger, bucketConfig, name, hedgedRoundTripper)
}
