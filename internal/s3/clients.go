package s3

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	transfertypes "github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager/types"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

// clientCache owns the session's per-region caches: resolved bucket regions,
// S3 clients, and transfer managers. All maps are guarded by mu and entries
// are created exactly once per key.
type clientCache struct {
	cfg       aws.Config
	pathStyle bool
	endpoint  bool // custom endpoint: single-region, skips HeadBucket probes
	partSize  int64
	parallel  int

	mu      sync.Mutex
	clients map[string]*awss3.Client
	tms     map[string]*transfermanager.Client
	regions map[string]string
}

func newClientCache(cfg aws.Config, pathStyle, endpoint bool, partSize int64, parallel int) *clientCache {
	return &clientCache{
		cfg:       cfg,
		pathStyle: pathStyle,
		endpoint:  endpoint,
		partSize:  partSize,
		parallel:  parallel,
		clients:   make(map[string]*awss3.Client),
		tms:       make(map[string]*transfermanager.Client),
		regions:   make(map[string]string),
	}
}

// bucketRegion resolves a bucket's region once and keeps it in the cache.
// Custom endpoints are single-region endpoints, so they never need a
// HeadBucket probe.
func (c *clientCache) bucketRegion(ctx context.Context, bucket string) (string, error) {
	if region, ok := c.cachedRegion(bucket); ok {
		return region, nil
	}

	if c.endpoint {
		region := c.cfg.Region
		c.cacheRegion(bucket, region)
		return region, nil
	}

	client := c.clientForRegion(c.cfg.Region)
	out, err := client.HeadBucket(ctx, &awss3.HeadBucketInput{Bucket: aws.String(bucket)})
	if err == nil {
		region := aws.ToString(out.BucketRegion)
		if region == "" {
			region = c.cfg.Region
		}
		c.cacheRegion(bucket, region)
		return region, nil
	}

	if region := regionFromError(err); region != "" {
		c.cacheRegion(bucket, region)
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
func (c *clientCache) regionFor(ctx context.Context, bucket string) (string, error) {
	if c.endpoint {
		return c.cfg.Region, nil
	}
	return c.bucketRegion(ctx, bucket)
}

// client returns the cached client for bucket's region, creating it exactly
// once per region while holding the cache mutex.
func (c *clientCache) client(ctx context.Context, bucket string) (*awss3.Client, error) {
	region, err := c.regionFor(ctx, bucket)
	if err != nil {
		return nil, err
	}
	return c.clientForRegion(region), nil
}

func (c *clientCache) clientForRegion(region string) *awss3.Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.clientForRegionLocked(region)
}

// clientForRegionLocked returns the cached client for region. Callers must
// hold c.mu.
func (c *clientCache) clientForRegionLocked(region string) *awss3.Client {
	if client := c.clients[region]; client != nil {
		return client
	}

	client := awss3.NewFromConfig(c.cfg, func(o *awss3.Options) {
		o.Region = region
		o.UsePathStyle = c.pathStyle
	})
	c.clients[region] = client
	return client
}

// transferFailTimeout is how long the transfer manager waits for a
// failed part before aborting a multipart transfer.
const transferFailTimeout = 30 * time.Second

func (c *clientCache) transferOptions() func(*transfermanager.Options) {
	return func(o *transfermanager.Options) {
		o.PartSizeBytes = c.partSize
		o.Concurrency = c.parallel
		o.GetObjectType = transfertypes.GetObjectRanges
		o.FailTimeout = transferFailTimeout
	}
}

// transfer returns the cached transfer manager for bucket's region.
func (c *clientCache) transfer(ctx context.Context, bucket string) (*transfermanager.Client, error) {
	region, err := c.regionFor(ctx, bucket)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if manager := c.tms[region]; manager != nil {
		return manager, nil
	}

	manager := transfermanager.New(c.clientForRegionLocked(region), c.transferOptions())
	c.tms[region] = manager
	return manager, nil
}

func (c *clientCache) cachedRegion(bucket string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	region, ok := c.regions[bucket]
	return region, ok
}

func (c *clientCache) cacheRegion(bucket, region string) {
	c.mu.Lock()
	c.regions[bucket] = region
	c.mu.Unlock()
}
