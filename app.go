package candy

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hashicorp/go-multierror"
)

type App struct {
	Host     string
	Protocol string
	Addr     string
}

type AppServiceConfig struct {
	TLDs      []string
	DomainDir string
}

func NewAppService(cfg AppServiceConfig) *AppService {
	return &AppService{cfg: cfg}
}

type AppService struct {
	cfg AppServiceConfig
}

func (f *AppService) FindApps() ([]App, error) {
	files, err := ioutil.ReadDir(f.cfg.DomainDir)
	if err != nil {
		return nil, err
	}

	var (
		result []App
		merr   *multierror.Error
	)

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		b, err := ioutil.ReadFile(filepath.Join(f.cfg.DomainDir, file.Name()))
		if err != nil {
			return nil, err
		}

		apps, err := f.parseApps(file.Name(), strings.TrimSpace(string(b)))
		if err != nil {
			merr = multierror.Append(merr, err)
			continue
		}

		result = append(result, apps...)
	}

	return result, err
}

func (f *AppService) parseApps(domain, data string) ([]App, error) {
	// port
	port, err := strconv.Atoi(data)
	if err == nil {
		return f.buildApps(domain, "http", fmt.Sprintf("127.0.0.1:%d", port)), nil
	}

	// ip:port
	host, sport, err := net.SplitHostPort(data)
	if err == nil {
		return f.buildApps(domain, "http", host+":"+sport), nil
	}

	// http://ip:port
	u, err := url.Parse(data)
	if err == nil {
		return f.buildApps(domain, u.Scheme, u.Host), nil
	}

	// TODO: json
	return nil, fmt.Errorf("invalid domain for file: %s", filepath.Join(f.cfg.DomainDir, domain))
}

func (f *AppService) buildApps(domain, protocol, addr string) []App {
	var apps []App
	for _, tld := range f.cfg.TLDs {
		apps = append(apps, App{
			Host:     domain + "." + tld, // e.g., app.test
			Protocol: protocol,
			Addr:     addr,
		})
	}

	return apps
}
