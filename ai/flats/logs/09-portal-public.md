# 09 Portal 공개 실측 (2026-10-16, v0.9.1)

사용자가 콘솔에서 Portal 제공자를 켜고 `k8s-network`(Page, v1)를 Public 으로 전환·승인했다. 이 로그는 그 뒤 메인 세션이 읽기 명령만으로 확인한 결과다.

## 상태

```text
$ flats list
SLUG             TYPE  VISIBILITY  LIVE     VERSIONS  URL
k8s-network      page  public      v1       1         https://k8s-network.portal.rabbitson87.dev (*)
try-pod-network  page  private     v1       1         http://try-pod-network.localhost:7879
...
(*) public URL:
  k8s-network: This flat is public: anyone on the internet can open it. A domain or URL is not what makes it public.
```

## 공개 주소에서 콘텐츠와 관리 경로

```text
$ U=https://k8s-network.portal.rabbitson87.dev
$ curl -s -o … -w '%{http_code} %{content_type} %{size_download}B' "$U$p"
/                200 text/html; charset=utf-8 11157B
/mcp             404 text/plain; charset=utf-8 19B
/api/flats       404 text/plain; charset=utf-8 19B
/api/approvals   404 text/plain; charset=utf-8 19B
/console         404 text/plain; charset=utf-8 19B
/api/system      404 text/plain; charset=utf-8 19B
$ curl -s "$U/" | grep -o '<title>[^<]*</title>'
<title>Kubernetes 네트워크 흐름: Pod에서 다른 노드의 Pod까지</title>
$ curl -s -X POST -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' "$U/mcp"
method not allowed
```

## 대조: 같은 경로를 로컬 콘솔(7878)에서

```text
/mcp         405   (GET)
/api/flats   200
/api/system  404
$ curl -s -X POST … -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' http://127.0.0.1:7878/mcp
→ result.tools 배열이 돌아옴
```

## 바인딩

```text
$ lsof -nP -iTCP -sTCP:LISTEN | grep -i flats
flats      3647 user   10u  IPv4 …  TCP 127.0.0.1:7879 (LISTEN)
flats      3647 user   21u  IPv4 …  TCP 127.0.0.1:7878 (LISTEN)
```

결론:
- Portal 공개 주소는 승인된 Page 콘텐츠만 200 으로 준다. 로컬 콘솔에 실재하는 `/api/flats`(200)·`/mcp`(도구 목록) 경로가 공개 주소에서는 404·"method not allowed" 로 막힌다.
- Portal 을 켠 뒤에도 7878·7879 는 `127.0.0.1` 에만 LISTEN 한다. Portal 은 들어오는 포트를 열지 않고 릴레이로 나가는 연결을 쓴다.

Needs Verification:
- Portal README 의 "릴레이에서 TLS 를 풀지 않는다(E2E tenant TLS)" 주장은 확인하지 않았다.
- relay listing 을 hidden 으로 했는지, 릴레이 목록에서 실제로 빠졌는지는 확인하지 않았다.
- 다른 네트워크(휴대폰 LTE 등)에서 열리는지는 확인하지 않았다. 이 확인은 같은 Mac 에서 나간 요청이다.
