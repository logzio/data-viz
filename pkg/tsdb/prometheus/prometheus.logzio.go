// LOGZ.IO GRAFANA CHANGE :: (ALERTS) DEV-16492 Support external alert evaluation

package prometheus

import (
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/tsdb"
	"github.com/prometheus/client_golang/api"
	apiv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"net/http"
)

type logzIoAuthTransport struct {
	Transport     http.RoundTripper
	logzIoHeaders *models.LogzIoHeaders
}

var (
	clientLog = log.New("tsdb.prometheus.logzio.client")
)

func (lat logzIoAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// TODO: DEV-23495 Remove once we support POST method in the m3-query-service for query and query_range
	// the prometheus client will attempt to do POST request, and on a 405 it will fallback to a GET request
	if req.Method == "POST" {
		clientLog.Debug("Forcing GET request fallback", "method", req.Method, "url", req.URL.String())

		return &http.Response {StatusCode: http.StatusMethodNotAllowed}, nil
	}

	clientLog.Debug("Executing request", "method", req.Method, "url", req.URL.String())
	req.Header = lat.logzIoHeaders.GetDatasourceQueryHeaders(req.Header)
	resp, err := lat.Transport.RoundTrip(req)

	if resp != nil && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		clientLog.Error("got bad response status from datasource", "status", resp.StatusCode, "method", req.Method, "url", req.URL.String())
	}

	return resp, err
}

func (e *PrometheusExecutor) getLogzioAuthClient(dsInfo *models.DataSource, tsdbQuery *tsdb.TsdbQuery) (apiv1.API, error) {
	cfg := api.Config{
		Address:      dsInfo.Url,
		RoundTripper: e.Transport,
	}

	cfg.RoundTripper = logzIoAuthTransport{
		Transport:     e.Transport,
		logzIoHeaders: tsdbQuery.LogzIoHeaders,
	}

	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	return apiv1.NewAPI(client), nil
}

// LOGZ.IO GRAFANA CHANGE :: end
