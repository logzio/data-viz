package es

import (
	"bytes"
	"context"
	"encoding/json"
	"errors" // LOGZ.IO GRAFANA CHANGE :: DEV-17927 - Add errors import
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	sdkhttpclient "github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/infra/httpclient"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/tsdb/intervalv2"
)

type DatasourceInfo struct {
	ID                         int64
	HTTPClientOpts             sdkhttpclient.Options
	URL                        string
	Database                   string
	ESVersion                  *semver.Version
	TimeField                  string
	Interval                   string
	TimeInterval               string
	MaxConcurrentShardRequests int64
	IncludeFrozen              bool
	XPack                      bool
}

const loggerName = "tsdb.elasticsearch.client"

var (
	clientLog = log.New(loggerName)
)

var newDatasourceHttpClient = func(httpClientProvider httpclient.Provider, ds *DatasourceInfo) (*http.Client, error) {
	return httpClientProvider.New(ds.HTTPClientOpts)
}

// Client represents a client which can interact with elasticsearch api
type Client interface {
	GetVersion() *semver.Version
	GetTimeField() string
	GetMinInterval(queryInterval string) (time.Duration, error)
	ExecuteMultisearch(r *MultiSearchRequest) (*MultiSearchResponse, error)
	MultiSearch() *MultiSearchRequestBuilder
	EnableDebug()
}

// NewClient creates a new elasticsearch client
var NewClient = func(ctx context.Context, httpClientProvider httpclient.Provider, ds *DatasourceInfo, timeRange backend.TimeRange) (Client, error) {
	ip, err := newIndexPattern(ds.Interval, ds.Database)
	if err != nil {
		return nil, err
	}

	indices, err := ip.GetIndices(timeRange)
	if err != nil {
		return nil, err
	}

	clientLog.Debug("Creating new client", "version", ds.ESVersion, "timeField", ds.TimeField, "indices", strings.Join(indices, ", "))

	// LOGZ.IO GRAFANA CHANGE :: Upgrade to 8.4.0 start
	logzIoHeaders := &models.LogzIoHeaders{}
	headers := ctx.Value("logzioHeaders")
	if headers != nil {
		logzIoHeaders.RequestHeaders = http.Header{}
		for key, value := range headers.(map[string]string) {
			logzIoHeaders.RequestHeaders.Set(key, value)
		}
	}
	// LOGZ.IO GRAFANA CHANGE :: Upgrade to 8.4.0 end

	return &baseClientImpl{
		ctx:                ctx,
		httpClientProvider: httpClientProvider,
		ds:                 ds,
		version:            ds.ESVersion,
		timeField:          ds.TimeField,
		indices:            indices,
		timeRange:          timeRange,
		logzIoHeaders:      logzIoHeaders, // LOGZ.IO GRAFANA CHANGE :: (ALERTS) DEV-16492 Support external alert evaluation
	}, nil
}

type baseClientImpl struct {
	ctx                context.Context
	httpClientProvider httpclient.Provider
	ds                 *DatasourceInfo
	version            *semver.Version
	timeField          string
	indices            []string
	timeRange          backend.TimeRange
	debugEnabled       bool
	logzIoHeaders      *models.LogzIoHeaders // LOGZ.IO GRAFANA CHANGE :: DEV-17927 - add LogzIoHeaders
}

func (c *baseClientImpl) GetVersion() *semver.Version {
	return c.version
}

func (c *baseClientImpl) GetTimeField() string {
	return c.timeField
}

func (c *baseClientImpl) GetMinInterval(queryInterval string) (time.Duration, error) {
	timeInterval := c.ds.TimeInterval
	return intervalv2.GetIntervalFrom(queryInterval, timeInterval, 0, 5*time.Second)
}

type multiRequest struct {
	header   map[string]interface{}
	body     interface{}
	interval intervalv2.Interval
}

func (c *baseClientImpl) executeBatchRequest(uriPath, uriQuery string, requests []*multiRequest) (*response, error) {
	bytes, err := c.encodeBatchRequests(requests)
	if err != nil {
		return nil, err
	}
	return c.executeRequest(http.MethodPost, uriPath, uriQuery, bytes)
}

