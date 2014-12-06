package command

import (
	"fmt"
	"sort"

	"github.com/mailgun/vulcand/engine"
)

// Detailed hosts view with all the information
func hostsView(hs []engine.Host) *StringTree {
	r := &StringTree{
		Node: "[hosts]",
	}
	for _, h := range hs {
		r.AddChild(hostView(&h))
	}
	return r
}

func hostView(h *engine.Host) *StringTree {
	host := &StringTree{
		Node: fmt.Sprintf("host[%s]", h.Name),
	}
	return host
}

func listenersView(ls []engine.Listener) *StringTree {
	r := &StringTree{
		Node: "[listeners]",
	}
	if len(ls) == 0 {
		return r
	}
	for _, l := range ls {
		r.AddChild(listenerView(&l))
	}
	return r
}

func listenerView(l *engine.Listener) *StringTree {
	return &StringTree{
		Node: fmt.Sprintf("listener[%s, %s, %s://%s]", l.Id, l.Protocol, l.Address.Network, l.Address.Address),
	}
}

func frontendsView(fs []engine.Frontend) *StringTree {
	r := &StringTree{
		Node: "[frontends]",
	}
	if len(fs) == 0 {
		return r
	}
	for _, f := range fs {
		r.AddChild(frontendView(&f))
	}
	return r
}

func frontendView(f *engine.Frontend) *StringTree {
	return &StringTree{
		Node: fmt.Sprintf("frontend[%s, %s]", f.Id, f.Route),
	}
}

func backendsView(bs []engine.Backend) *StringTree {
	r := &StringTree{
		Node: "[backends]",
	}
	for _, b := range bs {
		r.AddChild(backendView(&b))
	}
	return r
}

func backendView(b *engine.Backend) *StringTree {
	return &StringTree{
		Node: fmt.Sprintf("backend[%s]", b.Id),
	}
}

func serversView(srvs []engine.Server) *StringTree {
	r := &StringTree{
		Node: "[servers]",
	}
	for _, s := range srvs {
		r.AddChild(serverView(&s))
	}
	return r
}

func serverView(s *engine.Server) *StringTree {
	return &StringTree{
		Node: fmt.Sprintf("server[%s, %s]", s.Id, s.URL),
	}
}

func middlewaresView(ms []engine.Middleware) *StringTree {
	r := &StringTree{
		Node: "[middlewares]",
	}
	if len(ms) == 0 {
		return r
	}
	sort.Sort(&middlewareSorter{ms: ms})
	for _, m := range ms {
		r.AddChild(middlewareView(m))
	}
	return r
}

func middlewareView(m engine.Middleware) *StringTree {
	return &StringTree{
		Node: fmt.Sprintf("%s[%d, %s, %s]", m.Type, m.Priority, m.Id, m.Middleware),
	}
}

// Sorts middlewares by their priority
type middlewareSorter struct {
	ms []engine.Middleware
}

// Len is part of sort.Interface.
func (s *middlewareSorter) Len() int {
	return len(s.ms)
}

// Swap is part of sort.Interface.
func (s *middlewareSorter) Swap(i, j int) {
	s.ms[i], s.ms[j] = s.ms[j], s.ms[i]
}

func (s *middlewareSorter) Less(i, j int) bool {
	return s.ms[i].Priority < s.ms[j].Priority
}
