package dns

import (
	"context"
	"net"
	"time"

	"github.com/miekg/dns"
	"github.com/oklog/run"
	"github.com/owenthereal/candy"
)

type Config struct {
	Addr string
}

func New(cfg Config) candy.DNSServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &dnsServer{
		udp:    &dns.Server{Addr: cfg.Addr, Net: "udp"},
		tcp:    &dns.Server{Addr: cfg.Addr, Net: "tcp"},
		ctx:    ctx,
		cancel: cancel,
	}
}

type dnsServer struct {
	udp    *dns.Server
	tcp    *dns.Server
	ctx    context.Context
	cancel context.CancelFunc
}

func (d *dnsServer) Start(cfg candy.DNSServerConfig) error {
	for _, domain := range cfg.Domains {
		dns.HandleFunc(domain+".", d.handleDNS)
	}

	var g run.Group
	{
		g.Add(func() error {
			return d.udp.ListenAndServe()
		}, func(err error) {
			d.Shutdown()
		})
	}
	{
		g.Add(func() error {
			return d.tcp.ListenAndServe()
		}, func(err error) {
			d.Shutdown()
		})
	}
	{
		g.Add(func() error {
			<-d.ctx.Done()
			return d.ctx.Err()
		}, func(err error) {
			d.Shutdown()
		})
	}

	return g.Run()
}

func (d *dnsServer) Shutdown() error {
	defer d.cancel()

	candy.Log().Info("shutting down DNS server")

	d.udp.Shutdown()
	d.tcp.Shutdown()

	return nil
}

func (d *dnsServer) handleDNS(w dns.ResponseWriter, r *dns.Msg) {
	var (
		v4 bool
		rr dns.RR
		a  net.IP
	)

	dom := r.Question[0].Name

	m := new(dns.Msg)
	m.SetReply(r)

	if ip, ok := w.RemoteAddr().(*net.UDPAddr); ok {
		a = ip.IP
		v4 = a.To4() != nil
	}
	if ip, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		a = ip.IP
		v4 = a.To4() != nil
	}

	if v4 {
		rr = &dns.A{
			Hdr: dns.RR_Header{Name: dom, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 0},
			A:   a.To4(),
		}
	} else {
		rr = &dns.AAAA{
			Hdr:  dns.RR_Header{Name: dom, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 0},
			AAAA: a,
		}
	}

	switch r.Question[0].Qtype {
	case dns.TypeAAAA, dns.TypeA:
		m.Answer = append(m.Answer, rr)
	}

	if r.IsTsig() != nil {
		if w.TsigStatus() == nil {
			m.SetTsig(r.Extra[len(r.Extra)-1].(*dns.TSIG).Hdr.Name, dns.HmacMD5, 300, time.Now().Unix())
		}
	}

	w.WriteMsg(m)
}