func (c *baseClientImpl) encodeBatchRequests(requests []*multiRequest) ([]byte, error) {
	clientLog.Debug("Encoding batch requests to json", "batch requests", len(requests))
	start := time.Now()

	payload := bytes.Buffer{}
	for _, r := range requests {
		reqHeader, err := json.Marshal(r.header)
		if err != nil {
			return nil, err
		}
		payload.WriteString(string(reqHeader) + "\n")

		reqBody, err := json.Marshal(r.body)
		if err != nil {
			return nil, err
		}

		body := string(reqBody)
		body = strings.ReplaceAll(body, "$__interval_ms", strconv.FormatInt(r.interval.Milliseconds(), 10))
		body = strings.ReplaceAll(body, "$__interval", r.interval.Text)

		payload.WriteString(body + "\n")
	}

	elapsed := time.Since(start)
	clientLog.Debug("Encoded batch requests to json", "took", elapsed)

	return payload.Bytes(), nil
}

func (c *baseClientImpl) executeRequest(method, uriPath, uriQuery string, body []byte) (*response, error) {
	u, err := url.Parse(c.ds.URL)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, uriPath)
	u.RawQuery = uriQuery

	var req *http.Request
	if method == http.MethodPost {
		req, err = http.NewRequestWithContext(c.ctx, http.MethodPost, u.String(), bytes.NewBuffer(body))
	} else {
		req, err = http.NewRequestWithContext(c.ctx, http.MethodGet, u.String(), nil)
	}
	if err != nil {
		return nil, err
	}

	clientLog.Debug("Executing request", "url", req.URL.String(), "method", method)

	var reqInfo *SearchRequestInfo
	if c.debugEnabled {
		reqInfo = &SearchRequestInfo{
			Method: req.Method,
			Url:    req.URL.String(),
			Data:   string(body),
		}
	}

	req.Header = c.logzIoHeaders.GetDatasourceQueryHeaders(req.Header) // LOGZ.IO GRAFANA CHANGE :: (ALERTS) DEV-16492 Support external alert evaluation

	// LOGZ.IO GRAFANA CHANGE :: use application/json to interact with query-service
	// 	req.Header.Set("Content-Type", "application/x-ndjson")
	req.Header.Set("Content-Type", "application/json")

	httpClient, err := newDatasourceHttpClient(c.httpClientProvider, c.ds)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	defer func() {
		elapsed := time.Since(start)
		clientLog.Debug("Executed request", "took", elapsed)
	}()
	//nolint:bodyclose
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	// LOGZ.IO GRAFANA CHANGE :: DEV-17927 - Add error msg
	if resp != nil && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		errorResponse, err := c.DecodeErrorResponse(resp)
		if err != nil {
			return nil, err
		}
		errMsg := fmt.Sprintf("got bad response status from datasource. StatusCode: %d, Status: %s, RequestId: '%s', Message: %s",
			resp.StatusCode, resp.Status, errorResponse.RequestId, errorResponse.Message)
		clientLog.Error(errMsg)
		return nil, errors.New(errMsg)
	}
	// LOGZ.IO GRAFANA CHANGE :: end
	return &response{
		httpResponse: resp,
		reqInfo:      reqInfo,
	}, nil
}

