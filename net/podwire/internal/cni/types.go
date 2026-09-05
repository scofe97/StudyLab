// Package cni 는 CNI spec 이 요구하는 입출력 형식과 실행 껍데기를 담는다.
//
// containernetworking/cni 라이브러리를 쓰지 않고 최소한만 직접 정의한다.
// 규약을 남의 타입으로 덮어 두면 "kubelet 이 정확히 무엇을 주고 무엇을 기대하는가"가
// 안 보이는데, 그걸 보는 것이 Stage 1 의 목적이기 때문이다.
// 규모가 커지면 그때 라이브러리로 갈아탄다.
package cni

// SpecVersion 은 이 플러그인이 말하는 CNI 규격 버전이다.
const SpecVersion = "1.0.0"

// NetConf 는 stdin 으로 들어오는 설정 JSON 이다.
// 공통 필드만 두고, 플러그인 전용 값은 아래 podwire 필드에 모은다.
type NetConf struct {
	CNIVersion string `json:"cniVersion"`
	Name       string `json:"name"`
	Type       string `json:"type"`

	// 이하 podwire 전용
	Bridge  string `json:"bridge"`  // 노드 브리지 이름 (예: podwire0)
	PodCIDR string `json:"podCIDR"` // 이 노드에 할당된 대역
	MTU     int    `json:"mtu"`     // 0 이면 1500. Stage 1b 에서 VXLAN 이 이 값을 깎는다
	DataDir string `json:"dataDir"` // IPAM 저장소 위치
}

// Result 는 ADD 성공 시 stdout 으로 내보내는 JSON 이다.
type Result struct {
	CNIVersion string      `json:"cniVersion"`
	Interfaces []Interface `json:"interfaces,omitempty"`
	IPs        []IPConfig  `json:"ips,omitempty"`
	Routes     []Route     `json:"routes,omitempty"`
	DNS        DNS         `json:"dns,omitempty"`
}

// Interface 는 만들어진 장치 하나를 설명한다.
// Sandbox 가 비어 있으면 호스트 쪽 장치, 채워져 있으면 컨테이너 안 장치다.
type Interface struct {
	Name    string `json:"name"`
	Mac     string `json:"mac,omitempty"`
	Sandbox string `json:"sandbox,omitempty"`
}

// IPConfig 는 부여한 주소 하나다.
// Interface 는 위 Interfaces 배열의 인덱스를 가리킨다.
type IPConfig struct {
	Address   string `json:"address"` // CIDR 표기
	Gateway   string `json:"gateway,omitempty"`
	Interface *int   `json:"interface,omitempty"`
}

// Route 는 컨테이너 안에 넣을 경로다.
type Route struct {
	Dst string `json:"dst"`
	GW  string `json:"gw,omitempty"`
}

// DNS 는 컨테이너의 resolv.conf 에 반영될 값이다.
// 쿠버네티스에서는 kubelet 이 따로 채우므로 보통 비워 둔다.
type DNS struct {
	Nameservers []string `json:"nameservers,omitempty"`
	Search      []string `json:"search,omitempty"`
	Options     []string `json:"options,omitempty"`
}

// Error 는 실패 시 stdout 으로 내보내는 JSON 이다.
// 실패를 stderr 에만 적고 종료 코드로 알리면 kubelet 이 이유를 못 읽는다.
type Error struct {
	CNIVersion string `json:"cniVersion"`
	Code       uint   `json:"code"`
	Msg        string `json:"msg"`
	Details    string `json:"details,omitempty"`
}

// spec 이 정한 오류 코드 일부.
const (
	ErrIncompatibleVersion uint = 1
	ErrUnsupportedField    uint = 2
	ErrUnknownContainer    uint = 3
	ErrInvalidEnv          uint = 4
	ErrIOFailure           uint = 5
	ErrDecodeFailure       uint = 6
	ErrInvalidNetworkConf  uint = 7
	ErrTryAgainLater       uint = 11
	ErrInternal            uint = 999
)

// VersionInfo 는 VERSION 명령의 응답이다.
type VersionInfo struct {
	CNIVersion        string   `json:"cniVersion"`
	SupportedVersions []string `json:"supportedVersions"`
}
