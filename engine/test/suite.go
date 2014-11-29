package test

import (
	"testing"
	"time"

	"github.com/mailgun/vulcand/engine"
	"github.com/mailgun/vulcand/plugin/connlimit"

	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestEtcd(t *testing.T) { TestingT(t) }

type EngineSuite struct {
	Engine   engine.Engine
	ChangesC chan interface{}
}

func (s *EngineSuite) collectChanges(c *C, expected int) []interface{} {
	changes := make([]interface{}, expected)
	for i, _ := range changes {
		select {
		case changes[i] = <-s.ChangesC:
			// successfully collected changes
		case <-time.After(2 * time.Second):
			c.Fatalf("Timeout occured")
		}
	}
	return changes
}

func (s *EngineSuite) expectChanges(c *C, expected ...interface{}) {
	changes := s.collectChanges(c, len(expected))
	for i, ch := range changes {
		c.Assert(ch, DeepEquals, expected[i])
	}
}

func (s *EngineSuite) makeConnLimit(id, variable string, conns int64) engine.Middleware {
	cl, err := connlimit.NewConnLimit(conns, variable)
	if err != nil {
		panic(err)
	}
	return engine.Middleware{
		Id:         id,
		Type:       "connlimit",
		Priority:   1,
		Middleware: cl,
	}
}

func (s *EngineSuite) HostCRUD(c *C) {
	host := engine.Host{Name: "localhost"}

	c.Assert(s.Engine.UpsertHost(host), IsNil)
	s.expectChanges(c, &engine.HostUpserted{Host: host})

	hs, err := s.Engine.GetHosts()
	c.Assert(err, IsNil)
	c.Assert(hs, DeepEquals, []engine.Host{host})

	hk := engine.HostKey{Name: "localhost"}
	c.Assert(s.Engine.DeleteHost(hk), IsNil)

	s.expectChanges(c, &engine.HostDeleted{HostKey: hk})
}

func (s *EngineSuite) HostWithKeyPair(c *C) {
	host := engine.Host{Name: "localhost"}

	host.Options.Default = true
	host.Options.KeyPair = &engine.KeyPair{
		Key:  []byte("hello"),
		Cert: []byte("world"),
	}

	c.Assert(s.Engine.UpsertHost(host), IsNil)
	s.expectChanges(c, &engine.HostUpserted{Host: host})

	hk := engine.HostKey{Name: host.Name}
	c.Assert(s.Engine.DeleteHost(hk), IsNil)

	s.expectChanges(c, &engine.HostDeleted{
		HostKey: hk,
	})
}

func (s *EngineSuite) HostUpsertKeyPair(c *C) {
	host := engine.Host{Name: "localhost"}

	c.Assert(s.Engine.UpsertHost(host), IsNil)

	hostNoKeyPair := host
	hostNoKeyPair.Options.KeyPair = nil

	host.Options.KeyPair = &engine.KeyPair{
		Key:  []byte("hello"),
		Cert: []byte("world"),
	}
	c.Assert(s.Engine.UpsertHost(host), IsNil)

	s.expectChanges(c,
		&engine.HostUpserted{Host: hostNoKeyPair},
		&engine.HostUpserted{Host: host})
}

func (s *EngineSuite) ListenerCRUD(c *C) {
	host := engine.Host{Name: "localhost"}

	listener := engine.Listener{
		Id:       "l1",
		Protocol: "http",
		Address: engine.Address{
			Network: "tcp",
			Address: "127.0.0.1:9000",
		},
	}

	hk := engine.HostKey{Name: "localhost"}
	c.Assert(s.Engine.UpsertHost(host), IsNil)
	c.Assert(s.Engine.UpsertListener(hk, listener), IsNil)
	lk := engine.ListenerKey{HostKey: hk, Id: listener.Id}

	out, err := s.Engine.GetListener(lk)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, &listener)

	ls, err := s.Engine.GetListeners(hk)
	c.Assert(err, IsNil)
	c.Assert(ls, DeepEquals, []engine.Listener{listener})

	s.expectChanges(c,
		&engine.HostUpserted{Host: host},
		&engine.ListenerUpserted{HostKey: hk, Listener: listener},
	)
	c.Assert(s.Engine.DeleteListener(lk), IsNil)

	s.expectChanges(c,
		&engine.ListenerDeleted{ListenerKey: lk},
	)
}

func (s *EngineSuite) BackendCRUD(c *C) {
	b := engine.Backend{Id: "b1", Type: engine.HTTP, Settings: engine.HTTPBackendSettings{}}

	c.Assert(s.Engine.UpsertBackend(b), IsNil)

	s.expectChanges(c, &engine.BackendUpserted{Backend: b})

	bk := engine.BackendKey{Id: b.Id}

	out, err := s.Engine.GetBackend(bk)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, &b)

	bs, err := s.Engine.GetBackends()
	c.Assert(len(bs), Equals, 1)
	c.Assert(bs[0], DeepEquals, b)

	b.Settings = engine.HTTPBackendSettings{Timeouts: engine.HTTPBackendTimeouts{Read: "1s"}}

	c.Assert(s.Engine.UpsertBackend(b), IsNil)

	s.expectChanges(c, &engine.BackendUpserted{Backend: b})

	err = s.Engine.DeleteBackend(bk)
	c.Assert(err, IsNil)

	s.expectChanges(c, &engine.BackendDeleted{
		BackendKey: bk,
	})
}

