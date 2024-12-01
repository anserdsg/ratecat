package metric

type NullMetricBackend struct{}

var NullMetric = &NullMetricBackend{}

func (m *NullMetricBackend) ReportAPICmdRPS(rps float64) (err error)                { return }
func (m *NullMetricBackend) ReportResourceRPS(name string, rps float64) (err error) { return }
