// Package restrictprefix 는 경로 세그먼트가 금지 접두사로 시작하는 요청을
// 앞단에서 끊는 Caddy HTTP 미들웨어 모듈이다.
//
// 노트 10-01 §5 의 restrict_prefix 를 바탕으로 두 가지를 바꿨다.
//   - 접두사를 하나(prefix)가 아니라 여러 개(prefixes) 받는다.
//   - 기본값은 Provision 에서 채우고, Validate 는 검사만 한다.
package restrictprefix

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
)

func init() {
	// JSON 에서 "handler": "restrict_prefix" 로 부를 수 있게 등록한다
	caddy.RegisterModule(RestrictPrefix{})
	// Caddyfile 에서 restrict_prefix 지시어로 부를 수 있게 등록한다
	httpcaddyfile.RegisterHandlerDirective("restrict_prefix", parseCaddyfile)
}

// RestrictPrefix 는 URL 경로를 "/" 로 자른 세그먼트 가운데 하나라도
// Prefixes 의 어느 접두사로 시작하면 다음 핸들러로 넘기지 않는다.
type RestrictPrefix struct {
	// 막을 세그먼트 접두사 목록. 비어 있으면 Provision 이 "." 하나로 채운다.
	Prefixes []string `json:"prefixes,omitempty"`

	logger *zap.Logger
}

// CaddyModule 은 모듈 ID 와 새 인스턴스를 만드는 함수를 돌려준다.
func (RestrictPrefix) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.restrict_prefix",
		New: func() caddy.Module { return new(RestrictPrefix) },
	}
}

// Provision 은 JSON 을 구조체에 채운 직후 불린다. 로거를 받고 기본값을 채운다.
func (p *RestrictPrefix) Provision(ctx caddy.Context) error {
	// 인자 없는 ctx.Logger() 는 지금 프로비저닝 중인 모듈의 이름을 붙인 로거를 준다
	p.logger = ctx.Logger()
	if len(p.Prefixes) == 0 {
		p.Prefixes = []string{"."}
	}
	return nil
}

// Validate 는 Provision 다음에 불린다. 설정을 고치지 않고 검사만 한다.
func (p *RestrictPrefix) Validate() error {
	for i, prefix := range p.Prefixes {
		if prefix == "" {
			return fmt.Errorf("prefixes[%d]: 빈 문자열은 모든 세그먼트와 맞는다", i)
		}
		// 세그먼트는 "/" 로 자른 조각이므로 "/" 를 품은 접두사와는 비교할 일이 없다
		if strings.Contains(prefix, "/") {
			return fmt.Errorf("prefixes[%d] %q: 접두사는 세그먼트 하나를 가리키므로 '/' 를 넣을 수 없다", i, prefix)
		}
	}
	return nil
}

// ServeHTTP 는 요청마다 불린다. 막을 때는 응답을 직접 쓰지 않고 오류를 돌려준다.
func (p RestrictPrefix) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	if segment, prefix, ok := p.match(r.URL.Path); ok {
		p.logger.Debug("restricted path segment",
			zap.String("path", r.URL.Path),
			zap.String("segment", segment),
			zap.String("prefix", prefix))
		return caddyhttp.Error(http.StatusNotFound,
			fmt.Errorf("restricted segment %q (prefix %q)", segment, prefix))
	}
	return next.ServeHTTP(w, r)
}

// match 는 path 의 세그먼트 가운데 처음으로 접두사에 걸린 것을 돌려준다.
func (p RestrictPrefix) match(path string) (segment, prefix string, ok bool) {
	for _, segment := range strings.Split(path, "/") {
		for _, prefix := range p.Prefixes {
			if strings.HasPrefix(segment, prefix) {
				return segment, prefix, true
			}
		}
	}
	return "", "", false
}

// UnmarshalCaddyfile 은 다음 문법을 읽는다.
//
//	restrict_prefix [<prefix>...]
func (p *RestrictPrefix) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	// 첫 토큰은 지시어 이름이다
	d.Next()
	p.Prefixes = append(p.Prefixes, d.RemainingArgs()...)
	if d.NextBlock(0) {
		return d.Err("restrict_prefix 는 블록을 받지 않는다")
	}
	return nil
}

func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	p := new(RestrictPrefix)
	err := p.UnmarshalCaddyfile(h.Dispenser)
	return p, err
}

// 필요한 인터페이스를 모두 구현했는지 컴파일 때 확인한다
var (
	_ caddy.Provisioner           = (*RestrictPrefix)(nil)
	_ caddy.Validator             = (*RestrictPrefix)(nil)
	_ caddyhttp.MiddlewareHandler = (*RestrictPrefix)(nil)
	_ caddyfile.Unmarshaler       = (*RestrictPrefix)(nil)
)
