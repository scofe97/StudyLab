package cni

import "testing"

func TestParseArgs(t *testing.T) {
	// Given — 쿠버네티스가 CNI_ARGS 로 넘기는 형식
	in := "IgnoreUnknown=1;K8S_POD_NAMESPACE=default;K8S_POD_NAME=nginx-abc;K8S_POD_INFRA_CONTAINER_ID=deadbeef"

	// When — 맵으로 파싱한다
	got := ParseArgs(in)

	// Then — 네임스페이스와 Pod 이름을 꺼낼 수 있다
	if got["K8S_POD_NAMESPACE"] != "default" {
		t.Errorf("네임스페이스 = %q, 원하는 값 %q", got["K8S_POD_NAMESPACE"], "default")
	}
	if got["K8S_POD_NAME"] != "nginx-abc" {
		t.Errorf("Pod 이름 = %q, 원하는 값 %q", got["K8S_POD_NAME"], "nginx-abc")
	}
	if len(got) != 4 {
		t.Errorf("항목 수 = %d, 원하는 값 4", len(got))
	}
}

func TestParseArgsEmpty(t *testing.T) {
	// Given — CNI_ARGS 가 비어 있는 경우 (cnitool 로 직접 부를 때 흔하다)
	// When
	got := ParseArgs("")

	// Then — 빈 맵이지 nil 이 아니어야 한다. 호출부가 그냥 조회할 수 있어야 하기 때문이다
	if got == nil {
		t.Fatal("nil 맵이 반환됨")
	}
	if len(got) != 0 {
		t.Errorf("항목 수 = %d, 원하는 값 0", len(got))
	}
}