func (s *EngineSuite) BackendDeleteUsed(c *C) {
	b := engine.Backend{Id: "b0", Type: engine.HTTP, Settings: engine.HTTPBackendSettings{}}
	c.Assert(s.Engine.UpsertBackend(b), IsNil)

	f := engine.Frontend{
		Id:        "f1",
		BackendId: b.Id,
		Type:      engine.HTTP,
		Settings:  engine.HTTPFrontendSettings{Route: `Path("/hello")`},
	}
	c.Assert(s.Engine.UpsertFrontend(f, 0), IsNil)

	s.collectChanges(c, 2)

	c.Assert(s.Engine.DeleteBackend(engine.BackendKey{Id: b.Id}), NotNil)
}

func (s *EngineSuite) ServerCRUD(c *C) {
	b := engine.Backend{Id: "b0", Type: engine.HTTP, Settings: engine.HTTPBackendSettings{}}

	c.Assert(s.Engine.UpsertBackend(b), IsNil)

	s.expectChanges(c, &engine.BackendUpserted{Backend: b})

	srv := engine.Server{Id: "srv0", URL: "http://localhost:1000"}
	bk := engine.BackendKey{Id: b.Id}
	sk := engine.ServerKey{BackendKey: bk, Id: srv.Id}

	c.Assert(s.Engine.UpsertServer(bk, srv, 0), IsNil)

	srvo, err := s.Engine.GetServer(sk)
	c.Assert(err, IsNil)
	c.Assert(srvo, DeepEquals, &srv)

	srvs, err := s.Engine.GetServers(bk)
	c.Assert(err, IsNil)
	c.Assert(srvs, DeepEquals, []engine.Server{srv})

	s.expectChanges(c, &engine.ServerUpserted{
		BackendKey: bk,
		Server:     srv,
	})

	err = s.Engine.DeleteServer(sk)
	c.Assert(err, IsNil)

	s.expectChanges(c, &engine.ServerDeleted{
		ServerKey: sk,
	})
}

func (s *EngineSuite) ServerExpire(c *C) {
	b := engine.Backend{Id: "b0", Type: engine.HTTP, Settings: engine.HTTPBackendSettings{}}

	c.Assert(s.Engine.UpsertBackend(b), IsNil)
	s.collectChanges(c, 1)

	srv := engine.Server{Id: "srv0", URL: "http://localhost:1000"}
	bk := engine.BackendKey{Id: b.Id}
	sk := engine.ServerKey{BackendKey: bk, Id: srv.Id}
	c.Assert(s.Engine.UpsertServer(bk, srv, time.Second), IsNil)

	s.expectChanges(c,
		&engine.ServerUpserted{
			BackendKey: bk,
			Server:     srv,
		}, &engine.ServerDeleted{
			ServerKey: sk,
		})
}

func (s *EngineSuite) FrontendCRUD(c *C) {
	b := engine.Backend{Id: "b0", Type: engine.HTTP, Settings: engine.HTTPBackendSettings{}}
	c.Assert(s.Engine.UpsertBackend(b), IsNil)
	s.collectChanges(c, 1)

	f := engine.Frontend{
		Id:        "f1",
		BackendId: b.Id,
		Type:      engine.HTTP,
		Settings:  engine.HTTPFrontendSettings{Route: `Path("/hello")`},
	}

	c.Assert(s.Engine.UpsertFrontend(f, 0), IsNil)

	fk := engine.FrontendKey{Id: f.Id}
	out, err := s.Engine.GetFrontend(fk)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, &f)

	s.expectChanges(c, &engine.FrontendUpserted{
		Frontend: f,
	})

	// Make some updates
	b1 := engine.Backend{Id: "b1", Type: engine.HTTP, Settings: engine.HTTPBackendSettings{}}
	c.Assert(s.Engine.UpsertBackend(b1), IsNil)
	s.collectChanges(c, 1)

	f.BackendId = "b1"
	f.Settings = engine.HTTPFrontendSettings{
		Route: `Path("/hello")`,
		Options: engine.HTTPFrontendOptions{
			Hostname: "host1",
		},
	}
	c.Assert(s.Engine.UpsertFrontend(f, 0), IsNil)

	out, err = s.Engine.GetFrontend(fk)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, &f)

	s.expectChanges(c, &engine.FrontendUpserted{
		Frontend: f,
	})

	// Delete
	c.Assert(s.Engine.DeleteFrontend(fk), IsNil)
	s.expectChanges(c, &engine.FrontendDeleted{
		FrontendKey: fk,
	})
}

