package engine

import (
	"encoding/json"
	"fmt"

	"github.com/mailgun/vulcand/plugin"
)

type rawServers struct {
	Servers []json.RawMessage
}

type rawBackends struct {
	Backends []json.RawMessage
}

type rawMiddlewares struct {
	Middlewares []json.RawMessage
}

type rawFrontends struct {
	Frontends []json.RawMessage
}

type rawHosts struct {
	Hosts []json.RawMessage
}

type rawListeners struct {
	Listeners []json.RawMessage
}

type rawFrontend struct {
	Id        string
	Type      string
	BackendId string
	Settings  json.RawMessage
}

type rawBackend struct {
	Id       string
	Type     string
	Settings json.RawMessage
}

type RawMiddleware struct {
	Id         string
	Type       string
	Priority   int
	Middleware json.RawMessage
}

func HostsFromJSON(in []byte) ([]Host, error) {
	var hs rawHosts
	err := json.Unmarshal(in, &hs)
	if err != nil {
		return nil, err
	}
	out := []Host{}
	if len(hs.Hosts) != 0 {
		for _, raw := range hs.Hosts {
			h, err := HostFromJSON(raw)
			if err != nil {
				return nil, err
			}
			out = append(out, *h)
		}
	}
	return out, nil
}

func FrontendsFromJSON(in []byte) ([]Frontend, error) {
	var rf *rawFrontends
	err := json.Unmarshal(in, &rf)
	if err != nil {
		return nil, err
	}
	out := make([]Frontend, len(rf.Frontends))
	for i, raw := range rf.Frontends {
		f, err := FrontendFromJSON(raw)
		if err != nil {
			return nil, err
		}
		out[i] = *f
	}
	return out, nil
}

func HostFromJSON(in []byte) (*Host, error) {
	var h *Host
	err := json.Unmarshal(in, &h)
	if err != nil {
		return nil, err
	}
	return NewHost(h.Name, h.Options)
}

func ListenerFromJSON(in []byte) (*Listener, error) {
	var l *Listener
	err := json.Unmarshal(in, &l)
	if err != nil {
		return nil, err
	}
	return NewListener(l.Id, l.Protocol, l.Address.Network, l.Address.Address)
}

func ListenersFromJSON(in []byte) ([]Listener, error) {
	var rls *rawListeners
	if err := json.Unmarshal(in, &rls); err != nil {
		return nil, err
	}
	out := make([]Listener, len(rls.Listeners))
	if len(out) == 0 {
		return out, nil
	}
	for i, rl := range rls.Listeners {
		l, err := ListenerFromJSON(rl)
		if err != nil {
			return nil, err
		}
		out[i] = *l
	}
	return out, nil
}

func KeyPairFromJSON(in []byte) (*KeyPair, error) {
	var c *KeyPair
	err := json.Unmarshal(in, &c)
	if err != nil {
		return nil, err
	}
	return NewKeyPair(c.Cert, c.Key)
}

func FrontendFromJSON(in []byte) (*Frontend, error) {
	var rf *rawFrontend
	if err := json.Unmarshal(in, &rf); err != nil {
		return nil, err
	}
	if rf.Type != HTTP {
		return nil, fmt.Errorf("Unsupported frontend type: %v", rf.Type)
	}
	var s *HTTPFrontendSettings
	if err := json.Unmarshal(rf.Settings, &s); err != nil {
		return nil, err
	}
	if s == nil {
		s = &HTTPFrontendSettings{}
	}
	f, err := NewHTTPFrontend(rf.Id, rf.BackendId, *s)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func MiddlewareFromJSON(in []byte, getter plugin.SpecGetter) (*Middleware, error) {
	var ms *RawMiddleware
	err := json.Unmarshal(in, &ms)
	if err != nil {
		return nil, err
	}
	spec := getter(ms.Type)
	if spec == nil {
		return nil, fmt.Errorf("middleware of type %s is not supported", ms.Type)
	}
	m, err := spec.FromJSON(ms.Middleware)
	if err != nil {
		return nil, err
	}
	return &Middleware{
		Id:         ms.Id,
		Type:       ms.Type,
		Middleware: m,
		Priority:   ms.Priority,
	}, nil
}

func BackendsFromJSON(in []byte) ([]Backend, error) {
	var rbs *rawBackends
	if err := json.Unmarshal(in, &rbs); err != nil {
		return nil, err
	}
	out := make([]Backend, len(rbs.Backends))
	if len(out) == 0 {
		return out, nil
	}
	for i, rb := range rbs.Backends {
		b, err := BackendFromJSON(rb)
		if err != nil {
			return nil, err
		}
		out[i] = *b
	}
	return out, nil
}

func BackendFromJSON(in []byte) (*Backend, error) {
	var rb *rawBackend

	if err := json.Unmarshal(in, &rb); err != nil {
		return nil, err
	}
	if rb.Type != HTTP {
		return nil, fmt.Errorf("Unsupported backend type %v", rb.Type)
	}

	var s *HTTPBackendSettings
	if err := json.Unmarshal(rb.Settings, &s); err != nil {
		return nil, err
	}
	if s == nil {
		s = &HTTPBackendSettings{}
	}
	b, err := NewHTTPBackend(rb.Id, *s)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func ServersFromJSON(in []byte) ([]Server, error) {
	var rs *rawServers
	if err := json.Unmarshal(in, &rs); err != nil {
		return nil, err
	}
	out := make([]Server, len(rs.Servers))
	if len(out) == 0 {
		return out, nil
	}
	for i, rs := range rs.Servers {
		s, err := ServerFromJSON(rs)
		if err != nil {
			return nil, err
		}
		out[i] = *s
	}
	return out, nil
}

func MiddlewaresFromJSON(in []byte, getter plugin.SpecGetter) ([]Middleware, error) {
	var rm *rawMiddlewares
	if err := json.Unmarshal(in, &rm); err != nil {
		return nil, err
	}
	out := make([]Middleware, len(rm.Middlewares))
	if len(out) == 0 {
		return out, nil
	}
	for i, r := range rm.Middlewares {
		m, err := MiddlewareFromJSON(r, getter)
		if err != nil {
			return nil, err
		}
		out[i] = *m
	}
	return out, nil
}

func ServerFromJSON(in []byte) (*Server, error) {
	var e *Server
	err := json.Unmarshal(in, &e)
	if err != nil {
		return nil, err
	}
	return NewServer(e.Id, e.URL)
}
