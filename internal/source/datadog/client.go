package datadog

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/titlis/insights/internal/model"
	"github.com/titlis/insights/internal/source"
)

type Credential struct {
	Site   string
	APIKey string
	AppKey string
	EnvTag string
}

type CredentialProvider interface {
	ForTenant(ctx context.Context, tenantID int64) (Credential, error)
}

type Client struct {
	creds   CredentialProvider
	http    *http.Client
	timeout time.Duration
}

func NewClient(creds CredentialProvider) *Client {
	return &Client{
		creds:   creds,
		http:    &http.Client{Timeout: 30 * time.Second},
		timeout: 30 * time.Second,
	}
}

func (c *Client) Name() string { return "datadog" }

func (c *Client) UsageSnapshot(ctx context.Context, w model.WorkloadContext, windowDays int) (model.UsageSnapshot, error) {
	if !w.HasDatadog {
		return model.UsageSnapshot{}, source.ErrSourceUnavailable
	}
	cred, err := c.creds.ForTenant(ctx, w.TenantID)
	if err != nil {
		return model.UsageSnapshot{}, fmt.Errorf("credentials: %w", err)
	}
	if cred.APIKey == "" || cred.AppKey == "" {
		return model.UsageSnapshot{}, source.ErrSourceUnavailable
	}
	site := cred.Site
	if site == "" {
		site = "datadoghq.com"
	}

	now := time.Now()
	from := now.AddDate(0, 0, -windowDays).Unix()
	to := now.Unix()

	cpuP95, cpuP99, cpuSamples, err := c.queryCPU(ctx, cred, site, w.DeploymentName, w.Namespace, from, to)
	if err != nil {
		return model.UsageSnapshot{}, fmt.Errorf("query cpu: %w", err)
	}

	replicaPeak, replicaAvg, err := c.queryReplicas(ctx, cred, site, w.DeploymentName, w.Namespace, from, to)
	if err != nil {
		return model.UsageSnapshot{}, fmt.Errorf("query replicas: %w", err)
	}

	return model.UsageSnapshot{
		WindowDays:         windowDays,
		CPUP95Pct:          cpuP95,
		CPUP99Pct:          cpuP99,
		ReplicasPeak:       replicaPeak,
		ReplicasAverage:    replicaAvg,
		SamplesCount:       cpuSamples,
		ConfidenceFraction: computeConfidence(cpuSamples, windowDays),
	}, nil
}

// queryCPU returns (P95%, P99%, sampleCount, error).
// Uses kubernetes.cpu.usage.total (nanocores) divided by kubernetes.cpu.limits (nanocores).
func (c *Client) queryCPU(ctx context.Context, cred Credential, site, deployment, namespace string, from, to int64) (p95, p99 float64, samples int, err error) {
	tags := fmt.Sprintf("kube_deployment:%s,kube_namespace:%s", deployment, namespace)
	usageQuery := fmt.Sprintf("avg:kubernetes.cpu.usage.total{%s}", tags)
	usagePoints, err := c.queryTimeSeries(ctx, cred, site, from, to, usageQuery)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("usage: %w", err)
	}
	if len(usagePoints) == 0 {
		return 0, 0, 0, source.ErrSourceUnavailable
	}

	limitsQuery := fmt.Sprintf("avg:kubernetes.cpu.limits{%s}", tags)
	limitsPoints, err := c.queryTimeSeries(ctx, cred, site, from, to, limitsQuery)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("limits: %w", err)
	}
	if len(limitsPoints) == 0 {
		return 0, 0, 0, source.ErrSourceUnavailable
	}

	var totalLimits float64
	for _, v := range limitsPoints {
		totalLimits += v
	}
	avgLimits := totalLimits / float64(len(limitsPoints))
	if avgLimits <= 0 {
		return 0, 0, 0, source.ErrSourceUnavailable
	}

	pcts := make([]float64, 0, len(usagePoints))
	for _, v := range usagePoints {
		if v >= 0 {
			pcts = append(pcts, (v/avgLimits)*100)
		}
	}
	if len(pcts) == 0 {
		return 0, 0, 0, source.ErrSourceUnavailable
	}
	sort.Float64s(pcts)
	return percentile(pcts, 0.95), percentile(pcts, 0.99), len(pcts), nil
}

