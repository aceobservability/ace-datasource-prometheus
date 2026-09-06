package prometheus

import (
	"context"
	"net/http"
	"time"

	"github.com/aceobservability/ace-datasource-prometheus/promclient"
	"github.com/aceobservability/ace/backend/pkg/datasource"
)

// Type is the RegisterDatasource key Ace uses for this module.
const Type = "prometheus"

// Client implements the Ace Prometheus query datasource.
type Client struct {
	url        string
	httpClient *http.Client
	api        *promclient.Client
	meta       *promqlMetadata
}

// New constructs a Prometheus datasource client.
// httpClient is required so Ace can inject DatasourceClient (dial/redirect policy + auth).
func New(prometheusURL string, httpClient *http.Client) (*Client, error) {
	api, err := promclient.NewClient(prometheusURL, httpClient)
	if err != nil {
		return nil, err
	}
	return &Client{
		url:        prometheusURL,
		httpClient: httpClient,
		api:        api,
		meta: &promqlMetadata{
			baseURL: prometheusURL,
			client:  httpClient,
		},
	}, nil
}

// HTTPClient returns the injected HTTP client. Ace SSRF tests inspect policy wiring.
func (c *Client) HTTPClient() *http.Client {
	return c.httpClient
}

func (c *Client) Query(ctx context.Context, query string, start, end time.Time, step time.Duration, limit int) (*datasource.QueryResult, error) {
	result, err := c.api.QueryRange(ctx, query, start, end, step)
	if err != nil {
		return nil, err
	}

	qr := &datasource.QueryResult{
		Status:     result.Status,
		Error:      result.Error,
		ResultType: "metrics",
	}

	if result.Data != nil {
		qr.Data = &datasource.QueryData{
			ResultType: result.Data.ResultType,
			Result:     make([]datasource.MetricResult, len(result.Data.Result)),
		}
		for i, r := range result.Data.Result {
			qr.Data.Result[i] = datasource.MetricResult{
				Metric: r.Metric,
				Values: r.Values,
			}
		}
	}

	return qr, nil
}

func (c *Client) Labels(ctx context.Context, metric string) ([]string, error) {
	return c.meta.Labels(ctx, metric)
}

func (c *Client) LabelValues(ctx context.Context, label, metric string) ([]string, error) {
	return c.meta.LabelValues(ctx, label, metric)
}

func (c *Client) MetricNames(ctx context.Context, search string) ([]string, error) {
	return c.meta.MetricNames(ctx, search)
}

var (
	_ datasource.Client                  = (*Client)(nil)
	_ datasource.MetricLabelsClient      = (*Client)(nil)
	_ datasource.MetricLabelValuesClient = (*Client)(nil)
	_ datasource.MetricNamesClient       = (*Client)(nil)
)
