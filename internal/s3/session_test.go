package s3

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	transfertypes "github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager/types"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

func TestNewSessionRejectsSmallPartSize(t *testing.T) {
	_, err := NewSession(context.Background(), Options{PartSize: 1 * 1024 * 1024})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("NewSession error = %v, want ErrInvalid", err)
	}
}

func TestNewSessionTunesTransport(t *testing.T) {
	setFakeEnvironment(t)

	for _, concurrency := range []int{8, 40} {
		t.Run("concurrency-"+strconv.Itoa(concurrency), func(t *testing.T) {
			session, err := NewSession(t.Context(), Options{
				Region:      "us-east-1",
				Concurrency: concurrency,
				HTTPClient:  nil,
			})
			if err != nil {
				t.Fatalf("NewSession: %v", err)
			}

			client, ok := session.cfg.HTTPClient.(*awshttp.BuildableClient)
			if !ok {
				t.Fatalf("cfg.HTTPClient = %T, want *awshttp.BuildableClient", session.cfg.HTTPClient)
			}
			transport := client.GetTransport()
			wantMaxIdleConns := max(64, 2*concurrency+16)
			if transport.MaxIdleConns != wantMaxIdleConns {
				t.Fatalf("MaxIdleConns = %d, want %d", transport.MaxIdleConns, wantMaxIdleConns)
			}
			if transport.MaxIdleConnsPerHost != wantMaxIdleConns {
				t.Fatalf("MaxIdleConnsPerHost = %d, want %d", transport.MaxIdleConnsPerHost, wantMaxIdleConns)
			}
			if transport.IdleConnTimeout != 90*time.Second {
				t.Fatalf("IdleConnTimeout = %s, want %s", transport.IdleConnTimeout, 90*time.Second)
			}
		})
	}
}

func TestSessionEndpointDoesNotProbeBucketRegion(t *testing.T) {
	var headRequests atomic.Int64
	var nonHeadRequests atomic.Int64
	session, _, _ := newCountingFake(t, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodHead {
				headRequests.Add(1)
			} else {
				nonHeadRequests.Add(1)
			}
			next.ServeHTTP(w, r)
		})
	})

	client, err := session.client(t.Context(), "bucket")
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	if !client.Options().UsePathStyle {
		t.Fatal("client.Options().UsePathStyle = false, want true for the fake endpoint")
	}
	region, err := session.BucketRegion(t.Context(), "bucket")
	if err != nil {
		t.Fatalf("BucketRegion: %v", err)
	}
	if region != "us-east-1" {
		t.Fatalf("BucketRegion = %q, want us-east-1", region)
	}
	if _, err := client.ListBuckets(t.Context(), &awss3.ListBucketsInput{}); err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}
	if got := headRequests.Load(); got != 0 {
		t.Fatalf("HEAD requests = %d, want 0", got)
	}
	if got := nonHeadRequests.Load(); got < 1 {
		t.Fatalf("non-HEAD requests = %d, want at least 1", got)
	}
}

