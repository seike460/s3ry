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
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	transfertypes "github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager/types"
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

// Session owns the AWS configuration and the clients used for each bucket
// region.
type Session struct {
	cfg          aws.Config
	opts         Options
	endpoint     string
	endpointMode bool
	pathStyle    bool
	mu           sync.Mutex
	clients      map[string]*awss3.Client
	tms          map[string]*transfermanager.Client
	regions      map[string]string
	singleDelete atomic.Bool
}

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
		maxIdleConns := max(64, 2*opts.Concurrency+16)
		httpClient = awshttp.NewBuildableClient().WithTransportOptions(func(tr *stdhttp.Transport) {
			tr.MaxIdleConns = maxIdleConns
			tr.MaxIdleConnsPerHost = maxIdleConns
			tr.IdleConnTimeout = 90 * time.Second
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

	defaultClient := awss3.NewFromConfig(cfg)
	baseEndpoint := defaultClient.Options().BaseEndpoint
	endpointMode := baseEndpoint != nil
	endpoint := ""
	if endpointMode {
		endpoint = aws.ToString(baseEndpoint)
	}
	pathStyleOpts := opts
	pathStyleOpts.EndpointURL = endpoint

	return &Session{
		cfg:          cfg,
		opts:         opts,
		endpoint:     endpoint,
		endpointMode: endpointMode,
		pathStyle:    effectivePathStyle(pathStyleOpts),
		clients:      make(map[string]*awss3.Client),
		tms:          make(map[string]*transfermanager.Client),
		regions:      make(map[string]string),
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
// cache. Custom endpoints are single-region endpoints, so they never need a
// HeadBucket probe.
func (s *Session) BucketRegion(ctx context.Context, bucket string) (string, error) {
	if region, ok := s.cachedRegion(bucket); ok {
		return region, nil
	}

	if s.endpointMode {
		region := s.cfg.Region
		s.cacheRegion(bucket, region)
		return region, nil
	}

	client := s.clientForRegion(s.cfg.Region)
	out, err := client.HeadBucket(ctx, &awss3.HeadBucketInput{Bucket: aws.String(bucket)})
	if err == nil {
		region := aws.ToString(out.BucketRegion)
		if region == "" {
			region = s.cfg.Region
		}
		s.cacheRegion(bucket, region)
		return region, nil
	}

	if region := regionFromError(err); region != "" {
		s.cacheRegion(bucket, region)
		return region, nil
	}

	return "", Classify("head", bucket, "", err)
}

func regionFromError(err error) string {
	var responseErr *awshttp.ResponseError
	if !errors.As(err, &responseErr) || responseErr == nil {
		return ""
	}
	// The embedded *smithyhttp.ResponseError may be nil; promoted field
	// access through a nil embedded pointer panics, so check it directly.
	embedded := responseErr.ResponseError
	if embedded == nil || embedded.Response == nil || embedded.Response.Response == nil {
		return ""
	}

	response := responseErr.HTTPResponse()
	if response == nil || response.Response == nil {
		return ""
	}
	return response.Header.Get("x-amz-bucket-region")
}

// regionFor resolves the bucket's region. Custom endpoints are single-region
// endpoints, so they skip the HeadBucket probe entirely.
func (s *Session) regionFor(ctx context.Context, bucket string) (string, error) {
	if s.endpointMode {
		return s.cfg.Region, nil
	}
	return s.BucketRegion(ctx, bucket)
}

// client returns the cached client for bucket's region, creating it exactly
// once per region while holding the session mutex.
func (s *Session) client(ctx context.Context, bucket string) (*awss3.Client, error) {
	region, err := s.regionFor(ctx, bucket)
	if err != nil {
		return nil, err
	}
	return s.clientForRegion(region), nil
}

func (s *Session) clientForRegion(region string) *awss3.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clientForRegionLocked(region)
}

// clientForRegionLocked returns the cached client for region. Callers must
// hold s.mu.
func (s *Session) clientForRegionLocked(region string) *awss3.Client {
	if client := s.clients[region]; client != nil {
		return client
	}

	client := awss3.NewFromConfig(s.cfg, func(o *awss3.Options) {
		o.Region = region
		o.UsePathStyle = s.pathStyle
	})
	s.clients[region] = client
	return client
}

func (s *Session) transferOptions() func(*transfermanager.Options) {
	return func(o *transfermanager.Options) {
		o.PartSizeBytes = s.opts.PartSize
		o.Concurrency = s.opts.Concurrency
		o.GetObjectType = transfertypes.GetObjectRanges
		o.FailTimeout = 30 * time.Second
	}
}

// transfer returns the cached transfer manager for bucket's region.
func (s *Session) transfer(ctx context.Context, bucket string) (*transfermanager.Client, error) {
	region, err := s.regionFor(ctx, bucket)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if manager := s.tms[region]; manager != nil {
		return manager, nil
	}

	manager := transfermanager.New(s.clientForRegionLocked(region), s.transferOptions())
	s.tms[region] = manager
	return manager, nil
}

func (s *Session) cachedRegion(bucket string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	region, ok := s.regions[bucket]
	return region, ok
}

func (s *Session) cacheRegion(bucket, region string) {
	s.mu.Lock()
	s.regions[bucket] = region
	s.mu.Unlock()
}
