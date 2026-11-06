package proxy

import (
	"fmt"
	"net"
	"net/netip"
	"strings"
)

// TrustedProxies 는 X-Forwarded-For 를 이어 써도 되는 바로 앞 홉의 주소 대역이다.
// 비어 있으면 아무도 믿지 않는다 (ADR 0017).
type TrustedProxies []netip.Prefix

// ParseTrustedProxies 는 쉼표로 나눈 CIDR 목록("127.0.0.1/32, 10.0.0.0/8")을 읽는다.
// 빈 문자열이면 빈 목록을 돌려준다.
func ParseTrustedProxies(s string) (TrustedProxies, error) {
	var out TrustedProxies
	if strings.TrimSpace(s) == "" {
		return out, nil
	}

	for _, part := range strings.Split(s, ",") {
		p, err := netip.ParsePrefix(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("믿는 대역 %q: %w", part, err)
		}
		out = append(out, p)
	}
	return out, nil
}

// Trusts 는 바로 앞 홉의 주소(r.RemoteAddr, "ip:port" 꼴)가 믿는 대역 안에 있는지 본다.
// 꼴이 틀리면 믿지 않는다.
func (t TrustedProxies) Trusts(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	for _, p := range t {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}
