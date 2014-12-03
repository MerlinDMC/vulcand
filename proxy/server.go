package proxy

import (
	"fmt"
	"net/url"

	"github.com/mailgun/vulcand/engine"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/netutils"
)

type muxServer struct {
	url *url.URL
	id  string
	sk  engine.ServerKey

	mon *perfMon
}

func newMuxServer(sk engine.ServerKey, s *engine.Server, mon *perfMon) (*muxServer, error) {
	url, err := netutils.ParseUrl(s.URL)
	if err != nil {
		return nil, err
	}
	return &muxServer{id: s.Id, url: url, mon: mon}, nil
}

func (e *muxServer) String() string {
	return fmt.Sprintf("muxServer(id=%s, url=%s)", e.id, e.url.String())
}

func (e *muxServer) GetId() string {
	return e.id
}

func (e *muxServer) GetUrl() *url.URL {
	return e.url
}
