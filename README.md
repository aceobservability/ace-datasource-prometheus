# ace-datasource-prometheus

Compile-time Prometheus datasource module for [Ace](https://github.com/aceobservability/ace).

Ace keeps the datasource contract and registry in
`github.com/aceobservability/ace/backend/pkg/datasource`. This module implements
that `Client` (plus connection test and PromQL metadata). Ace registers the
factory at `init` and injects its SSRF-safe HTTP client — this module does not
import Ace `internal/` packages and does not construct an unpolicy'd client.

## Contract

| Surface | Package |
| --- | --- |
| Query / result types | `github.com/aceobservability/ace/backend/pkg/datasource` |
| Registry type key | `prometheus` (`Type`) |
| Factory | `New(url string, httpClient *http.Client)` |

`httpClient` is required. Ace passes `ssrf.DatasourceClient` wrapped with stored
datasource credentials.

The low-level Prometheus HTTP API client lives in `./promclient` (used by Ace's
built-in `PROMETHEUS_URL` handler as well as this adapter).

## Tests

```
go test ./...
```

Query and connection tests speak to an `httptest` fixture. No live Prometheus
is required.
