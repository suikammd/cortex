package aliyun

import (
	"flag"
	"net/http"

	bucket_http "github.com/cortexproject/cortex/pkg/storage/bucket/http"
	"github.com/cortexproject/cortex/pkg/storage/bucket/s3"
	"github.com/cortexproject/cortex/pkg/util/flagext"
)

// HTTPConfig stores the http.Transport configuration for the Aliyun OSS client.
type HTTPConfig struct {
	bucket_http.Config `yaml:",inline"`

	Transport http.RoundTripper `yaml:"-"`
}

// RegisterFlagsWithPrefix registers the flags for the Aliyun OSS HTTP client with the provided prefix.
func (cfg *HTTPConfig) RegisterFlagsWithPrefix(prefix string, f *flag.FlagSet) {
	cfg.Config.RegisterFlagsWithPrefix(prefix+"aliyun.", f)
}

// Config holds the config options for the Aliyun OSS backend.
type Config struct {
	Endpoint           string         `yaml:"endpoint"`
	Region             string         `yaml:"region"`
	BucketName         string         `yaml:"bucket_name"`
	DisableDualstack   bool           `yaml:"disable_dualstack"`
	SecretAccessKey    flagext.Secret `yaml:"secret_access_key"`
	AccessKeyID        string         `yaml:"access_key_id"`
	SessionToken       flagext.Secret `yaml:"session_token"`
	RoleArn            string         `yaml:"role_arn"`
	OIDCProviderArn    string         `yaml:"oidc_provider_arn"`
	OIDCTokenFile      string         `yaml:"oidc_token_file"`
	RoleSessionName    string         `yaml:"role_session_name"`
	Policy             flagext.Secret `yaml:"policy"`
	RoleSessionExpiry  int            `yaml:"role_session_expiration"`
	STSEndpoint        string         `yaml:"sts_endpoint"`
	Insecure           bool           `yaml:"insecure"`
	SignatureVersion   string         `yaml:"signature_version"`
	BucketLookupType   string         `yaml:"bucket_lookup_type"`
	SendContentMd5     bool           `yaml:"send_content_md5"`
	ListObjectsVersion string         `yaml:"list_objects_version"`

	SSE  s3.SSEConfig `yaml:"sse"`
	HTTP HTTPConfig   `yaml:"http"`
}

// RegisterFlags registers the flags for Aliyun OSS storage.
func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	cfg.RegisterFlagsWithPrefix("", f)
}

// RegisterFlagsWithPrefix registers the flags for Aliyun OSS storage with the provided prefix.
func (cfg *Config) RegisterFlagsWithPrefix(prefix string, f *flag.FlagSet) {
	f.StringVar(&cfg.AccessKeyID, prefix+"aliyun.access-key-id", "", "Aliyun OSS access key ID")
	f.Var(&cfg.SecretAccessKey, prefix+"aliyun.secret-access-key", "Aliyun OSS secret access key")
	f.Var(&cfg.SessionToken, prefix+"aliyun.session-token", "Aliyun OSS session token")
	f.StringVar(&cfg.RoleArn, prefix+"aliyun.role-arn", "", "Aliyun RAM role ARN used for RRSA / OIDC role assumption.")
	f.StringVar(&cfg.OIDCProviderArn, prefix+"aliyun.oidc-provider-arn", "", "Aliyun OIDC provider ARN used for RRSA / OIDC role assumption.")
	f.StringVar(&cfg.OIDCTokenFile, prefix+"aliyun.oidc-token-file", "", "Path to the OIDC token file used for RRSA / OIDC role assumption.")
	f.StringVar(&cfg.RoleSessionName, prefix+"aliyun.role-session-name", "", "Role session name used for RRSA / OIDC role assumption. If empty, Cortex will generate one.")
	f.Var(&cfg.Policy, prefix+"aliyun.policy", "Optional inline policy used to further restrict permissions of the RRSA / OIDC assumed role session.")
	f.IntVar(&cfg.RoleSessionExpiry, prefix+"aliyun.role-session-expiration", 3600, "RRSA / OIDC role session duration in seconds.")
	f.StringVar(&cfg.STSEndpoint, prefix+"aliyun.sts-endpoint", "", "Aliyun STS endpoint used for RRSA / OIDC role assumption. Defaults to sts.aliyuncs.com or sts.<region>.aliyuncs.com when region is set.")
	f.StringVar(&cfg.BucketName, prefix+"aliyun.bucket-name", "", "Aliyun OSS bucket name")
	f.StringVar(&cfg.Region, prefix+"aliyun.region", "", "Aliyun OSS region")
	f.BoolVar(&cfg.DisableDualstack, prefix+"aliyun.disable-dualstack", false, "If enabled, the client will use the non-dualstack endpoint variant.")
	f.StringVar(&cfg.Endpoint, prefix+"aliyun.endpoint", "", "The Aliyun OSS endpoint in hostname:port format.")
	f.BoolVar(&cfg.Insecure, prefix+"aliyun.insecure", false, "If enabled, use http:// for the OSS endpoint instead of https://.")
	f.StringVar(&cfg.SignatureVersion, prefix+"aliyun.signature-version", s3.SignatureVersionV4, "The signature version to use for authenticating against OSS.")
	f.StringVar(&cfg.BucketLookupType, prefix+"aliyun.bucket-lookup-type", s3.BucketAutoLookup, "The OSS bucket lookup style.")
	f.BoolVar(&cfg.SendContentMd5, prefix+"aliyun.send-content-md5", true, "If true, attach MD5 checksum when uploading objects.")
	f.StringVar(&cfg.ListObjectsVersion, prefix+"aliyun.list-objects-version", "", "The list api version. Supported values are: v1, v2, and ''.")
	cfg.SSE.RegisterFlagsWithPrefix(prefix+"aliyun.sse.", f)
	cfg.HTTP.RegisterFlagsWithPrefix(prefix, f)
}

func (cfg *Config) Validate() error {
	if _, _, _, err := resolveCredentials(*cfg); err != nil {
		return err
	}

	s3Cfg := cfg.baseS3Config()
	return s3Cfg.Validate()
}

func (cfg *Config) toS3Config() (s3.Config, error) {
	accessKeyID, secretAccessKey, sessionToken, err := resolveCredentials(*cfg)
	if err != nil {
		return s3.Config{}, err
	}

	s3Cfg := cfg.baseS3Config()
	s3Cfg.SecretAccessKey = flagext.Secret{Value: secretAccessKey}
	s3Cfg.AccessKeyID = accessKeyID
	s3Cfg.SessionToken = flagext.Secret{Value: sessionToken}

	return s3Cfg, nil
}

func (cfg *Config) baseS3Config() s3.Config {
	return s3.Config{
		Endpoint:           cfg.Endpoint,
		Region:             cfg.Region,
		BucketName:         cfg.BucketName,
		DisableDualstack:   cfg.DisableDualstack,
		Insecure:           cfg.Insecure,
		SignatureVersion:   cfg.SignatureVersion,
		BucketLookupType:   cfg.BucketLookupType,
		SendContentMd5:     cfg.SendContentMd5,
		ListObjectsVersion: cfg.ListObjectsVersion,
		SSE:                cfg.SSE,
		HTTP: s3.HTTPConfig{
			Config:    cfg.HTTP.Config,
			Transport: cfg.HTTP.Transport,
		},
	}
}
