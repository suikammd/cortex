package aliyun

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	bucket_http "github.com/cortexproject/cortex/pkg/storage/bucket/http"
	"github.com/cortexproject/cortex/pkg/storage/bucket/s3"
	"github.com/cortexproject/cortex/pkg/util/flagext"
)

func TestConfig(t *testing.T) {
	t.Parallel()

	cfg := Config{}
	flagext.DefaultValues(&cfg)

	err := yaml.Unmarshal([]byte(`
endpoint: test-endpoint
region: cn-hangzhou
bucket_name: test-bucket-name
disable_dualstack: true
secret_access_key: test-secret-access-key
access_key_id: test-access-key-id
session_token: test-session-token
insecure: true
signature_version: v4
bucket_lookup_type: virtual-hosted
http:
  idle_conn_timeout: 2s
`), &cfg)
	require.NoError(t, err)
	require.Equal(t, Config{
		Endpoint:          "test-endpoint",
		Region:            "cn-hangzhou",
		BucketName:        "test-bucket-name",
		DisableDualstack:  true,
		SecretAccessKey:   flagext.Secret{Value: "test-secret-access-key"},
		AccessKeyID:       "test-access-key-id",
		SessionToken:      flagext.Secret{Value: "test-session-token"},
		RoleSessionExpiry: 3600,
		Insecure:          true,
		SignatureVersion:  s3.SignatureVersionV4,
		BucketLookupType:  s3.BucketVirtualHostLookup,
		SendContentMd5:    true,
		HTTP: HTTPConfig{
			Config: bucket_http.Config{
				IdleConnTimeout:       2 * time.Second,
				ResponseHeaderTimeout: 2 * time.Minute,
				InsecureSkipVerify:    false,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				MaxIdleConns:          100,
				MaxIdleConnsPerHost:   100,
				MaxConnsPerHost:       0,
			},
		},
	}, cfg)
}

func TestToS3Config_UsesExplicitCredentials(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Endpoint:         "oss-cn-hangzhou.aliyuncs.com",
		AccessKeyID:      "explicit-ak",
		SecretAccessKey:  flagext.Secret{Value: "explicit-sk"},
		SessionToken:     flagext.Secret{Value: "explicit-token"},
		SignatureVersion: s3.SignatureVersionV4,
		BucketLookupType: s3.BucketAutoLookup,
		SendContentMd5:   true,
	}
	s3Cfg, err := cfg.toS3Config()
	require.NoError(t, err)
	require.Equal(t, "explicit-ak", s3Cfg.AccessKeyID)
	require.Equal(t, "explicit-sk", s3Cfg.SecretAccessKey.Value)
	require.Equal(t, "explicit-token", s3Cfg.SessionToken.Value)
}

func TestToS3Config_UsesEnvironmentCredentials(t *testing.T) {
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_ID", "aliyun-ak")
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET", "aliyun-sk")
	t.Setenv("ALIBABA_CLOUD_SECURITY_TOKEN", "aliyun-token")

	cfg := Config{
		Endpoint:         "oss-cn-hangzhou.aliyuncs.com",
		SignatureVersion: s3.SignatureVersionV4,
		BucketLookupType: s3.BucketAutoLookup,
		SendContentMd5:   true,
	}
	s3Cfg, err := cfg.toS3Config()
	require.NoError(t, err)
	require.Equal(t, "aliyun-ak", s3Cfg.AccessKeyID)
	require.Equal(t, "aliyun-sk", s3Cfg.SecretAccessKey.Value)
	require.Equal(t, "aliyun-token", s3Cfg.SessionToken.Value)
}

func TestToS3Config_FailsWithoutCredentials(t *testing.T) {
	cfg := Config{
		Endpoint:         "oss-cn-hangzhou.aliyuncs.com",
		SignatureVersion: s3.SignatureVersionV4,
		BucketLookupType: s3.BucketAutoLookup,
		SendContentMd5:   true,
	}
	_, err := cfg.toS3Config()
	require.EqualError(t, err, "missing Aliyun credentials: set access_key_id/secret_access_key, ALIBABA_CLOUD_ACCESS_KEY_ID/SECRET, or RRSA env vars ALIBABA_CLOUD_ROLE_ARN, ALIBABA_CLOUD_OIDC_PROVIDER_ARN, ALIBABA_CLOUD_OIDC_TOKEN_FILE")
}
