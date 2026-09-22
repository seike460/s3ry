// Package s3 wraps AWS SDK for Go v2 behind a region-aware Session that
// caches clients and transfer managers per bucket region.
package s3

import (
	"context"
	"errors"
	"net"
	stdhttp "net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	defaultConcurrency = 8
	defaultPartSize    = 8 * 1024 * 1024
	minimumPartSize    = 5 * 1024 * 1024
)

// Options controls how a Session loads AWS configuration and creates S3
// clients.
type Options struct {
	Profile, Region, EndpointURL string
	PathStyle, NoSignRequest     bool
	Concurrency                  int
	PartSize                     int64
	HTTPClient                   config.HTTPClient
}

// Session owns the AWS configuration used by every operation. Per-region
// client, transfer-manager, and bucket-region caches live in clientCache;
// this type holds only configuration and session-wide flags.
type Session struct {
	cfg          aws.Config
	opts         Options
	cache        *clientCache
	singleDelete atomic.Bool
}

// HTTP transport tuning for the default client. The idle pool scales with
// the configured concurrency so parallel transfers reuse connections.
const (
	baselineIdleConns = 64
	idleConnHeadroom  = 16
	idleConnTimeout   = 90 * time.Second
)

// NewSession loads the AWS configuration and prepares the client caches.
func NewSession(ctx context.Context, opts Options) (*Session, error) {
	if opts.Concurrency <= 0 {
		opts.Concurrency = defaultConcurrency
	}
	if opts.PartSize <= 0 {
		opts.PartSize = defaultPartSize
	}
	if opts.PartSize < minimumPartSize {
		return nil, &Error{
			Kind: KindInvalid,
			Op:   "session",
			Err:  errors.New("part size must be at least 5 MiB"),
		}
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		maxIdleConns := max(baselineIdleConns, 2*opts.Concurrency+idleConnHeadroom)
		httpClient = awshttp.NewBuildableClient().WithTransportOptions(func(tr *stdhttp.Transport) {
			tr.MaxIdleConns = maxIdleConns
			tr.MaxIdleConnsPerHost = maxIdleConns
			tr.IdleConnTimeout = idleConnTimeout
		})
	}

	loadOptions := []func(*config.LoadOptions) error{
		config.WithHTTPClient(httpClient),
	}
	if opts.Profile != "" {
		loadOptions = append(loadOptions, config.WithSharedConfigProfile(opts.Profile))
	}
	if opts.Region != "" {
		loadOptions = append(loadOptions, config.WithRegion(opts.Region))
	}
	if opts.EndpointURL != "" {
		loadOptions = append(loadOptions, config.WithBaseEndpoint(opts.EndpointURL))
	}
	if opts.NoSignRequest {
		loadOptions = append(loadOptions, config.WithCredentialsProvider(aws.AnonymousCredentials{}))
	}

	cfg, err := config.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, err
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	baseEndpoint := awss3.NewFromConfig(cfg).Options().BaseEndpoint
	endpointMode := baseEndpoint != nil
	endpoint := ""
	if endpointMode {
		endpoint = aws.ToString(baseEndpoint)
	}
	pathStyleOpts := opts
	pathStyleOpts.EndpointURL = endpoint

	return &Session{
		cfg:  cfg,
		opts: opts,
		cache: newClientCache(cfg,
			effectivePathStyle(pathStyleOpts), endpointMode,
			opts.PartSize, opts.Concurrency),
	}, nil
}

// effectivePathStyle enables path-style addressing for an explicitly
// configured local or IP-literal endpoint, unless the caller already selected
// a path-style mode explicitly.
func effectivePathStyle(opts Options) bool {
	if opts.PathStyle {
		return true
	}
	if opts.EndpointURL == "" {
		return false
	}

	endpoint, err := url.Parse(opts.EndpointURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(endpoint.Hostname())
	return host == "localhost" || net.ParseIP(host) != nil
}

// Options returns the normalized session options.
func (s *Session) Options() Options {
	return s.opts
}

// Region returns the default region used by the session.
func (s *Session) Region() string {
	return s.cfg.Region
}

// CheckCredentials verifies that the configured credential provider can
// return usable credentials before an S3 operation is started.
func (s *Session) CheckCredentials(ctx context.Context) error {
	if s.opts.NoSignRequest {
		return nil
	}
	if s.cfg.Credentials == nil {
		return &Error{
			Kind: KindNoCredentials,
			Op:   "credentials",
			Err:  errors.New("credentials provider is not configured"),
		}
	}

	credentials, err := s.cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return &Error{Kind: KindNoCredentials, Op: "credentials", Err: err}
	}
	if !credentials.HasKeys() {
		return &Error{
			Kind: KindNoCredentials,
			Op:   "credentials",
			Err:  errors.New("retrieved credentials are empty"),
		}
	}
	return nil
}

// BucketRegion resolves a bucket's region once and keeps it in the session
// cache.
func (s *Session) BucketRegion(ctx context.Context, bucket string) (string, error) {
	return s.cache.bucketRegion(ctx, bucket)
}

// client returns the cached client for bucket's region.
func (s *Session) client(ctx context.Context, bucket string) (*awss3.Client, error) {
	return s.cache.client(ctx, bucket)
}

// transfer returns the cached transfer manager for bucket's region.
func (s *Session) transfer(ctx context.Context, bucket string) (*transfermanager.Client, error) {
	return s.cache.transfer(ctx, bucket)
}
