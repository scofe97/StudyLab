// Command netpath — 대상을 재고 요청 경로가 어디서 실패했는지 보고한다.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/scofe97/runners-lab/net/netpath/probe"
	"github.com/scofe97/runners-lab/net/netpath/types"
)

// exitStageFailed 는 "쟀고 실패를 봤다", exitUnprobeable 은 "재지 못했다" — 후자는 호출 쪽 문제라 구분한다.
const (
	exitOK          = 0
	exitStageFailed = 1
	exitUnprobeable = 2
)

func main() { os.Exit(run()) }

func run() int {
	asJSON := flag.Bool("json", false, "waterfall 대신 Result JSON 을 한 줄에 하나씩 출력한다")
	count := flag.Int("count", 1, "프로브 반복 횟수")
	timeout := flag.Duration("timeout", 10*time.Second, "프로브 하나의 전체 예산")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: netpath [flags] <target>\n\n")
		fmt.Fprintf(os.Stderr, "  요청 하나를 DNS·TCP·HTTP 로 갈라 재고 어디서 왜 실패했는지 보고한다.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		return exitUnprobeable
	}
	if *count < 1 {
		fmt.Fprintln(os.Stderr, "--count 는 1 이상이어야 한다")
		return exitUnprobeable
	}

	target := flag.Arg(0)
	cfg := probe.Config{Timeout: *timeout}

	// JSON Lines — 배열로 묶으면 전부 끝나야 나오고 jq 로 한 줄씩 처리하기도 어렵다.
	enc := json.NewEncoder(os.Stdout)

	succeeded := 0
	failures := map[types.ErrorKind]int{}

	for i := 0; i < *count; i++ {
		r, err := probe.Do(context.Background(), target, cfg)
		if err != nil {
			// 재기 시작조차 못 했다 — 반복해도 같은 결과이므로 즉시 멈춘다.
			fmt.Fprintln(os.Stderr, err)
			return exitUnprobeable
		}

		if *asJSON {
			if err := enc.Encode(r); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return exitUnprobeable
			}
		} else {
			if i > 0 {
				fmt.Println()
			}
			fmt.Print(r.Waterfall())
		}

		if r.FailureStage == "" {
			succeeded++
		} else {
			failures[r.ErrorKind]++
		}
	}

	if *count > 1 && !*asJSON {
		fmt.Print(summarize(*count, succeeded, failures))
	}

	// ping 과 같은 규칙 — 한 번이라도 성공하면 0. 간헐적 실패는 요약 줄로 보완하고,
	// 스크립트 쪽이 필요해지면 --fail-on-any 를 더한다.
	if succeeded == 0 {
		return exitStageFailed
	}
	return exitOK
}

func summarize(count, succeeded int, failures map[types.ErrorKind]int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n%d회 중 %d회 성공", count, succeeded)

	if len(failures) > 0 {
		// map 순회 순서는 무작위 — 정렬 안 하면 같은 입력에 다른 줄이 나온다.
		kinds := make([]string, 0, len(failures))
		for k, n := range failures {
			kinds = append(kinds, fmt.Sprintf("%s %d회", k, n))
		}
		sort.Strings(kinds)
		fmt.Fprintf(&b, " — 실패 %d회 (%s)", count-succeeded, strings.Join(kinds, ", "))
	}
	b.WriteString("\n")
	return b.String()
}