func (s *EngineSuite) FrontendExpire(c *C) {
	b := engine.Backend{Id: "b0", Type: engine.HTTP, Settings: engine.HTTPBackendSettings{}}
	c.Assert(s.Engine.UpsertBackend(b), IsNil)
	s.collectChanges(c, 1)

	f := engine.Frontend{
		Id:        "f1",
		BackendId: b.Id,
		Type:      engine.HTTP,
		Settings:  engine.HTTPFrontendSettings{Route: `Path("/hello")`},
	}
	c.Assert(s.Engine.UpsertFrontend(f, time.Second), IsNil)

	fk := engine.FrontendKey{Id: f.Id}
	s.expectChanges(c,
		&engine.FrontendUpserted{
			Frontend: f,
		}, &engine.FrontendDeleted{
			FrontendKey: fk,
		})
}

func (s *EngineSuite) FrontendBadBackend(c *C) {
	c.Assert(
		s.Engine.UpsertFrontend(engine.Frontend{
			Id:        "f1",
			Type:      engine.HTTP,
			BackendId: "Nonexistent",
			Settings:  engine.HTTPFrontendSettings{Route: `Path("/hello")`},
		}, 0),
		NotNil)
}

func (s *EngineSuite) MiddlewareCRUD(c *C) {
	b := engine.Backend{Id: "b1", Type: engine.HTTP, Settings: engine.HTTPBackendSettings{}}
	c.Assert(s.Engine.UpsertBackend(b), IsNil)

	f := engine.Frontend{
		Id:        "f1",
		Type:      engine.HTTP,
		Settings:  engine.HTTPFrontendSettings{Route: `Path("/hello")`},
		BackendId: b.Id,
	}
	c.Assert(s.Engine.UpsertFrontend(f, 0), IsNil)
	s.collectChanges(c, 2)

	fk := engine.FrontendKey{Id: f.Id}
	m := s.makeConnLimit("cl1", "client.ip", 10)
	c.Assert(s.Engine.UpsertMiddleware(fk, m, 0), IsNil)
	s.expectChanges(c, &engine.MiddlewareUpserted{
		FrontendKey: fk,
		Middleware:  m,
	})

	mk := engine.MiddlewareKey{Id: m.Id, FrontendKey: fk}
	out, err := s.Engine.GetMiddleware(mk)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, &m)

	// Let us upsert middleware
	m.Middleware.(*connlimit.ConnLimit).Connections = 100
	c.Assert(s.Engine.UpsertMiddleware(fk, m, 0), IsNil)
	s.expectChanges(c, &engine.MiddlewareUpserted{
		FrontendKey: fk,
		Middleware:  m,
	})

	ms, err := s.Engine.GetMiddlewares(fk)
	c.Assert(err, IsNil)
	c.Assert(ms, DeepEquals, []engine.Middleware{m})

	c.Assert(s.Engine.DeleteMiddleware(mk), IsNil)

	s.expectChanges(c, &engine.MiddlewareDeleted{
		MiddlewareKey: mk,
	})
}

func (s *EngineSuite) MiddlewareExpire(c *C) {
	b := engine.Backend{Id: "b1", Type: engine.HTTP, Settings: engine.HTTPBackendSettings{}}
	c.Assert(s.Engine.UpsertBackend(b), IsNil)

	f := engine.Frontend{
		Id:        "f1",
		BackendId: b.Id,
		Type:      engine.HTTP,
		Settings:  engine.HTTPFrontendSettings{Route: `Path("/hello")`},
	}
	c.Assert(s.Engine.UpsertFrontend(f, 0), IsNil)
	s.collectChanges(c, 2)

	fk := engine.FrontendKey{Id: f.Id}
	m := s.makeConnLimit("cl1", "client.ip", 10)
	c.Assert(s.Engine.UpsertMiddleware(fk, m, time.Second), IsNil)
	s.expectChanges(c, &engine.MiddlewareUpserted{
		FrontendKey: fk,
		Middleware:  m,
	})

	mk := engine.MiddlewareKey{Id: m.Id, FrontendKey: fk}
	s.expectChanges(c, &engine.MiddlewareDeleted{
		MiddlewareKey: mk,
	})
}

func (s *EngineSuite) MiddlewareBadFrontend(c *C) {
	fk := engine.FrontendKey{Id: "wrong"}
	m := s.makeConnLimit("cl1", "client.ip", 10)
	c.Assert(s.Engine.UpsertMiddleware(fk, m, 0), NotNil)
}

func (s *EngineSuite) MiddlewareBadType(c *C) {
	fk := engine.FrontendKey{Id: "wrong"}
	m := s.makeConnLimit("cl1", "client.ip", 10)

	b := engine.Backend{Id: "b1", Type: engine.HTTP, Settings: engine.HTTPBackendSettings{}}
	c.Assert(s.Engine.UpsertBackend(b), IsNil)

	f := engine.Frontend{
		Id:        "f1",
		Type:      engine.HTTP,
		BackendId: b.Id,
		Settings:  engine.HTTPFrontendSettings{Route: `Path("/hello")`},
	}
	c.Assert(s.Engine.UpsertFrontend(f, 0), IsNil)
	s.collectChanges(c, 2)
	m.Type = "blabla"
	c.Assert(s.Engine.UpsertMiddleware(fk, m, 0), NotNil)
}