func TestRegionFromError(t *testing.T) {
	responseError := func(region string) error {
		response := &http.Response{}
		if region != "" {
			response.Header = http.Header{"X-Amz-Bucket-Region": []string{region}}
		}
		return &awshttp.ResponseError{
			ResponseError: &smithyhttp.ResponseError{
				Response: &smithyhttp.Response{Response: response},
			},
		}
	}

	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "nil embed", err: &awshttp.ResponseError{}, want: ""},
		{name: "response with header", err: responseError("eu-west-1"), want: "eu-west-1"},
		{name: "response without header", err: responseError(""), want: ""},
		{name: "non-ResponseError", err: errors.New("not an HTTP response"), want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := regionFromError(tt.err); got != tt.want {
				t.Fatalf("regionFromError = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBucketRegionUsesRedirectRegionHeader(t *testing.T) {
	setFakeEnvironment(t)
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead || r.URL.Path != "/bucket" {
			t.Errorf("request = %s %s, want HEAD /bucket", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-Amz-Bucket-Region", "eu-west-1")
		w.WriteHeader(http.StatusMovedPermanently)
	}))

	session, err := NewSession(t.Context(), Options{
		Region:     "us-east-1",
		PathStyle:  true,
		HTTPClient: routeAllRequestsTo(server),
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	region, err := session.BucketRegion(t.Context(), "bucket")
	if err != nil {
		t.Fatalf("BucketRegion: %v", err)
	}
	if region != "eu-west-1" {
		t.Fatalf("BucketRegion = %q, want eu-west-1", region)
	}
}

func TestBucketRegionNotFound(t *testing.T) {
	setFakeEnvironment(t)
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead || r.URL.Path != "/missing" {
			t.Errorf("request = %s %s, want HEAD /missing", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNotFound)
	}))

	session, err := NewSession(t.Context(), Options{
		Region:     "us-east-1",
		PathStyle:  true,
		HTTPClient: routeAllRequestsTo(server),
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	_, err = session.BucketRegion(t.Context(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("BucketRegion error = %v, want ErrNotFound", err)
	}
}

func TestSessionProfileRegionPrecedence(t *testing.T) {
	setFakeEnvironment(t)
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_PROFILE", "")

	configPath := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(configPath, []byte("[profile it]\nregion = eu-central-1\n"), 0o600); err != nil {
		t.Fatalf("write AWS config: %v", err)
	}
	t.Setenv("AWS_CONFIG_FILE", configPath)

	tests := []struct {
		name       string
		region     string
		wantRegion string
	}{
		{name: "profile region", region: "", wantRegion: "eu-central-1"},
		{name: "explicit region", region: "ap-northeast-1", wantRegion: "ap-northeast-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, err := NewSession(t.Context(), Options{Profile: "it", Region: tt.region})
			if err != nil {
				t.Fatalf("NewSession: %v", err)
			}
			if got := session.Region(); got != tt.wantRegion {
				t.Fatalf("Session.Region = %q, want %q", got, tt.wantRegion)
			}
		})
	}
}

func TestSessionEnvironmentEndpointDoesNotProbeBucketRegion(t *testing.T) {
	var headRequests atomic.Int64
	session, _, _ := newCountingFake(t, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodHead {
				headRequests.Add(1)
			}
			next.ServeHTTP(w, r)
		})
	}, func(opts *Options) {
		t.Setenv("AWS_ENDPOINT_URL", opts.EndpointURL)
		opts.EndpointURL = ""
		opts.PathStyle = true
	})

	if session.Options().EndpointURL != "" {
		t.Fatalf("Options.EndpointURL = %q, want empty", session.Options().EndpointURL)
	}
	region, err := session.BucketRegion(t.Context(), "bucket")
	if err != nil {
		t.Fatalf("BucketRegion: %v", err)
	}
	if region != "us-east-1" {
		t.Fatalf("BucketRegion = %q, want us-east-1", region)
	}
	if got := headRequests.Load(); got != 0 {
		t.Fatalf("HEAD requests = %d, want 0", got)
	}
}

func TestCheckCredentialsWithoutCredentials(t *testing.T) {
	setFakeEnvironment(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_PROFILE", "")

	session, err := NewSession(t.Context(), Options{Region: "us-east-1"})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if err := session.CheckCredentials(t.Context()); !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("CheckCredentials error = %v, want ErrNoCredentials", err)
	}
}

