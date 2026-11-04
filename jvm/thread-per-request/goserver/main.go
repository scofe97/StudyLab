// goserver 는 Spring 변형과 같은 세 엔드포인트(+/stats)를 net/http 로 제공한다.
// net/http 는 연결마다 goroutine 을 띄우므로 "요청당 스레드"의 Go 쪽 대응이다.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strconv"
	"time"
)

var chunk = func() []byte {
	b := make([]byte, 8192)
	for i := range b {
		b[i] = 'x'
	}
	return b
}()

func intParam(r *http.Request, name string, def int) int {
	if v, err := strconv.Atoi(r.URL.Query().Get(name)); err == nil && v >= 0 {
		return v
	}
	return def
}

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /small", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	})
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
		ms := intParam(r, "ms", 200)
		time.Sleep(time.Duration(ms) * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"slept":%d}`, ms)
	})
	mux.HandleFunc("GET /big", func(w http.ResponseWriter, r *http.Request) {
		total := intParam(r, "kb", 1024) * 1024
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.Itoa(total))
		for left := total; left > 0; {
			n := min(left, len(chunk))
			if _, err := w.Write(chunk[:n]); err != nil {
				return
			}
			left -= n
		}
	})
	// Spring 의 /stats 와 같은 키. Go 는 OS 스레드가 아니라 goroutine 수를 보고한다.
	mux.HandleFunc("GET /stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"threads":%d}`, runtime.NumGoroutine())
	})
	return mux
}

func main() {
	port := flag.Int("port", 8080, "listen port")
	flag.Parse()
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("goserver listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, newMux()))
}
