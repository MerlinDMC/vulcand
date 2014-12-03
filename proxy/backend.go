package proxy

import (
	"fmt"
	"net/http"

	"github.com/mailgun/vulcand/engine"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
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

func (b *backend) String() string {
	return fmt.Sprintf("%v upstream(wrap=%v)", b.mux, &b.backend)
}

func (b *backend) linkFrontend(key engine.FrontendKey, f *frontend) {
	b.frontends[key] = f
}

func (b *backend) unlinkFrontend(key engine.FrontendKey) {
	delete(b.frontends, key)
}

func (b *backend) Close() error {
	b.transport.CloseIdleConnections()
	return nil
}

func (b *backend) update(be engine.Backend) error {
	if err := b.updateSettings(be); err != nil {
		return err
	}
	b.backend = be
	return nil
}

func (b *backend) updateSettings(be engine.Backend) error {
	olds := b.backend.HTTPSettings()
	news := be.HTTPSettings()

	// Nothing changed in transport options
	if news.Equals(olds) {
		return nil
	}
	o, err := b.mux.getTransportOptions(be)
	if err != nil {
		return err
	}
	t := httploc.NewTransport(*o)
	b.transport.CloseIdleConnections()
	b.transport = t
	for _, f := range b.frontends {
		if err := f.hloc.SetTransport(t); err != nil {
			log.Errorf("%v failed to set transport for %v, err: %v", b, f, err)
		}
	}
	return nil
}

func (b *backend) indexOfServer(id string) int {
	for i := range b.servers {
		if b.servers[i].Id == id {
			return i
		}
	}
	return -1
}

func (b *backend) upsertServer(s engine.Server) error {
	if i := b.indexOfServer(s.Id); i != -1 {
		b.servers[i] = s
	}
	b.servers = append(b.servers, s)
	return b.updateFrontends()
}

func (b *backend) deleteServer(sk engine.ServerKey) error {
	i := b.indexOfServer(sk.Id)
	if i == -1 {
		return fmt.Errorf("%v not found %v", b, sk)
	}
	b.servers = append(b.servers[:i], b.servers[i+1:]...)
	return b.updateFrontends()
}

func (b *backend) updateFrontends() error {
	for _, f := range b.frontends {
		if err := f.updateBackend(b); err != nil {
			return err
		}
	}
	return nil
}
