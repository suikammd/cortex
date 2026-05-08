package oss

import (
	"flag"

	"github.com/cortexproject/cortex/pkg/util/flagext"
)

// Config holds the config options for OSS backend
type Config struct {
	Endpoint          string         `yaml:"endpoint"`
	BucketName        string         `yaml:"bucket_name"`
	AccessKeyID       string         `yaml:"access_key_id"`
	SecretAccessKey   flagext.Secret `yaml:"secret_access_key"`
	RoleArn           string         `yaml:"role_arn"`
	OIDCProviderArn   string         `yaml:"oidc_provider_arn"`
	OIDCTokenFilePath string         `yaml:"oidc_token_file_path"`
	RoleSessionName   string         `yaml:"role_session_name"`
}

// RegisterFlags registers the flags for OSS storage
func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	cfg.RegisterFlagsWithPrefix("", f)
}

// RegisterFlagsWithPrefix registers the flags for OSS storage with the provided prefix
func (cfg *Config) RegisterFlagsWithPrefix(prefix string, f *flag.FlagSet) {
	f.StringVar(&cfg.Endpoint, prefix+"oss.endpoint", "", "OSS endpoint")
	f.StringVar(&cfg.BucketName, prefix+"oss.bucket-name", "", "OSS bucket name")
	f.StringVar(&cfg.AccessKeyID, prefix+"oss.access-key-id", "", "OSS access key ID")
	f.Var(&cfg.SecretAccessKey, prefix+"oss.secret-access-key", "OSS secret access key")
	f.StringVar(&cfg.RoleArn, prefix+"oss.role-arn", "", "OSS Role ARN")
	f.StringVar(&cfg.OIDCProviderArn, prefix+"oss.oidc-provider-arn", "", "OSS OIDC provider ARN")
	f.StringVar(&cfg.OIDCTokenFilePath, prefix+"oss.oidc-token-file-path", "", "OSS OIDC token file path")
	f.StringVar(&cfg.RoleSessionName, prefix+"oss.role-session-name", "", "OSS role session name")
}
