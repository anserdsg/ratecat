package metric

import (
	"github.com/anserdsg/ratecat/v1/internal/logging"
)

type LogMetricBackend struct{}

func NewLogMetricBackend() *LogMetricBackend {
	return &LogMetricBackend{}
}

func (m *LogMetricBackend) ReportAPICmdRPS(rps float64) (err error) {
	if rps == 0.0 {
		return
	}

	logging.Infow("API command RPS", "rps", rps)
	return
}

func (m *LogMetricBackend) ReportResourceRPS(name string, rps float64) (err error) {
	if rps == 0.0 {
		return
	}

	logging.Infow("Resource RPS", "name", name, "rps", rps)
	return
}
