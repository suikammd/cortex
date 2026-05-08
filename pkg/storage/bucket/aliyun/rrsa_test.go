package aliyun

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/require"
)

func TestRRSAConfig_FromEnvironment(t *testing.T) {
	t.Setenv("ALIBABA_CLOUD_ROLE_ARN", "acs:ram::123:role/test")
	t.Setenv("ALIBABA_CLOUD_OIDC_PROVIDER_ARN", "acs:ram::123:oidc-provider/test")
	t.Setenv("ALIBABA_CLOUD_OIDC_TOKEN_FILE", "/tmp/token")
	t.Setenv("ALIBABA_CLOUD_ROLE_SESSION_NAME", "session-from-env")

	cfg, ok, err := (Config{Region: "cn-hangzhou"}).rrsaConfig()
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "acs:ram::123:role/test", cfg.RoleArn)
	require.Equal(t, "acs:ram::123:oidc-provider/test", cfg.OIDCProviderArn)
	require.Equal(t, "/tmp/token", cfg.OIDCTokenFile)
	require.Equal(t, "session-from-env", cfg.RoleSessionName)
	require.Equal(t, "sts.cn-hangzhou.aliyuncs.com", cfg.STSEndpoint)
	require.Equal(t, 3600, cfg.RoleSessionExpiry)
}

func TestRRSAProvider_RetrieveWithCredContext(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "token")
	require.NoError(t, os.WriteFile(tokenFile, []byte("test-oidc-token\n"), 0o600))

	provider := newRRSACredentialsProvider(rrsaConfig{
		RoleArn:           "acs:ram::123:role/test",
		OIDCProviderArn:   "acs:ram::123:oidc-provider/test",
		OIDCTokenFile:     tokenFile,
		RoleSessionName:   "cortex-test",
		RoleSessionExpiry: 3600,
		STSEndpoint:       "sts.cn-hangzhou.aliyuncs.com",
	}).(*rrsaCredentialsProvider)

	var got url.Values
	provider.client = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			got, err = url.ParseQuery(string(body))
			require.NoError(t, err)

			respBody := `{"Credentials":{"AccessKeyId":"rrsa-ak","AccessKeySecret":"rrsa-sk","SecurityToken":"rrsa-token","Expiration":"2030-01-02T03:04:05Z"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(respBody)),
			}, nil
		}),
	}

	value, err := provider.RetrieveWithCredContext(&credentials.CredContext{Client: provider.client})
	require.NoError(t, err)
	require.Equal(t, "rrsa-ak", value.AccessKeyID)
	require.Equal(t, "rrsa-sk", value.SecretAccessKey)
	require.Equal(t, "rrsa-token", value.SessionToken)
	require.Equal(t, "AssumeRoleWithOIDC", got.Get("Action"))
	require.Equal(t, "acs:ram::123:role/test", got.Get("RoleArn"))
	require.Equal(t, "acs:ram::123:oidc-provider/test", got.Get("OIDCProviderArn"))
	require.Equal(t, "test-oidc-token", got.Get("OIDCToken"))
	require.Equal(t, "cortex-test", got.Get("RoleSessionName"))
	require.Equal(t, "3600", got.Get("DurationSeconds"))
	require.False(t, provider.IsExpired())
}

func TestResolveCredentials_PrefersRRSABeforeError(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "token")
	require.NoError(t, os.WriteFile(tokenFile, []byte("test-oidc-token"), 0o600))

	accessKeyID, secretAccessKey, sessionToken, err := resolveCredentials(Config{
		RoleArn:           "acs:ram::123:role/test",
		OIDCProviderArn:   "acs:ram::123:oidc-provider/test",
		OIDCTokenFile:     tokenFile,
		RoleSessionName:   "cortex-test",
		RoleSessionExpiry: 3600,
	})
	require.NoError(t, err)
	require.Equal(t, "acs:ram::123:role/test", accessKeyID)
	require.Equal(t, "__rrsa__", secretAccessKey)
	require.Empty(t, sessionToken)
}

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
