// fakeapi 는 service 창의 cmd/api 가 생기기 전까지 쓰는 임시 백엔드다.
// 계약(플래그·엔드포인트·SIGTERM 드레인)만 흉내 내고, 업무 로직은 없다.
// cmd/api 가 빌드되면 run.sh 가 그쪽을 쓴다.
//
//	go run ./ch07-10-service/edge/cmd/fakeapi -addr 127.0.0.1:8081 -trusted-proxies 127.0.0.1/32
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8081", "listen address")
	trusted := flag.String("trusted-proxies", "", "XFF 를 믿을 직전 홉 CIDR 목록(쉼표). 비우면 XFF 를 무시한다")
	drain := flag.Duration("drain", 3*time.Second, "SIGTERM 뒤 /healthz 가 503 을 내는 시간")
	_ = flag.String("upstream", "", "계약 호환용(쓰지 않음)")
	_ = flag.String("admin-socket", "", "계약 호환용(쓰지 않음)")
	// cmd/api 의 플래그. README·run.sh 가 넘겨도 폴백이 뜨도록 받기만 하고 흉내 내지 않는다.
	ignored := map[string]*string{
		"deny-ips": flag.String("deny-ips", "", "계약 호환용(흉내 내지 않음)"),
		"xff-pick": flag.String("xff-pick", "", "계약 호환용(흉내 내지 않음)"),
	}
	drainNoKA := flag.Bool("drain-close-keepalive", false, "계약 호환용(흉내 내지 않음)")
	flag.Parse()
	for name, v := range ignored {
		if *v != "" {
			log.Printf("fakeapi: -%s=%q 는 흉내 내지 않습니다", name, *v)
		}
	}
	if *drainNoKA {
		log.Printf("fakeapi: -drain-close-keepalive 는 흉내 내지 않습니다")
	}

	proxies, err := parseCIDRs(*trusted)
	if err != nil {
		log.Fatal(err)
	}

	var draining atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if draining.Load() {
			http.Error(w, "draining", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /whoami", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{
			"remote_addr":     r.RemoteAddr,
			"client_ip":       clientIP(r, proxies),
			"x_forwarded_for": strings.Join(r.Header.Values("X-Forwarded-For"), ", "),
		})
	})
	mux.HandleFunc("GET /api/v1/rates/{base}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"base":  strings.ToUpper(r.PathValue("base")),
			"rates": map[string]float64{"KRW": 1380.5, "JPY": 151.2},
			"fake":  true,
		})
	})

	srv := &http.Server{Addr: *addr, Handler: logRequests(mux), ReadHeaderTimeout: 5 * time.Second}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, os.Interrupt)
	go func() {
		<-sig
		log.Printf("%s: SIGTERM — draining %s", *addr, *drain)
		draining.Store(true)
		time.Sleep(*drain)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		log.Printf("%s: Shutdown", *addr)
		srv.Shutdown(ctx)
	}()

	log.Printf("fakeapi listening on %s (trusted-proxies=%q)", *addr, *trusted)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	log.Printf("%s: stopped", *addr)
}

// clientIP 는 직전 홉이 신뢰 목록에 있을 때만 XFF 를 오른쪽부터 읽어,
// 신뢰 목록에 없는 첫 주소를 클라이언트로 본다.
func clientIP(r *http.Request, proxies []netip.Prefix) string {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	peer, err := netip.ParseAddr(host)
	if err != nil || !contains(proxies, peer) {
		return host
	}
	var hops []string
	for _, v := range r.Header.Values("X-Forwarded-For") {
		for _, h := range strings.Split(v, ",") {
			hops = append(hops, strings.TrimSpace(h))
		}
	}
	for i := len(hops) - 1; i >= 0; i-- {
		ip, err := netip.ParseAddr(hops[i])
		if err != nil {
			return host
		}
		if !contains(proxies, ip) {
			return ip.String()
		}
	}
	return host
}

func contains(prefixes []netip.Prefix, ip netip.Addr) bool {
	for _, p := range prefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

func parseCIDRs(s string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, f := range strings.Split(s, ",") {
		if f = strings.TrimSpace(f); f == "" {
			continue
		}
		p, err := netip.ParsePrefix(f)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s from=%s xff=%q", r.Method, r.URL.Path, r.RemoteAddr, r.Header.Get("X-Forwarded-For"))
		next.ServeHTTP(w, r)
	})
}
