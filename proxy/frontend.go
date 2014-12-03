package proxy

import (
	"fmt"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/loadbalance/roundrobin"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/location/httploc"

	"github.com/mailgun/vulcand/engine"
)

type frontend struct {
	key      engine.FrontendKey
	mux      *mux
	frontend engine.Frontend
	hloc     *httploc.HttpLocation
	backend  *backend
}

func (f *frontend) getLB() *roundrobin.RoundRobin {
	return f.hloc.GetLoadBalancer().(*roundrobin.RoundRobin)
}

func newFrontend(m *mux, f engine.Frontend, b *backend) (*frontend, error) {
	router := m.router

	// Create a load balancer that handles all the endpoints within the given frontend
	rr, err := roundrobin.NewRoundRobin()
	if err != nil {
		return nil, err
	}

	// Create a http frontend
	settings := f.HTTPSettings()
	options, err := settings.GetOptions()
	if err != nil {
		return nil, err
	}

	// Use the transport from the backend
	options.Transport = b.transport
	hloc, err := httploc.NewLocationWithOptions(f.Id, rr, *options)
	if err != nil {
		return nil, err
	}

	// Register metric emitters and performance monitors
	hloc.GetObserverChain().Upsert(Metrics, NewReporter(m.options.MetricsClient, f.Id))
	hloc.GetObserverChain().Upsert(PerfMon, m.perfMon)

	fr := &frontend{
		key:      engine.FrontendKey{Id: f.Id},
		hloc:     hloc,
		frontend: f,
		mux:      m,
		backend:  b,
	}

	if err := fr.syncServers(); err != nil {
		return nil, err
	}

	// Add the frontend to the router
	if err := router.AddRoute(settings.Route, hloc); err != nil {
		return nil, err
	}

	b.linkFrontend(engine.FrontendKey{f.Id}, fr)

	return fr, nil
}

func (f *frontend) String() string {
	return fmt.Sprintf("%v frontend(wrap=%v)", f.mux, &f.frontend)
}

func (f *frontend) syncServers() error {
	rr := f.getLB()
	if rr == nil {
		return fmt.Errorf("%v lb not found", f.frontend)
	}

	// First, collect and parse servers to add
	newServers := map[string]*muxServer{}
	for _, s := range f.backend.servers {
		ep, err := newMuxServer(engine.ServerKey{BackendKey: engine.BackendKey{Id: f.backend.backend.Id}, Id: s.Id}, &s, f.mux.perfMon)
		if err != nil {
			return fmt.Errorf("failed to create load balancer from %v", &s)
		}
		newServers[s.URL] = ep
	}

	// Memorize what endpoints exist in load balancer at the moment
	existingServers := map[string]*muxServer{}
	for _, e := range rr.GetEndpoints() {
		existingServers[e.GetUrl().String()] = e.GetOriginalEndpoint().(*muxServer)
	}

	// First, add endpoints, that should be added and are not in lb
	for _, s := range newServers {
		if _, exists := existingServers[s.GetUrl().String()]; !exists {
			if err := rr.AddEndpoint(s); err != nil {
				log.Errorf("%v failed to add %v, err: %s", f.mux, s, err)
			} else {
				log.Infof("%v add %v to %v", f.mux, s, &f.frontend)
			}
		}
	}

	// Second, remove endpoints that should not be there any more
	for _, s := range existingServers {
		if _, exists := newServers[s.GetUrl().String()]; !exists {
			f.mux.perfMon.deleteServer(s.sk)
			if err := rr.RemoveEndpoint(s); err != nil {
				log.Errorf("%v failed to remove %v, err: %v", f.mux, s, err)
			} else {
				log.Infof("%v removed %v from %v", f.mux, s, &f.frontend)
			}
		}
	}
	return nil
}

func (f *frontend) upsertMiddleware(fk engine.FrontendKey, mi *engine.Middleware) error {
	instance, err := mi.Middleware.NewMiddleware()
	if err != nil {
		return err
	}
	mk := engine.MiddlewareKey{FrontendKey: fk, Id: mi.Id}
	f.hloc.GetMiddlewareChain().Upsert(mk.String(), mi.Priority, instance)
	return nil
}

func (f *frontend) deleteMiddleware(mk engine.MiddlewareKey) error {
	return f.hloc.GetMiddlewareChain().Remove(mk.String())
}

func (f *frontend) updateBackend(b *backend) error {
	oldb := f.backend
	f.backend = b

	// Switching backends, set the new transport and perform switch
	if b.backend.Id != oldb.backend.Id {
		log.Infof("%v updating backend from %v to %v", f, &oldb, &f.backend)
		oldb.unlinkFrontend(f.key)
		b.linkFrontend(f.key, f)
		f.hloc.SetTransport(b.transport)
	}
	return f.syncServers()
}

// TODO: implement rollback in case of suboperation failure
func (f *frontend) update(ef engine.Frontend, b *backend) error {
	oldf := f.frontend
	f.frontend = ef

	settings := f.frontend.HTTPSettings()
	options, err := settings.GetOptions()
	if err != nil {
		return err
	}

	options.Transport = f.backend.transport
	if err := f.hloc.SetOptions(*options); err != nil {
		return err
	}

	if err := f.updateBackend(b); err != nil {
		return err
	}

	oldsettings := oldf.HTTPSettings()

	if oldsettings.Route != settings.Route {
		log.Infof("%v updating route from %v to %v", oldsettings.Route, settings.Route)
		if f.mux.router.AddRoute(settings.Route, f); err != nil {
			return err
		}
		if err := f.mux.router.RemoveRoute(oldsettings.Route); err != nil {
			return err
		}
	}
	return nil
}

func (f *frontend) remove() error {
	f.mux.perfMon.deleteFrontend(f.key)
	f.backend.unlinkFrontend(f.key)
	return f.mux.router.RemoveRoute(f.frontend.HTTPSettings().Route)
}
