package proxy

import (
	"fmt"
	"sort"
	"sync"

	"github.com/mailgun/vulcand/engine"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/timetools"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/metrics"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
)

// perfMon stands for performance monitor, it is observer that watches realtime metrics
// for locations, endpoints and upstreams
type perfMon struct {
	m         *sync.RWMutex
	frontends map[string]*metricsBucket
	servers   map[string]*metricsBucket
	backends  map[string]*metricsBucket
	clock     timetools.TimeProvider
}

func newPerfMon(clock timetools.TimeProvider) *perfMon {
	return &perfMon{
		m:         &sync.RWMutex{},
		frontends: make(map[string]*metricsBucket),
		servers:   make(map[string]*metricsBucket),
		backends:  make(map[string]*metricsBucket),
		clock:     clock,
	}
}

func (m *perfMon) topFrontends(key *engine.BackendKey) ([]engine.Frontend, error) {
	m.m.RLock()
	defer m.m.RUnlock()

	frontends := []engine.Frontend{}
	for _, m := range m.frontends {
		if key != nil && key.Id != m.server.sk.Id {
			continue
		}
		f := m.server.f.frontend
		if s, err := m.getStats(); err == nil {
			f.Stats = s
		}
		frontends = append(frontends, f)
	}
	sort.Sort(&frontendSorter{frontends: frontends})
	return frontends, nil
}

func (m *perfMon) topServers(key *engine.BackendKey) ([]engine.Server, error) {
	m.m.RLock()
	defer m.m.RUnlock()

	servers := []engine.Server{}
	for _, m := range m.servers {
		srv := m.server.s
		if key != nil && key.Id != m.server.sk.BackendKey.Id {
			continue
		}
		if s, err := m.getStats(); err == nil {
			srv.Stats = s
		}
		servers = append(servers, srv)
	}
	sort.Sort(&serverSorter{es: servers})
	return servers, nil
}

func (m *perfMon) frontendStats(key engine.FrontendKey) (*engine.RoundTripStats, error) {
	m.m.RLock()
	defer m.m.RUnlock()

	b, err := m.findBucket(key.String(), m.frontends)
	if err != nil {
		return nil, err
	}

	return b.getStats()
}

func (m *perfMon) serverStats(key engine.ServerKey) (*engine.RoundTripStats, error) {
	m.m.RLock()
	defer m.m.RUnlock()

	b, err := m.findBucket(key.String(), m.servers)
	if err != nil {
		return nil, err
	}
	return b.getStats()
}

func (m *perfMon) backendStats(key engine.BackendKey) (*engine.RoundTripStats, error) {
	m.m.RLock()
	defer m.m.RUnlock()

	b, err := m.findBucket(key.String(), m.backends)
	if err != nil {
		return nil, err
	}
	return b.getStats()
}

func (m *perfMon) ObserveRequest(r request.Request) {
}

func (m *perfMon) ObserveResponse(r request.Request, a request.Attempt) {
	if a == nil || a.GetEndpoint() == nil {
		return
	}

	srv, ok := a.GetEndpoint().(*muxServer)
	if !ok {
		log.Errorf("Unknown endpoint type %T", a.GetEndpoint())
		return
	}

	m.recordBucketMetrics(engine.FrontendKey{Id: srv.f.frontend.Id}.String(), m.frontends, a, srv)
	m.recordBucketMetrics(srv.sk.BackendKey.String(), m.backends, a, srv)
	m.recordBucketMetrics(srv.sk.String(), m.servers, a, srv)
}

func (m *perfMon) deleteFrontend(key engine.FrontendKey) {
	m.deleteBucket(key.String(), m.frontends)
}

func (m *perfMon) deleteServer(key engine.ServerKey) {
	m.deleteBucket(key.String(), m.servers)
}

func (m *perfMon) deleteBackend(bk engine.BackendKey) {
	m.deleteBucket(bk.String(), m.backends)
	for k, _ := range m.servers {
		key := engine.MustParseServerKey(k)
		if key.BackendKey == bk {
			m.deleteBucket(key.String(), m.servers)
		}
	}
}

func (m *perfMon) recordBucketMetrics(id string, ms map[string]*metricsBucket, a request.Attempt, e *muxServer) {
	m.m.Lock()
	defer m.m.Unlock()

	if b, err := m.getBucket(id, ms, e); err == nil {
		b.recordMetrics(a)
	} else {
		log.Errorf("failed to get bucket for %v, error: %v", id, err)
	}
}

func (m *perfMon) deleteBucket(id string, ms map[string]*metricsBucket) {
	m.m.Lock()
	defer m.m.Unlock()

	delete(ms, id)
}

func (m *perfMon) findBucket(id string, ms map[string]*metricsBucket) (*metricsBucket, error) {
	if b, ok := ms[id]; ok {
		return b, nil
	}
	return nil, fmt.Errorf("bucket %s not found", id)
}

func (m *perfMon) getBucket(id string, ms map[string]*metricsBucket, s *muxServer) (*metricsBucket, error) {
	if b, ok := ms[id]; ok {
		return b, nil
	}
	mt, err := metrics.NewRoundTripMetrics(metrics.RoundTripOptions{TimeProvider: m.clock})
	if err != nil {
		return nil, err
	}
	b := &metricsBucket{
		server:  s,
		metrics: mt,
	}
	ms[id] = b
	return b, nil
}

// metricBucket holds common metrics collected for every part that serves requests.
type metricsBucket struct {
	server  *muxServer
	metrics *metrics.RoundTripMetrics
}

func (m *metricsBucket) recordMetrics(a request.Attempt) {
	m.metrics.RecordMetrics(a)
}

func (m *metricsBucket) resetStats() error {
	m.metrics.Reset()
	return nil
}

func (m *metricsBucket) getStats() (*engine.RoundTripStats, error) {
	return engine.NewRoundTripStats(m.metrics)
}

type frontendSorter struct {
	frontends []engine.Frontend
}

func (s *frontendSorter) Len() int {
	return len(s.frontends)
}

func (s *frontendSorter) Swap(i, j int) {
	s.frontends[i], s.frontends[j] = s.frontends[j], s.frontends[i]
}

func (s *frontendSorter) Less(i, j int) bool {
	return cmpStats(s.frontends[i].Stats, s.frontends[j].Stats)
}

type serverSorter struct {
	es []engine.Server
}

func (s *serverSorter) Len() int {
	return len(s.es)
}

func (s *serverSorter) Swap(i, j int) {
	s.es[i], s.es[j] = s.es[j], s.es[i]
}

func (s *serverSorter) Less(i, j int) bool {
	return cmpStats(s.es[i].Stats, s.es[j].Stats)
}

func cmpStats(s1, s2 *engine.RoundTripStats) bool {
	// Items that have network errors go first
	if s1.NetErrorRatio() != 0 || s2.NetErrorRatio() != 0 {
		return s1.NetErrorRatio() > s2.NetErrorRatio()
	}

	// Items that have application level errors go next
	if s1.AppErrorRatio() != 0 || s2.AppErrorRatio() != 0 {
		return s1.AppErrorRatio() > s2.AppErrorRatio()
	}

	// More highly loaded items go next
	return s1.Counters.Total > s2.Counters.Total
}