func (c *baseClientImpl) ExecuteMultisearch(r *MultiSearchRequest) (*MultiSearchResponse, error) {
	clientLog.Debug("Executing multisearch", "search requests", len(r.Requests))

	multiRequests := c.createMultiSearchRequests(r.Requests)
	queryParams := c.getMultiSearchQueryParameters()
	clientRes, err := c.executeBatchRequest("_msearch", queryParams, multiRequests)
	if err != nil {
		return nil, err
	}
	res := clientRes.httpResponse
	defer func() {
		if err := res.Body.Close(); err != nil {
			clientLog.Warn("Failed to close response body", "err", err)
		}
	}()

	clientLog.Debug("Received multisearch response", "code", res.StatusCode, "status", res.Status, "content-length", res.ContentLength)

	start := time.Now()
	clientLog.Debug("Decoding multisearch json response")

	var bodyBytes []byte
	if c.debugEnabled {
		tmpBytes, err := ioutil.ReadAll(res.Body)
		if err != nil {
			clientLog.Error("failed to read http response bytes", "error", err)
		} else {
			bodyBytes = make([]byte, len(tmpBytes))
			copy(bodyBytes, tmpBytes)
			res.Body = ioutil.NopCloser(bytes.NewBuffer(tmpBytes))
		}
	}

	var msr MultiSearchResponse
	dec := json.NewDecoder(res.Body)
	err = dec.Decode(&msr)
	if err != nil {
		return nil, err
	}

	elapsed := time.Since(start)
	clientLog.Debug("Decoded multisearch json response", "took", elapsed)

	msr.Status = res.StatusCode

	if c.debugEnabled {
		bodyJSON, err := simplejson.NewFromReader(bytes.NewBuffer(bodyBytes))
		var data *simplejson.Json
		if err != nil {
			clientLog.Error("failed to decode http response into json", "error", err)
		} else {
			data = bodyJSON
		}

		msr.DebugInfo = &SearchDebugInfo{
			Request: clientRes.reqInfo,
			Response: &SearchResponseInfo{
				Status: res.StatusCode,
				Data:   data,
			},
		}
	}

	return &msr, nil
}

func (c *baseClientImpl) createMultiSearchRequests(searchRequests []*SearchRequest) []*multiRequest {
	multiRequests := []*multiRequest{}

	for _, searchReq := range searchRequests {
		mr := multiRequest{
			header: map[string]interface{}{
				"search_type":        "query_then_fetch",
				"ignore_unavailable": true,
				"index":              strings.Join(c.indices, ","),
			},
			body:     searchReq,
			interval: searchReq.Interval,
		}

		if c.version.Major() < 5 {
			mr.header["search_type"] = "count"
		}

		// LOGZ.IO GRAFANA CHANGE :: DEV-44969 do not set max_concurrent_shard_requests in query metadata or query params
		//else {
		//	allowedVersionRange, _ := semver.NewConstraint(">=5.6.0, <7.0.0")
		//
		//	if allowedVersionRange.Check(c.version) {
		//		maxConcurrentShardRequests := c.ds.MaxConcurrentShardRequests
		//		if maxConcurrentShardRequests == 0 {
		//			maxConcurrentShardRequests = 256
		//		}
		//		mr.header["max_concurrent_shard_requests"] = maxConcurrentShardRequests
		//	}
		//}
		// LOGZ.IO end
		multiRequests = append(multiRequests, &mr)
	}

	return multiRequests
}

func (c *baseClientImpl) getMultiSearchQueryParameters() string {
	var qs []string

	// LOGZ.IO GRAFANA CHANGE :: DEV-20400 Grafana alerts evaluation - set 'accountsToSearch' query param
	datasourceUrl, _ := url.Parse(c.ds.URL)
	q, _ := url.ParseQuery(datasourceUrl.RawQuery)
	if len(q.Get("querySource")) > 0 {
		// set/override 'accountsToSearch' as Database (accountId)
		qs = append(qs, fmt.Sprintf("accountsToSearch=%s", c.ds.Database))
		qs = append(qs, "querySource=INTERNAL_METRICS_ALERTS")
	}
	// LOGZ.IO end

	// LOGZ.IO GRAFANA CHANGE :: DEV-44969 do not set max_concurrent_shard_requests in query metadata or query params
	//if c.version.Major() >= 7 {
	//	maxConcurrentShardRequests := c.ds.MaxConcurrentShardRequests
	//	if maxConcurrentShardRequests == 0 {
	//		maxConcurrentShardRequests = 5
	//	}
	//	qs = append(qs, fmt.Sprintf("max_concurrent_shard_requests=%d", maxConcurrentShardRequests))
	//}
	// LOGZ.io end

	allowedFrozenIndicesVersionRange, _ := semver.NewConstraint(">=6.6.0")

	if (allowedFrozenIndicesVersionRange.Check(c.version)) && c.ds.IncludeFrozen && c.ds.XPack {
		qs = append(qs, "ignore_throttled=false")
	}

	return strings.Join(qs, "&")
}

func (c *baseClientImpl) MultiSearch() *MultiSearchRequestBuilder {
	return NewMultiSearchRequestBuilder(c.GetVersion())
}

func (c *baseClientImpl) EnableDebug() {
	c.debugEnabled = true
}