// queryReplicas returns (peak, average, error).
func (c *Client) queryReplicas(ctx context.Context, cred Credential, site, deployment, namespace string, from, to int64) (peak int, avg float64, err error) {
	tags := fmt.Sprintf("kube_deployment:%s,kube_namespace:%s", deployment, namespace)
	query := fmt.Sprintf("sum:kubernetes.pods.running{%s}", tags)
	points, err := c.queryTimeSeries(ctx, cred, site, from, to, query)
	if err != nil {
		return 0, 0, fmt.Errorf("replicas: %w", err)
	}
	if len(points) == 0 {
		return 0, 0, source.ErrSourceUnavailable
	}
	var maxVal, totalVal float64
	for _, v := range points {
		if v > maxVal {
			maxVal = v
		}
		totalVal += v
	}
	return int(math.Ceil(maxVal)), totalVal / float64(len(points)), nil
}

type ddQueryResponse struct {
	Status string `json:"status"`
	Series []struct {
		Pointlist [][2]*float64 `json:"pointlist"`
	} `json:"series"`
}

func (c *Client) queryTimeSeries(ctx context.Context, cred Credential, site string, from, to int64, query string) ([]float64, error) {
	apiURL := fmt.Sprintf("https://api.%s/api/v1/query?from=%d&to=%d&query=%s",
		site, from, to, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("DD-API-KEY", cred.APIKey)
	req.Header.Set("DD-APPLICATION-KEY", cred.AppKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return nil, fmt.Errorf("rate limited: %w", source.ErrSourceUnavailable)
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, fmt.Errorf("auth error %d: %w", resp.StatusCode, source.ErrSourceUnavailable)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("datadog api %d", resp.StatusCode)
	}

	var result ddQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if result.Status != "ok" {
		return nil, fmt.Errorf("datadog status: %s", result.Status)
	}
	if len(result.Series) == 0 {
		return nil, nil
	}
	points := make([]float64, 0, len(result.Series[0].Pointlist))
	for _, pt := range result.Series[0].Pointlist {
		if pt[1] != nil {
			points = append(points, *pt[1])
		}
	}
	return points, nil
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(p * float64(len(sorted)-1))
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// computeConfidence estimates quality based on sample count vs expected samples.
// Assumes ~288 data points per day (5-minute resolution).
func computeConfidence(samples, windowDays int) float64 {
	if samples == 0 {
		return 0
	}
	expected := windowDays * 288
	ratio := float64(samples) / float64(expected)
	if ratio > 1 {
		ratio = 1
	}
	return ratio
}

// Probe calls the Datadog validate endpoint to verify that the tenant's
// API key and application key are functional.
// Returns (true, "ok", nil) on success; (false, reason, nil) on auth failure.
func (c *Client) Probe(ctx context.Context, tenantID int64) (ok bool, reason string, err error) {
	cred, credErr := c.creds.ForTenant(ctx, tenantID)
	if credErr != nil {
		return false, "credential_error", fmt.Errorf("credentials: %w", credErr)
	}
	if cred.APIKey == "" {
		return false, "no_api_key", nil
	}
	site := cred.Site
	if site == "" {
		site = "datadoghq.com"
	}
	validateURL := fmt.Sprintf("https://api.%s/api/v1/validate", site)
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, validateURL, nil)
	if reqErr != nil {
		return false, "request_build_failed", reqErr
	}
	req.Header.Set("DD-API-KEY", cred.APIKey)
	if cred.AppKey != "" {
		req.Header.Set("DD-APPLICATION-KEY", cred.AppKey)
	}
	resp, doErr := c.http.Do(req)
	if doErr != nil {
		return false, "request_failed", doErr
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return true, "ok", nil
	case http.StatusForbidden, http.StatusUnauthorized:
		return false, "invalid_credentials", nil
	default:
		return false, fmt.Sprintf("http_%d", resp.StatusCode), nil
	}
}

type StaticCredentialProvider struct {
	Site   string
	APIKey string
	AppKey string
	EnvTag string
}

func (s StaticCredentialProvider) ForTenant(_ context.Context, _ int64) (Credential, error) {
	return Credential{Site: s.Site, APIKey: s.APIKey, AppKey: s.AppKey, EnvTag: s.EnvTag}, nil
}
