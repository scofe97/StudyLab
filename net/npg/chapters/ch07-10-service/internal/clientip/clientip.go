// Package clientip 은 요청을 보낸 클라이언트의 IP 를 정한다.
//
// TCP 상대 주소(RemoteAddr)가 신뢰하는 프록시 대역 안에 있을 때만
// X-Forwarded-For 를 읽는다.
package clientip

import (
	"fmt"
	"net"
	"net/netip"
	"strings"
)

// ParsePrefixes 는 "10.0.0.0/8,127.0.0.1" 같은 쉼표 목록을 대역 목록으로 바꾼다.
// 접두 길이가 없는 주소는 /32(IPv6 는 /128) 로 본다.
func ParsePrefixes(s string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !strings.Contains(part, "/") {
			a, err := netip.ParseAddr(part)
			if err != nil {
				return nil, fmt.Errorf("%q: %w", part, err)
			}
			out = append(out, netip.PrefixFrom(a.Unmap(), a.Unmap().BitLen()))
			continue
		}
		p, err := netip.ParsePrefix(part)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", part, err)
		}
		out = append(out, p.Masked())
	}
	return out, nil
}

// Contains 는 a 가 목록의 어느 대역에 드는지 본다.
func Contains(list []netip.Prefix, a netip.Addr) bool {
	a = a.Unmap()
	for _, p := range list {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// Resolver 는 신뢰 대역과 X-Forwarded-For 를 읽는 방향을 갖는다.
type Resolver struct {
	trusted []netip.Prefix
	// rightmost 는 오른쪽부터 신뢰 대역을 건너뛰고 처음 만나는 주소를 고른다.
	// leftmost 는 목록의 맨 왼쪽 주소를 고른다.
	pick string
}

// New 는 Resolver 를 만든다. pick 은 rightmost 또는 leftmost 다.
func New(trusted []netip.Prefix, pick string) (*Resolver, error) {
	if pick != "rightmost" && pick != "leftmost" {
		return nil, fmt.Errorf("pick 은 rightmost 또는 leftmost: %q", pick)
	}
	return &Resolver{trusted: trusted, pick: pick}, nil
}

// RemoteIP 는 "host:port" 에서 IP 만 꺼낸다. 꺼낼 수 없으면 유효하지 않은 Addr 를 돌려준다.
func RemoteIP(remoteAddr string) netip.Addr {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	a, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return a.Unmap()
}

// splitXFF 는 여러 줄로 올 수 있는 X-Forwarded-For 값을 왼쪽부터 순서대로 펼친다.
func splitXFF(values []string) []string {
	var out []string
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			if p := strings.TrimSpace(part); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func parseHop(s string) (netip.Addr, bool) {
	if ap, err := netip.ParseAddrPort(s); err == nil {
		return ap.Addr().Unmap(), true
	}
	a, err := netip.ParseAddr(strings.Trim(s, "[]"))
	if err != nil {
		return netip.Addr{}, false
	}
	return a.Unmap(), true
}

// ClientIP 는 RemoteAddr 와 X-Forwarded-For 헤더 값들로 클라이언트 IP 를 정한다.
func (r *Resolver) ClientIP(remoteAddr string, xff []string) netip.Addr {
	remote := RemoteIP(remoteAddr)
	if !remote.IsValid() || len(r.trusted) == 0 || !Contains(r.trusted, remote) {
		return remote
	}
	hops := splitXFF(xff)
	if len(hops) == 0 {
		return remote
	}
	if r.pick == "leftmost" {
		for _, h := range hops {
			if a, ok := parseHop(h); ok {
				return a
			}
		}
		return remote
	}
	current := remote
	for i := len(hops) - 1; i >= 0; i-- {
		a, ok := parseHop(hops[i])
		if !ok {
			return current
		}
		if !Contains(r.trusted, a) {
			return a
		}
		current = a
	}
	return current
}
