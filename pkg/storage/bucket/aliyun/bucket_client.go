package aliyun

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-kit/log"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/thanos-io/objstore"

	cortexs3 "github.com/cortexproject/cortex/pkg/storage/bucket/s3"
)

var defaultRetryMinBackoff = 5 * time.Second
var defaultRetryMaxBackoff = 1 * time.Minute

func NewBucketClient(cfg Config, hedgedRoundTripper func(rt http.RoundTripper) http.RoundTripper, name string, logger log.Logger) (objstore.Bucket, error) {
	baseBucket, err := newBucket(cfg, hedgedRoundTripper, name, logger)
	if err != nil {
		return nil, err
	}

	return cortexs3.NewBucketWithRetries(baseBucket, 5, defaultRetryMinBackoff, defaultRetryMaxBackoff, logger)
}

func resolveCredentials(cfg Config) (accessKeyID, secretAccessKey, sessionToken string, err error) {
	if cfg.AccessKeyID != "" {
		return cfg.AccessKeyID, cfg.SecretAccessKey.Value, cfg.SessionToken.Value, nil
	}

	if rrsaConfig, ok, err := cfg.rrsaConfig(); err != nil {
		return "", "", "", err
	} else if ok {
		return rrsaConfig.RoleArn, "__rrsa__", "", nil
	}

	accessKeyID = firstNonEmptyEnv("ALIBABA_CLOUD_ACCESS_KEY_ID", "OSS_ACCESS_KEY_ID")
	secretAccessKey = firstNonEmptyEnv("ALIBABA_CLOUD_ACCESS_KEY_SECRET", "OSS_ACCESS_KEY_SECRET")
	sessionToken = firstNonEmptyEnv("ALIBABA_CLOUD_SECURITY_TOKEN", "OSS_SESSION_TOKEN")

	if accessKeyID == "" || secretAccessKey == "" {
		return "", "", "", fmt.Errorf("missing Aliyun credentials: set access_key_id/secret_access_key, ALIBABA_CLOUD_ACCESS_KEY_ID/SECRET, or RRSA env vars ALIBABA_CLOUD_ROLE_ARN, ALIBABA_CLOUD_OIDC_PROVIDER_ARN, ALIBABA_CLOUD_OIDC_TOKEN_FILE")
	}

	return accessKeyID, secretAccessKey, sessionToken, nil
}

func buildProvider(cfg Config) (credentials.Provider, error) {
	if cfg.AccessKeyID != "" {
		return &credentials.Static{
			Value: credentials.Value{
				AccessKeyID:     cfg.AccessKeyID,
				SecretAccessKey: cfg.SecretAccessKey.Value,
				SessionToken:    cfg.SessionToken.Value,
				SignerType:      credentials.SignatureV4,
			},
		}, nil
	}

	accessKeyID := firstNonEmptyEnv("ALIBABA_CLOUD_ACCESS_KEY_ID", "OSS_ACCESS_KEY_ID")
	secretAccessKey := firstNonEmptyEnv("ALIBABA_CLOUD_ACCESS_KEY_SECRET", "OSS_ACCESS_KEY_SECRET")
	if accessKeyID != "" && secretAccessKey != "" {
		return &credentials.Static{
			Value: credentials.Value{
				AccessKeyID:     accessKeyID,
				SecretAccessKey: secretAccessKey,
				SessionToken:    firstNonEmptyEnv("ALIBABA_CLOUD_SECURITY_TOKEN", "OSS_SESSION_TOKEN"),
				SignerType:      credentials.SignatureV4,
			},
		}, nil
	}

	rrsaCfg, ok, err := cfg.rrsaConfig()
	if err != nil {
		return nil, err
	}
	if ok {
		return newRRSACredentialsProvider(rrsaCfg), nil
	}

	return nil, fmt.Errorf("missing Aliyun credentials: set access_key_id/secret_access_key, ALIBABA_CLOUD_ACCESS_KEY_ID/SECRET, or RRSA env vars ALIBABA_CLOUD_ROLE_ARN, ALIBABA_CLOUD_OIDC_PROVIDER_ARN, ALIBABA_CLOUD_OIDC_TOKEN_FILE")
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}

	return ""
}
