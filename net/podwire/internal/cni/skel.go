package cni

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Args 는 kubelet(또는 cnitool)이 환경변수와 stdin 으로 넘겨준 것을 모은 값이다.
//
// 환경변수 이름을 그대로 두는 이유가 있다. 실습 중 손으로 호출해 볼 때
//
//	CNI_COMMAND=ADD CNI_CONTAINERID=x CNI_NETNS=/var/run/netns/x \
//	CNI_IFNAME=eth0 CNI_PATH=/opt/cni/bin ./podwire < conf.json
//
// 처럼 그대로 치면 되기 때문이다.
type Args struct {
	ContainerID string
	Netns       string
	IfName      string
	Args        string // "K=V;K=V" 형식. 쿠버네티스가 Pod 이름·네임스페이스를 여기 넣는다
	Path        string
	StdinData   []byte
}

// ParseArgs 는 KV 문자열을 맵으로 바꾼다.
// K8S_POD_NAME·K8S_POD_NAMESPACE 를 꺼내 쓸 때 필요하다.
func ParseArgs(s string) map[string]string {
	out := map[string]string{}
	for _, pair := range strings.Split(s, ";") {
		if pair == "" {
			continue
		}
		k, v, ok := strings.Cut(pair, "=")
		if ok {
			out[k] = v
		}
	}
	return out
}

// Funcs 는 플러그인이 채워야 할 세 동작이다.
// Check 는 선택이지만 없으면 kubelet 이 규격 위반으로 볼 수 있으니 최소한 nil 반환은 해 둔다.
type Funcs struct {
	Add   func(*Args, *NetConf) (*Result, error)
	Del   func(*Args, *NetConf) error
	Check func(*Args, *NetConf) error
}

// Main 은 CNI 플러그인의 표준 실행 흐름이다.
// 환경변수를 읽고, stdin 을 파싱하고, 명령에 맞는 함수를 부르고, 결과를 stdout 에 JSON 으로 낸다.
func Main(fns Funcs) {
	cmd := os.Getenv("CNI_COMMAND")

	if cmd == "VERSION" {
		writeJSON(os.Stdout, VersionInfo{
			CNIVersion:        SpecVersion,
			SupportedVersions: []string{"0.3.1", "0.4.0", "1.0.0"},
		})
		return
	}

	args := &Args{
		ContainerID: os.Getenv("CNI_CONTAINERID"),
		Netns:       os.Getenv("CNI_NETNS"),
		IfName:      os.Getenv("CNI_IFNAME"),
		Args:        os.Getenv("CNI_ARGS"),
		Path:        os.Getenv("CNI_PATH"),
	}

	stdin, err := io.ReadAll(os.Stdin)
	if err != nil {
		fail(ErrIOFailure, "stdin 읽기 실패", err.Error())
		return
	}
	args.StdinData = stdin

	var conf NetConf
	if err := json.Unmarshal(stdin, &conf); err != nil {
		fail(ErrDecodeFailure, "설정 JSON 파싱 실패", err.Error())
		return
	}

	switch cmd {
	case "ADD":
		if fns.Add == nil {
			fail(ErrInternal, "ADD 미구현", "")
			return
		}
		res, err := fns.Add(args, &conf)
		if err != nil {
			fail(ErrInternal, "ADD 실패", err.Error())
			return
		}
		res.CNIVersion = SpecVersion
		writeJSON(os.Stdout, res)

	case "DEL":
		// DEL 은 멱등해야 한다. 이미 지워진 것을 지우라는 요청도 성공으로 답한다.
		// 여기서 오류를 내면 Pod 이 Terminating 에 갇힌다.
		if fns.Del == nil {
			return
		}
		if err := fns.Del(args, &conf); err != nil {
			fail(ErrInternal, "DEL 실패", err.Error())
		}

	case "CHECK":
		if fns.Check == nil {
			return
		}
		if err := fns.Check(args, &conf); err != nil {
			fail(ErrInternal, "CHECK 실패", err.Error())
		}

	default:
		fail(ErrInvalidEnv, fmt.Sprintf("알 수 없는 CNI_COMMAND: %q", cmd), "")
	}
}

func writeJSON(w io.Writer, v any) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// fail 은 오류를 stdout 에 JSON 으로 내고 1 로 종료한다.
// stderr 에도 같은 내용을 적어 두면 journalctl -u kubelet 에서 바로 보인다.
func fail(code uint, msg, details string) {
	writeJSON(os.Stdout, Error{CNIVersion: SpecVersion, Code: code, Msg: msg, Details: details})
	fmt.Fprintf(os.Stderr, "podwire: %s (%s)\n", msg, details)
	os.Exit(1)
}
