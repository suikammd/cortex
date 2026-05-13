package oss

import (
	"flag"

	"github.com/cortexproject/cortex/pkg/util/flagext"
)

// Config holds the config options for OSS backend
type Config struct {
	Endpoint        string         `yaml:"endpoint"`
	Region          string         `yaml:"region"`
	BucketName      string         `yaml:"bucket_name"`
	AccessKeyID     string         `yaml:"access_key_id"`
	SecretAccessKey flagext.Secret `yaml:"secret_access_key"`
}

// RegisterFlags registers the flags for OSS storage
func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	cfg.RegisterFlagsWithPrefix("", f)
}

// RegisterFlagsWithPrefix registers the flags for OSS storage with the provided prefix
func (cfg *Config) RegisterFlagsWithPrefix(prefix string, f *flag.FlagSet) {
	f.StringVar(&cfg.Endpoint, prefix+"oss.endpoint", "", "OSS endpoint")
	f.StringVar(&cfg.Region, prefix+"oss.region", "", "OSS region")
	f.StringVar(&cfg.BucketName, prefix+"oss.bucket-name", "", "OSS bucket name")
	f.StringVar(&cfg.AccessKeyID, prefix+"oss.access-key-id", "", "OSS access key ID")
	f.Var(&cfg.SecretAccessKey, prefix+"oss.secret-access-key", "OSS secret access key")
}
