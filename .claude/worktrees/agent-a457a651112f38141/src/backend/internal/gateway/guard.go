package gateway

// Outbound URL guard (constitution C-10, docs/specs/w3-ssrf-guard/spec.md).
//
// The action pipeline takes URLs from the database. This guard makes sure a call
// can only reach a public HTTPS host: even if the configuration is wrong or
// hostile, even through DNS tricks and redirects.
//
//   - scheme is https only; no credentials in the URL; a real host; no control characters;
//   - every address the host resolves to must be public, and the check happens in the
//     dialer on the address actually used (defeats DNS rebinding);
//   - redirects are not followed;
//   - an optional allowlist (OUTBOUND_ALLOWED_HOSTS) narrows the destinations further.

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"unicode"
)

// ErrBlockedURL is returned for a URL, method or resolved address the guard refuses.
// It is terminal: retrying cannot make a blocked destination acceptable.
var ErrBlockedURL = errors.New("outbound destination blocked")

// GuardOptions configures a Guard. Resolve is injectable so tests need no DNS.
type GuardOptions struct {
	// AllowedHosts is a comma separated list of exact host names or "*.example.com"
	// suffixes. Empty means any public host is allowed.
	AllowedHosts string
	// Resolve returns the addresses for a host. Nil uses the system resolver.
	Resolve func(ctx context.Context, host string) ([]net.IP, error)
	// allowLoopback is a test-only seam (unexported: production code cannot set it).
	allowLoopback bool
}

// Guard validates outbound destinations.
type Guard struct {
	allowed       []string
	resolve       func(ctx context.Context, host string) ([]net.IP, error)
	allowLoopback bool
}

// NewGuard builds a Guard from options.
func NewGuard(o GuardOptions) *Guard {
	g := &Guard{resolve: o.Resolve, allowLoopback: o.allowLoopback}
	if g.resolve == nil {
		g.resolve = func(ctx context.Context, host string) ([]net.IP, error) {
			addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			out := make([]net.IP, 0, len(addrs))
			for _, a := range addrs {
				out = append(out, a.IP)
			}
			return out, nil
		}
	}
	for _, h := range strings.Split(o.AllowedHosts, ",") {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			g.allowed = append(g.allowed, h)
		}
	}
	return g
}

func blocked(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrBlockedURL, fmt.Sprintf(format, a...))
}

// CheckURL validates everything that can be decided without network I/O.
func (g *Guard) CheckURL(raw string) error {
	for _, r := range raw {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return blocked("URL contains control or space characters")
		}
	}
	u, err := url.Parse(raw)
	if err != nil || raw == "" {
		return blocked("URL is not valid")
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return blocked("scheme %q is not allowed (https only)", u.Scheme)
	}
	if u.User != nil {
		return blocked("credentials in the URL are not allowed")
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return blocked("URL has no host")
	}
	if ip := net.ParseIP(host); ip != nil {
		if g.isBlocked(ip) {
			return blocked("address %s is not a public address", ip)
		}
	}
	if len(g.allowed) > 0 && !g.hostAllowed(host) {
		return blocked("host %q is not in the allowlist", host)
	}
	return nil
}

func (g *Guard) isBlocked(ip net.IP) bool {
	if g.allowLoopback && ip.IsLoopback() {
		return false
	}
	return isBlockedIP(ip)
}

func (g *Guard) hostAllowed(host string) bool {
	for _, a := range g.allowed {
		if strings.HasPrefix(a, "*.") {
			if strings.HasSuffix(host, a[1:]) && len(host) > len(a)-1 {
				return true
			}
			continue
		}
		if host == a {
			return true
		}
	}
	return false
}

// safeDialAddrs resolves host and returns its addresses, or ErrBlockedURL if ANY of
// them is not public, or the resolution fails (fail closed).
func (g *Guard) safeDialAddrs(ctx context.Context, host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		if g.isBlocked(ip) {
			return nil, blocked("address %s is not a public address", ip)
		}
		return []net.IP{ip}, nil
	}
	ips, err := g.resolve(ctx, host)
	if err != nil {
		return nil, blocked("cannot resolve %q", host)
	}
	if len(ips) == 0 {
		return nil, blocked("%q resolves to no address", host)
	}
	for _, ip := range ips {
		if g.isBlocked(ip) {
			return nil, blocked("%q resolves to a non-public address", host)
		}
	}
	return ips, nil
}

// DialContext is a net.Dialer replacement that only connects to addresses that
// passed the check, using the validated IP (not the name) so a second resolution
// cannot return something different.
func (g *Guard) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, blocked("bad address")
	}
	ips, err := g.safeDialAddrs(ctx, host)
	if err != nil {
		return nil, err
	}
	var d net.Dialer
	var lastErr error
	for _, ip := range ips {
		conn, derr := d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if derr == nil {
			return conn, nil
		}
		lastErr = derr
	}
	return nil, lastErr
}

var privateNets = func() []*net.IPNet {
	var out []*net.IPNet
	for _, c := range []string{
		"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", // RFC 1918
		"100.64.0.0/10",  // carrier-grade NAT
		"169.254.0.0/16", // link-local, including the cloud metadata address
		"fc00::/7",       // unique local IPv6
		"fe80::/10",      // link-local IPv6
	} {
		_, n, _ := net.ParseCIDR(c)
		out = append(out, n)
	}
	return out
}()

// isBlockedIP reports whether ip is anything other than a public unicast address.
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if v4 := ip.To4(); v4 != nil { // also unwraps IPv4-mapped IPv6 such as ::ffff:10.0.0.1
		ip = v4
	}
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil && v4.Equal(net.IPv4bcast) {
		return true
	}
	for _, n := range privateNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
