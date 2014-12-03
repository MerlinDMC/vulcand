package proxy

import (
	"fmt"

	"github.com/mailgun/vulcand/engine"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/metrics"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
)

// Reporter reports real time metrics to the Statsd client
type Reporter struct {
	c  metrics.Client
	fk engine.FrontendKey
}

func NewReporter(c metrics.Client, fk engine.FrontendKey) *Reporter {
	return &Reporter{
		c:  c,
		fk: fk,
	}
}

func (rp *Reporter) ObserveRequest(r request.Request) {
}

func (rp *Reporter) ObserveResponse(r request.Request, a request.Attempt) {
	if a == nil {
		return
	}
	rp.emitMetrics(r, a, "frontend", rp.fk.Id)
	if a.GetEndpoint() != nil {
		srv, ok := a.GetEndpoint().(*muxServer)
		if ok {
			rp.emitMetrics(r, a, "server", srv.sk.BackendKey.Id, srv.sk.Id)
		}
	}
}

func (rp *Reporter) emitMetrics(r request.Request, a request.Attempt, p ...string) {
	// Report ttempt roundtrip time
	m := rp.c.Metric(p...)
	rp.c.TimingMs(m.Metric("rtt"), a.GetDuration(), 1)

	// Report request throughput
	if body := r.GetBody(); body != nil {
		if bytes, err := body.TotalSize(); err != nil {
			rp.c.Timing(m.Metric("request", "bytes"), bytes, 1)
		}
	}

	// Response code-related metrics
	if re := a.GetResponse(); re != nil {
		rp.c.Inc(m.Metric("code", fmt.Sprintf("%v", re.StatusCode)), 1, 1)
		rp.c.Inc(m.Metric("request"), 1, 1)
	}
}
