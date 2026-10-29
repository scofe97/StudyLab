package lab

import "testing"

func TestDefaultConfig(t *testing.T) {
	// Given — 기본 설정은 손 실습(02-04)과 같은 토폴로지여야 한다
	// When
	cfg := Default()

	// Then — 네임스페이스 둘, 브리지 하나, 각 끝의 이름이 겹치지 않는다
	if len(cfg.Namespaces) != 2 {
		t.Fatalf("네임스페이스 수 = %d, 원하는 값 2", len(cfg.Namespaces))
	}
	seen := map[string]bool{}
	for _, ns := range cfg.Namespaces {
		for _, name := range []string{ns.IfName, ns.PeerName} {
			if seen[name] {
				t.Errorf("장치 이름 중복: %s", name)
			}
			seen[name] = true
		}
	}
	if cfg.Bridge == "" || cfg.BridgeCIDR == "" {
		t.Error("브리지 이름과 주소는 비어 있으면 안 된다")
	}
}
