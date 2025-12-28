// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package daemon

import (
	"net/http"
	"net/http/httptest"

	"github.com/tombee/conductor/pkg/security"
)

// CombinedMetricsHandler combines OTel metrics with security metrics
type CombinedMetricsHandler struct {
	otelHandler      http.Handler
	securityExporter *security.PrometheusExporter
}

// NewCombinedMetricsHandler creates a handler that outputs both OTel and security metrics
func NewCombinedMetricsHandler(otelHandler http.Handler, metricsCollector *security.MetricsCollector) *CombinedMetricsHandler {
	var exporter *security.PrometheusExporter
	if metricsCollector != nil {
		exporter = security.NewPrometheusExporter(metricsCollector)
	}
	return &CombinedMetricsHandler{
		otelHandler:      otelHandler,
		securityExporter: exporter,
	}
}

// ServeHTTP implements http.Handler by combining OTel and security metrics
func (h *CombinedMetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// First, get the OTel metrics by capturing the response
	recorder := httptest.NewRecorder()
	h.otelHandler.ServeHTTP(recorder, r)

	// Write OTel metrics
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(recorder.Code)
	w.Write(recorder.Body.Bytes())

	// Append security metrics if available
	if h.securityExporter != nil {
		securityMetrics := h.securityExporter.Export()
		w.Write([]byte("\n"))
		w.Write([]byte(securityMetrics))
	}
}
