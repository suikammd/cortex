package aliyun

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pkg/errors"
)

type rrsaConfig struct {
	RoleArn           string
	OIDCProviderArn   string
	OIDCTokenFile     string
	RoleSessionName   string
	Policy            string
	RoleSessionExpiry int
	STSEndpoint       string
}

type rrsaCredentialsProvider struct {
	credentials.Expiry

	client *http.Client
	cfg    rrsaConfig
}

type assumeRoleWithOIDCResponse struct {
	Credentials struct {
		AccessKeyID     string `json:"AccessKeyId"`
		AccessKeySecret string `json:"AccessKeySecret"`
		SecurityToken   string `json:"SecurityToken"`
		Expiration      string `json:"Expiration"`
	} `json:"Credentials"`
}

func (cfg Config) rrsaConfig() (rrsaConfig, bool, error) {
	roleArn := firstNonEmpty(cfg.RoleArn, os.Getenv("ALIBABA_CLOUD_ROLE_ARN"))
	oidcProviderArn := firstNonEmpty(cfg.OIDCProviderArn, os.Getenv("ALIBABA_CLOUD_OIDC_PROVIDER_ARN"))
	oidcTokenFile := firstNonEmpty(cfg.OIDCTokenFile, os.Getenv("ALIBABA_CLOUD_OIDC_TOKEN_FILE"))
	roleSessionName := firstNonEmpty(cfg.RoleSessionName, os.Getenv("ALIBABA_CLOUD_ROLE_SESSION_NAME"))

	if roleArn == "" && oidcProviderArn == "" && oidcTokenFile == "" {
		return rrsaConfig{}, false, nil
	}
	if roleArn == "" || oidcProviderArn == "" || oidcTokenFile == "" {
		return rrsaConfig{}, false, errors.New("incomplete RRSA configuration: role_arn, oidc_provider_arn and oidc_token_file are all required")
	}

	roleSessionExpiry := cfg.RoleSessionExpiry
	if roleSessionExpiry == 0 {
		roleSessionExpiry = 3600
	}
	if roleSessionExpiry < 900 || roleSessionExpiry > 43200 {
		return rrsaConfig{}, false, errors.New("invalid role_session_expiration: expected a value between 900 and 43200 seconds")
	}

	if roleSessionName == "" {
		roleSessionName = fmt.Sprintf("cortex-%d", time.Now().Unix())
	}

	stsEndpoint := firstNonEmpty(cfg.STSEndpoint, deriveSTSEndpoint(cfg.Region))

	return rrsaConfig{
		RoleArn:           roleArn,
		OIDCProviderArn:   oidcProviderArn,
		OIDCTokenFile:     oidcTokenFile,
		RoleSessionName:   roleSessionName,
		Policy:            cfg.Policy.Value,
		RoleSessionExpiry: roleSessionExpiry,
		STSEndpoint:       stsEndpoint,
	}, true, nil
}

func newRRSACredentialsProvider(cfg rrsaConfig) credentials.Provider {
	return &rrsaCredentialsProvider{
		client: http.DefaultClient,
		cfg:    cfg,
	}
}

func (p *rrsaCredentialsProvider) RetrieveWithCredContext(cc *credentials.CredContext) (credentials.Value, error) {
	client := p.client
	if cc != nil && cc.Client != nil {
		client = cc.Client
	}

	token, err := os.ReadFile(p.cfg.OIDCTokenFile)
	if err != nil {
		return credentials.Value{}, errors.Wrap(err, "read RRSA OIDC token file")
	}

	values := url.Values{}
	values.Set("Action", "AssumeRoleWithOIDC")
	values.Set("Format", "JSON")
	values.Set("Version", "2015-04-01")
	values.Set("RoleArn", p.cfg.RoleArn)
	values.Set("OIDCProviderArn", p.cfg.OIDCProviderArn)
	values.Set("OIDCToken", strings.TrimSpace(string(token)))
	values.Set("RoleSessionName", p.cfg.RoleSessionName)
	values.Set("DurationSeconds", strconv.Itoa(p.cfg.RoleSessionExpiry))
	if p.cfg.Policy != "" {
		values.Set("Policy", p.cfg.Policy)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, buildSTSURL(p.cfg.STSEndpoint), strings.NewReader(values.Encode()))
	if err != nil {
		return credentials.Value{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return credentials.Value{}, errors.Wrap(err, "request RRSA credentials from STS")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return credentials.Value{}, errors.Wrap(err, "read RRSA credentials response")
	}
	if resp.StatusCode/100 != 2 {
		return credentials.Value{}, errors.Errorf("RRSA STS request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed assumeRoleWithOIDCResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return credentials.Value{}, errors.Wrap(err, "decode RRSA credentials response")
	}

	expiration, err := time.Parse(time.RFC3339, parsed.Credentials.Expiration)
	if err != nil {
		return credentials.Value{}, errors.Wrap(err, "parse RRSA credentials expiration")
	}
	p.SetExpiration(expiration, credentials.DefaultExpiryWindow)

	return credentials.Value{
		AccessKeyID:     parsed.Credentials.AccessKeyID,
		SecretAccessKey: parsed.Credentials.AccessKeySecret,
		SessionToken:    parsed.Credentials.SecurityToken,
		SignerType:      credentials.SignatureV4,
		Expiration:      expiration,
	}, nil
}

func (p *rrsaCredentialsProvider) Retrieve() (credentials.Value, error) {
	return p.RetrieveWithCredContext(nil)
}

func deriveSTSEndpoint(region string) string {
	if region == "" {
		return "sts.aliyuncs.com"
	}

	return "sts." + region + ".aliyuncs.com"
}

func buildSTSURL(endpoint string) string {
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}

	return "https://" + endpoint
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}
