package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/mailgun/vulcand/engine"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/metrics"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/route"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/timetools"
)

// mux is capable of listening on multiple interfaces, graceful shutdowns and updating TLS certificates
type mux struct {
	// Debugging id
	id int

	// Each listener address has a server associated with it
	servers map[engine.ListenerKey]*srv

	backends map[engine.BackendKey]*backend

	frontends map[engine.FrontendKey]*frontend

	hosts map[engine.HostKey]engine.Host

	// Options hold parameters that are used to initialize http servers
	options Options

	// Wait group for graceful shutdown
	wg *sync.WaitGroup

	// Read write mutex for serlialized operations
	mtx *sync.RWMutex

	// Router will be shared between mulitple listeners
	router *route.Mux

	// Current server stats
	state muxState

	// Connection watcher
	connTracker *connTracker
}

func (m *mux) String() string {
	return fmt.Sprintf("mux(%d, %v)", m.id, m.state)
}

func New(id int, o Options) (*mux, error) {
	o = setDefaults(o)
	m := &mux{
		id:  id,
		wg:  &sync.WaitGroup{},
		mtx: &sync.RWMutex{},

		options: o,

		router:      route.NewMux(),
		connTracker: newConnTracker(o.MetricsClient),

		servers:   make(map[engine.ListenerKey]*srv),
		backends:  make(map[engine.BackendKey]*backend),
		frontends: make(map[engine.FrontendKey]*frontend),
		hosts:     make(map[engine.HostKey]engine.Host),
	}
	m.router.NotFound = &NotFound{}
	if m.options.DefaultListener != nil {
		if err := m.upsertListener(*m.options.DefaultListener); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func (m *mux) GetFiles() ([]*FileDescriptor, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	fds := []*FileDescriptor{}

	for _, srv := range m.servers {
		fd, err := srv.GetFile()
		if err != nil {
			return nil, err
		}
		if fd != nil {
			fds = append(fds, fd)
		}
	}
	return fds, nil
}

func (m *mux) FrontendStats(key engine.FrontendKey) (*engine.RoundTripStats, error) {
	log.Infof("%s FrontendStats", m)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	return m.frontendStats(key)
}

func (m *mux) ServerStats(key engine.ServerKey) (*engine.RoundTripStats, error) {
	log.Infof("%s ServerStats", m)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	return m.serverStats(key)
}

func (m *mux) BackendStats(key engine.BackendKey) (*engine.RoundTripStats, error) {
	log.Infof("%s BackendStats", m)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	return m.backendStats(key)
}

// TopFrontends returns locations sorted by criteria (faulty, slow, most used)
// if hostname or backendId is present, will filter out locations for that host or backendId
func (m *mux) TopFrontends(key *engine.BackendKey) ([]engine.Frontend, error) {
	log.Infof("%s TopFrontends", m)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	return m.topFrontends(key)
}

// TopServers returns endpoints sorted by criteria (faulty, slow, mos used)
// if backendId is not empty, will filter out endpoints for that backendId
func (m *mux) TopServers(key *engine.BackendKey) ([]engine.Server, error) {
	log.Infof("%s TopServers", m)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	return m.topServers(key)
}

func (m *mux) TakeFiles(files []*FileDescriptor) error {
	log.Infof("%s TakeFiles %s", m, files)

	fMap := make(map[engine.Address]*FileDescriptor, len(files))
	for _, f := range files {
		fMap[f.Address] = f
	}

	m.mtx.Lock()
	defer m.mtx.Unlock()

	for _, srv := range m.servers {

		file, exists := fMap[srv.listener.Address]
		if !exists {
			log.Infof("%s skipping take of files from address %s, has no passed files", m, srv.listener.Address)
			continue
		}
		if err := srv.takeFile(file); err != nil {
			return err
		}
	}

	return nil
}

func (m *mux) Start() error {
	log.Infof("%s start", m)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	if m.state != stateInit {
		return fmt.Errorf("%s can start only from init state, got %d", m, m.state)
	}

	m.state = stateActive
	for _, s := range m.servers {
		if err := s.start(); err != nil {
			return err
		}
	}

	log.Infof("%s started", m)
	return nil
}

func (m *mux) Stop(wait bool) {
	log.Infof("%s Stop(%t)", m, wait)

	m.stopServers()

	if wait {
		log.Infof("%s waiting for the wait group to finish", m)
		m.wg.Wait()
		log.Infof("%s wait group finished", m)
	}
}

func (m *mux) stopServers() {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if m.state == stateInit {
		m.state = stateShuttingDown
		return
	}
	if m.state == stateShuttingDown {
		return
	}
	m.state = stateShuttingDown
	for _, s := range m.servers {
		s.shutdown()
	}
}

func (m *mux) UpsertHost(host engine.Host) error {
	log.Infof("%s UpsertHost %s", m, &host)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	m.hosts[engine.HostKey{Name: host.Name}] = host

	for _, s := range m.servers {
		if s.isTLS() {
			if err := s.upsertKeyPair(engine.HostKey{Name: host.Name}, host.Settings.KeyPair); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *mux) DeleteHost(hk engine.HostKey) error {
	log.Infof("%s DeleteHost %v", m, &hk)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	host, exists := m.hosts[hk]
	if !exists {
		return &engine.NotFoundError{Message: fmt.Sprintf("%v not found", hk)}
	}

	if host.Settings.KeyPair == nil {
		return nil
	}

	for _, s := range m.servers {
		if err := s.deleteKeyPair(hk); err != nil {
			return err
		}
	}
	return nil
}

func (m *mux) UpsertListener(l engine.Listener) error {
	log.Infof("%v UpsertListener %v", m, &l)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	return m.upsertListener(l)
}

func (m *mux) DeleteListener(lk engine.ListenerKey) error {
	log.Infof("%v DeleteListener %v", m, &lk)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	s, exists := m.servers[lk]
	if !exists {
		return &engine.NotFoundError{Message: fmt.Sprintf("%v not found", lk)}
	}

	delete(m.servers, lk)
	s.shutdown()
	return nil
}

func (m *mux) upsertListener(l engine.Listener) error {
	lk := engine.ListenerKey{Id: l.Id}
	s, exists := m.servers[lk]
	if exists {
		// We can not listen for different protocols on the same socket
		if s.listener.Protocol != l.Protocol {
			return fmt.Errorf("conflicting protocol %s and %s", s.listener.Protocol, l.Protocol)
		}
		return nil
	}

	// Check if there's a listener with the same address
	for _, srv := range m.servers {
		if srv.listener.Address == l.Address {
			return &engine.AlreadyExistsError{Message: fmt.Sprintf("%v conflicts with existing %v", l, srv.listener)}
		}
	}

	var err error
	if s, err = newSrv(m, l); err != nil {
		return err
	}
	m.servers[lk] = s
	// If we are active, start the server immediatelly
	if m.state == stateActive {
		log.Infof("Mux is in active state, starting the HTTP server")
		if err := s.start(); err != nil {
			return err
		}
	}
	return nil
}

func (m *mux) UpsertBackend(b engine.Backend) error {
	log.Infof("%v UpsertBackend %v", m, &b)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	_, err := m.upsertBackend(b)
	return err
}

func (m *mux) upsertBackend(be engine.Backend) (*backend, error) {
	bk := engine.BackendKey{Id: be.Id}
	b, ok := m.backends[bk]
	if ok {
		return b, b.update(be)
	}
	b, err := newBackend(m, be)
	if err != nil {
		return nil, err
	}
	m.backends[bk] = b
	return b, nil
}

func (m *mux) DeleteBackend(bk engine.BackendKey) error {
	log.Infof("%v DeleteBackend %s", m, &bk)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	b, ok := m.backends[bk]
	if !ok {
		return &engine.NotFoundError{Message: fmt.Sprintf("%v not found", bk)}
	}

	if len(b.frontends) != 0 {
		return fmt.Errorf("%v is used by frontends: %v", b, b.frontends)
	}

	b.Close()
	return nil
}

func (m *mux) UpsertFrontend(f engine.Frontend) error {
	log.Infof("%v UpsertFrontend %v", m, &f)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	_, err := m.upsertFrontend(f)
	return err
}

func (m *mux) upsertFrontend(fe engine.Frontend) (*frontend, error) {
	bk := engine.BackendKey{Id: fe.BackendId}
	b, ok := m.backends[bk]
	if !ok {
		return nil, &engine.NotFoundError{Message: fmt.Sprintf("%v not found", bk)}
	}
	fk := engine.FrontendKey{Id: fe.Id}
	f, ok := m.frontends[fk]
	if ok {
		return f, f.update(fe, b)
	}

	f, err := newFrontend(m, fe, b)
	if err != nil {
		return nil, err
	}

	m.frontends[fk] = f
	return f, nil
}

func (m *mux) DeleteFrontend(fk engine.FrontendKey) error {
	log.Infof("%v DeleteFrontend %v, %v", m, &fk)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	return m.deleteFrontend(fk)
}

func (m *mux) deleteFrontend(fk engine.FrontendKey) error {
	f, ok := m.frontends[fk]
	if !ok {
		return &engine.NotFoundError{Message: fmt.Sprintf("%v not found", fk)}
	}
	if err := f.remove(); err != nil {
		return err
	}
	delete(m.frontends, fk)
	return nil
}

func (m *mux) UpsertMiddleware(fk engine.FrontendKey, mi engine.Middleware) error {
	log.Infof("%v UpsertMiddleware %v, %v", m, &fk, &mi)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	return m.upsertMiddleware(fk, mi)
}

func (m *mux) upsertMiddleware(fk engine.FrontendKey, mi engine.Middleware) error {
	f, ok := m.frontends[fk]
	if !ok {
		return &engine.NotFoundError{Message: fmt.Sprintf("%v not found", fk)}
	}
	return f.upsertMiddleware(fk, mi)
}

func (m *mux) DeleteMiddleware(mk engine.MiddlewareKey) error {
	log.Infof("%v DeleteMiddleware(%v %v)", m, &mk)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	f, ok := m.frontends[mk.FrontendKey]
	if !ok {
		return &engine.NotFoundError{Message: fmt.Sprintf("%v not found", mk)}
	}

	return f.deleteMiddleware(mk)
}

func (m *mux) UpsertServer(bk engine.BackendKey, srv engine.Server) error {
	log.Infof("%v UpsertServer %v %v", m, &bk, &srv)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	if _, err := url.ParseRequestURI(srv.URL); err != nil {
		return fmt.Errorf("failed to parse %v, error: %v", srv, err)
	}

	b, ok := m.backends[bk]
	if !ok {
		var err error
		if b, err = m.upsertBackend(engine.Backend{Id: bk.Id, Type: engine.HTTP, Settings: engine.HTTPBackendSettings{}}); err != nil {
			return err
		}
	}

	return b.upsertServer(srv)
}

func (m *mux) DeleteServer(sk engine.ServerKey) error {
	log.Infof("%v DeleteServer %v", m, &sk)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	b, ok := m.backends[sk.BackendKey]
	if !ok {
		return &engine.NotFoundError{Message: fmt.Sprintf("%v not found", sk.BackendKey)}
	}

	return b.deleteServer(sk)
}

func (m *mux) transportSettings(b engine.Backend) (*engine.TransportSettings, error) {
	s, err := b.TransportSettings()
	if err != nil {
		return nil, err
	}
	// Apply global defaults if options are not set
	if s.Timeouts.Dial == 0 {
		s.Timeouts.Dial = m.options.DialTimeout
	}
	if s.Timeouts.Read == 0 {
		s.Timeouts.Read = m.options.ReadTimeout
	}
	return s, nil
}

type muxState int

const (
	stateInit         = iota // Server has been created, but does not accept connections yet
	stateActive              // Server is active and accepting connections
	stateShuttingDown        // Server is active, but is draining existing connections and does not accept new connections.
)

func (s muxState) String() string {
	switch s {
	case stateInit:
		return "init"
	case stateActive:
		return "active"
	case stateShuttingDown:
		return "shutting down"
	}
	return "undefined"
}

const (
	Metrics = "_metrics"
	PerfMon = "_perfMon"
)

func setDefaults(o Options) Options {
	if o.MetricsClient == nil {
		o.MetricsClient = metrics.NewNop()
	}
	if o.TimeProvider == nil {
		o.TimeProvider = &timetools.RealTime{}
	}
	return o
}

// NotFound is a generic http.Handler for request
type NotFound struct {
}

// ServeHTTP returns a simple 404 Not found response
func (*NotFound) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Infof("Not found: %v %v", r.Method, r.URL)
	w.Header().Set("Content-Type", "application-json")
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprint(w, `{"error":"not found"}`)
}
