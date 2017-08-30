package websetup

import (
	"context"
	"errors"
	"flamenco-manager/flamenco"
	"fmt"
	"net"
	"net/url"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	// ErrNoInterface is returned when no network interfaces with a real IP-address were found.
	ErrNoInterface = errors.New("No network interface found")
)

func networkInterfaces(includeLinkLocal bool) ([]net.IP, error) {
	log.Debug("Iterating over all network interfaces.")

	interfaces, err := net.Interfaces()
	if err != nil {
		return []net.IP{}, err
	}

	usableAddresses := make([]net.IP, 0)
	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for k := range addrs {
			var ip net.IP
			switch a := addrs[k].(type) {
			case *net.IPAddr:
				ip = a.IP.To16()
			case *net.IPNet:
				ip = a.IP.To16()
			default:
				log.Debugf("    - skipping unknown type %v", addrs[k])
				continue
			}

			if ip.IsMulticast() {
				log.Debugf("    - skipping multicast %v", ip)
				continue
			}
			if ip.IsUnspecified() {
				log.Debugf("    - skipping unspecified %v", ip)
				continue
			}
			if !includeLinkLocal && ip.IsLinkLocalUnicast() {
				log.Debugf("    - skipping link-local %v", ip)
				continue
			}

			log.Debugf("    - usable %v", ip)
			usableAddresses = append(usableAddresses, ip)
		}
	}

	if len(usableAddresses) == 0 {
		return usableAddresses, ErrNoInterface
	}

	return usableAddresses, nil
}

func availableURLs(config *flamenco.Conf) ([]*url.URL, error) {
	var schema string
	if config.HasTLS() {
		schema = "https"
	} else {
		schema = "http"
	}

	var (
		host, port string
		portnum    int
		err        error
	)

	if config.Listen == "" {
		panic("Empty config.Listen")
	}

	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second*10)
	defer ctxCancel()

	// Figure out which port we're supposted to listen on.
	if host, port, err = net.SplitHostPort(config.Listen); err != nil {
		return nil, fmt.Errorf("Unable to split host and port in address '%s'", config.Listen)
	}
	if portnum, err = net.DefaultResolver.LookupPort(ctx, "listen", port); err != nil {
		return nil, fmt.Errorf("Unable to look up port '%s'", port)
	}

	// If the host is empty or ::0/0.0.0.0, show a list of URLs to connect to.
	listenSpecificHost := false
	var ip net.IP
	if host != "" {
		ip = net.ParseIP(host)
		if ip == nil {
			addrs, erresolve := net.DefaultResolver.LookupHost(ctx, host)
			if erresolve != nil {
				return nil, fmt.Errorf("Unable to resolve listen host '%v': %s", host, erresolve)
			}
			if len(addrs) > 0 {
				ip = net.ParseIP(addrs[0])
			}
		}
		if ip != nil && !ip.IsUnspecified() {
			listenSpecificHost = true
		}
	}

	if listenSpecificHost {
		log.Debugf("Listening on host %v", ip)
		// We can just construct a URL here, since we know it's a specific host anyway.

		link := fmt.Sprintf("%s://%s:%d/", schema, host, portnum)
		myURL, errparse := url.Parse(link)
		if errparse != nil {
			return nil, fmt.Errorf("Unable to parse listen URL %s: %s", link, errparse)
		}
		return []*url.URL{myURL}, nil
	}

	log.Debugf("Not listening on any specific host '%v'", host)

	addrs, err := networkInterfaces(false)
	if err == ErrNoInterface {
		addrs, err = networkInterfaces(true)
	}
	if err != nil {
		return nil, err
	}

	log.Debugf("Iterating network interfaces to find possible URLs for Flamenco Manager.")

	links := make([]*url.URL, 0)
	for _, addr := range addrs {
		var strAddr string
		if ipv4 := addr.To4(); ipv4 != nil {
			strAddr = ipv4.String()
		} else {
			strAddr = fmt.Sprintf("[%s]", addr)
		}

		link := fmt.Sprintf("%s://%s:%d/", schema, strAddr, portnum)
		myURL, err := url.Parse(link)
		if err != nil {
			log.Warningf("Skipping address %s, as it results in an unparseable URL %s: %s", addr, link, err)
			continue
		}
		links = append(links, myURL)
	}

	return links, nil
}
