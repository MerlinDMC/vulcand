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
		r.AddChild(hostView(h))
	}
	return r
}

func hostView(h engine.Host) *StringTree {
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
		r.AddChild(
			&StringTree{
				Node: fmt.Sprintf("listener[%s, %s, %s://%s]", l.Id, l.Protocol, l.Address.Network, l.Address.Address),
			})
	}
	return r
}

func frontendsView(ls []engine.Frontend) *StringTree {
	r := &StringTree{
		Node: "[frontends]",
	}
	if len(ls) == 0 {
		return r
	}
	for _, l := range ls {
		r.AddChild(frontendView(l))
	}
	return r
}

func frontendView(l engine.Frontend) *StringTree {
	return &StringTree{
		Node: fmt.Sprintf("frontend[%s, %s]", l.Id, l.Route),
	}
	return r
}

func backendsView(bs []engine.Backend) *StringTree {
	r := &StringTree{
		Node: "[backends]",
	}
	for _, b := range bs {
		r.AddChild(backendView(b))
	}
	return r
}

func backendView(b engine.Backend) *StringTree {
	r := &StringTree{
		Node: fmt.Sprintf("backend[%s]", b.Id),
	}

	for _, s := range b.Servers {
		r.AddChild(serverView(s))
	}
	return r
}

func serverView(s engine.Server) *StringTree {
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
