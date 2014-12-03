package proxy

import (
	"net/http"

	"github.com/mailgun/vulcand/engine"

	//	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/location/httploc"
)

type backend struct {
	mux     *mux
	backend engine.Backend

	frontends map[engine.FrontendKey]*frontend
	servers   []engine.Server
	transport *http.Transport
}

func newBackend(m *mux, b engine.Backend) (*backend, error) {
	o, err := m.getTransportOptions(b)
	if err != nil {
		return nil, err
	}
	return &backend{
		mux:       m,
		backend:   b,
		transport: httploc.NewTransport(*o),
		servers:   []engine.Server{},
		frontends: make(map[engine.FrontendKey]*frontend),
	}, nil
}

func (b *backend) linkFrontend(key engine.FrontendKey, f *frontend) {
	b.frontends[key] = f
}

func (b *backend) unlinkFrontend(key engine.FrontendKey) {
	delete(b.frontends, key)
}

/*


func (b *backend) deleteFrontend(key engine.FrontendKey) {
	delete(b.fs, key)
}

func (b *backend) Close() error {
	b.t.CloseIdleConnections()
	return nil
}

func (b *backend) update(be engine.Backend) error {
	if err := b.updateSettings(be.Settings.(engine.HTTPBackendSettings)); err != nil {
		return err
	}
}

func (b *backend) updateSettings(opts engine.HTTPBackendSettings) error {
	// Nothing changed in transport options
	if u.up.Options.Equals(opts) {
		return nil
	}
	u.up.Options = opts

	o, err := u.m.getTransportOptions(&u.up)
	if err != nil {
		return err
	}
	t := httploc.NewTransport(*o)
	u.t.CloseIdleConnections()
	u.t = t
	for _, l := range u.locs {
		if err := l.hloc.SetTransport(u.t); err != nil {
			log.Errorf("Failed to set transport: %v", err)
		}
	}
	return nil
}

func (b *backend) updateServers(s []engine.Server) error {
	b.s = s
	for _, f := range u.fs {
		f.b = b
		if err := f.updateBackend(f.b); err != nil {
			log.Errorf("failed to update %v err: %s", l, err)
		}
	}
	return nil
}

func newBackend(m *mux, b *engine.Backend) (*backend, error) {
	o, err := m.getTransportOptions(b)
	if err != nil {
		return nil, err
	}
	t := httploc.NewTransport(*o)
	return &backend{
		m:    m,
		up:   *up,
		t:    t,
		locs: make(map[engine.FrontendKey]*frontend),
	}, nil
}
*/
