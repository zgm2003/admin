package provider

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

func publicAddress(ip net.IP) bool {
	return ip != nil && ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsUnspecified() && !(ip.To4() != nil && ip.To4()[0] == 100 && ip.To4()[1] >= 64 && ip.To4()[1] <= 127)
}
func sourceClient(input *http.Client) *http.Client {
	client := http.Client{Timeout: 20 * time.Second}
	if input != nil {
		client = *input
		if client.Timeout == 0 {
			client.Timeout = 20 * time.Second
		}
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return errors.New("AI source redirects are not allowed") }
	if client.Transport == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
		transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, errors.New("AI source address invalid")
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil || len(ips) == 0 {
				return nil, errors.New("AI source DNS unavailable")
			}
			for _, ip := range ips {
				if !publicAddress(ip.IP) {
					return nil, errors.New("AI source address is not public")
				}
			}
			var last error
			for _, ip := range ips {
				connection, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
				if err == nil {
					return connection, nil
				}
				last = err
			}
			return nil, last
		}
		client.Transport = transport
	}
	return &client
}