func TestCheckCredentialsNoSignRequest(t *testing.T) {
	setFakeEnvironment(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")

	session, err := NewSession(t.Context(), Options{
		Region:        "us-east-1",
		NoSignRequest: true,
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if err := session.CheckCredentials(t.Context()); err != nil {
		t.Fatalf("CheckCredentials: %v", err)
	}
	if !aws.IsCredentialsProvider(session.cfg.Credentials, aws.AnonymousCredentials{}) {
		t.Fatalf("cfg.Credentials = %T, want anonymous credentials provider", session.cfg.Credentials)
	}
}

func TestTransferOptions(t *testing.T) {
	setFakeEnvironment(t)
	opts := Options{
		Region:      "us-east-1",
		PartSize:    16 * 1024 * 1024,
		Concurrency: 12,
	}
	session, err := NewSession(t.Context(), opts)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	got := transfermanager.Options{}
	session.cache.transferOptions()(&got)
	if got.PartSizeBytes != opts.PartSize {
		t.Fatalf("PartSizeBytes = %d, want %d", got.PartSizeBytes, opts.PartSize)
	}
	if got.Concurrency != opts.Concurrency {
		t.Fatalf("Concurrency = %d, want %d", got.Concurrency, opts.Concurrency)
	}
	if got.GetObjectType != transfertypes.GetObjectRanges {
		t.Fatalf("GetObjectType = %q, want %q", got.GetObjectType, transfertypes.GetObjectRanges)
	}
	if got.FailTimeout != 30*time.Second {
		t.Fatalf("FailTimeout = %s, want %s", got.FailTimeout, 30*time.Second)
	}
}

func TestTransferClientIsCached(t *testing.T) {
	var requests atomic.Int64
	session, _, _ := newCountingFake(t, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			next.ServeHTTP(w, r)
		})
	})

	first, err := session.transfer(t.Context(), "bucket")
	if err != nil {
		t.Fatalf("first transfer: %v", err)
	}
	second, err := session.transfer(t.Context(), "bucket")
	if err != nil {
		t.Fatalf("second transfer: %v", err)
	}
	if first != second {
		t.Fatalf("transfer clients differ: first %p, second %p", first, second)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("HTTP requests = %d, want 0", got)
	}
}

func TestEffectivePathStyle(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want bool
	}{
		{
			name: "ipv4 endpoint",
			opts: Options{EndpointURL: "http://127.0.0.1:1234"},
			want: true,
		},
		{
			name: "localhost endpoint",
			opts: Options{EndpointURL: "http://localhost:9000"},
			want: true,
		},
		{
			name: "ipv6 endpoint",
			opts: Options{EndpointURL: "http://[::1]:9000"},
			want: true,
		},
		{
			name: "dns endpoint",
			opts: Options{EndpointURL: "https://s3.example.com"},
			want: false,
		},
		{
			name: "explicit path style",
			opts: Options{EndpointURL: "https://s3.example.com", PathStyle: true},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := effectivePathStyle(tt.opts); got != tt.want {
				t.Fatalf("effectivePathStyle = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBucketRegionConcurrent(t *testing.T) {
	setFakeEnvironment(t)
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead || r.URL.Path != "/bucket" {
			t.Errorf("request = %s %s, want HEAD /bucket", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-Amz-Bucket-Region", "eu-west-1")
		w.WriteHeader(http.StatusMovedPermanently)
	}))

	session, err := NewSession(t.Context(), Options{
		Region:     "us-east-1",
		PathStyle:  true,
		HTTPClient: routeAllRequestsTo(server),
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	const callers = 10
	results := make(chan string, callers)
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Go(func() {
			region, err := session.BucketRegion(t.Context(), "bucket")
			if err != nil {
				errs <- err
				return
			}
			results <- region
		})
	}
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		t.Fatalf("BucketRegion: %v", err)
	}
	for region := range results {
		if region != "eu-west-1" {
			t.Fatalf("BucketRegion = %q, want eu-west-1", region)
		}
	}
	if got := len(session.cache.clients); got > 2 {
		t.Fatalf("client cache size = %d, want at most 2", got)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func routeAllRequestsTo(server *httptest.Server) *http.Client {
	transport := server.Client().Transport
	if transport == nil {
		panic("test server has no transport")
	}
	target, err := url.Parse(server.URL)
	if err != nil {
		panic(err)
	}
	return &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			request := r.Clone(r.Context())
			requestURL := *request.URL
			requestURL.Scheme = target.Scheme
			requestURL.Host = target.Host
			request.URL = &requestURL
			request.Host = target.Host
			return transport.RoundTrip(request)
		}),
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
