# 06 테스트 D — 재시작 후 유지, 원본 변경 비반영, 저장 파일·SQLite 관계

실행일: 2026-10-15. 승인 전 상태라 "버전 유지"는 Draft·문서 버전 기준으로만 확인했다(Page·Website 의 v1 이 없다).

## 1. 서비스 재시작 전후 (원문)

> 아래 첫 블록의 `python3 … mcp.py` 줄은 경로 오타로 실패했다(그대로 둠). 문서 해시 비교는 §2 의 마지막 두 블록에 다시 했다.

```text
$ flats list
SLUG             TYPE  VISIBILITY  LIVE     VERSIONS  URL
learning-hub     flat  private     -        0         
k8s-network-doc  docs  private     running  -         http://k8s-network-doc.localhost:7879
k8s-network      page  private     -        0         
[exit=0]
```

```text
$ flats draft k8s-network; flats draft learning-hub
k8s-network Private Draft revision 2 (base v0), 2 files, 11.2 kB; hash 6167b7d7ea643288b100471ed7118aa6c6de1bf174542f291ed327e4207dd485; type page
learning-hub Private Draft revision 1 (base v0), 4 files, 3.5 kB; hash f8fc83ac01e931d9cbd6d333d12caa45f6e49fc69b27670e3530e4ab0563aa40; type flat
[exit=0]
```

```text
$ curl -s -o /dev/null -w 'preview rev2 %{http_code}\n' http://k8s-network-guubxv0d.localhost:7879/; curl -s -o /dev/null -w 'doc %{http_code}\n' http://k8s-network-doc.localhost:7879/_docs/healthz
preview rev2 200
doc 200
[exit=0]
```

```text
$ python3 -I /Users/user/study/flats-lab/../../../private/tmp/claude-501/-Users-user-study-flats-lab/982efcab-7f67-4652-8ade-b4b9525def1a/scratchpad/mcp.py get_document '{"slug":"k8s-network-doc"}' | grep -o '"hash\\\\":\\\\"[0-9a-f]*' | head -1
/opt/homebrew/Cellar/python@3.14/3.14.4_1/Frameworks/Python.framework/Versions/3.14/Resources/Python.app/Contents/MacOS/Python: can't open file '/Users/user/study/flats-lab/../../../private/tmp/claude-501/-Users-user-study-flats-lab/982efcab-7f67-4652-8ade-b4b9525def1a/scratchpad/mcp.py': [Errno 2] No such file or directory
[exit=0]
```

```text
$ launchctl kickstart -k gui/$(id -u)/dev.flats.serve
[exit=0]
```

```text
$ flats status | grep -E 'server:|launchd:'
server:  running at http://127.0.0.1:7878
launchd: loaded, running, pid 17896, last exit 0
[exit=0]
```

```text
$ flats list
SLUG             TYPE  VISIBILITY  LIVE     VERSIONS  URL
learning-hub     flat  private     -        0         
k8s-network-doc  docs  private     running  -         http://k8s-network-doc.localhost:7879
k8s-network      page  private     -        0         
[exit=0]
```

```text
$ flats draft k8s-network; flats draft learning-hub; flats approvals
k8s-network Private Draft revision 2 (base v0), 2 files, 11.2 kB; hash 6167b7d7ea643288b100471ed7118aa6c6de1bf174542f291ed327e4207dd485; type page
learning-hub Private Draft revision 1 (base v0), 4 files, 3.5 kB; hash f8fc83ac01e931d9cbd6d333d12caa45f6e49fc69b27670e3530e4ab0563aa40; type flat
ID                FLAT          ACTION           STATUS   VIA  REQUESTED         REASON
apr-zn27nzgynbpk  learning-hub  publish private  pending  cli  2026-10-15 10:57  
apr-96p1503rvmiz  k8s-network   publish private  pending  cli  2026-10-15 10:56  
apr-47ecg86fq2fh  k8s-network   publish private  pending  cli  2026-10-15 10:55  
[exit=0]
```

```text
$ curl -s -o /dev/null -w 'preview rev2 %{http_code}\n' http://k8s-network-guubxv0d.localhost:7879/; curl -s -o /dev/null -w 'preview learning-hub %{http_code}\n' http://learning-hub-pwtok4l7.localhost:7879/; curl -s -o /dev/null -w 'doc healthz %{http_code}\n' http://k8s-network-doc.localhost:7879/_docs/healthz
preview rev2 200
preview learning-hub 200
doc healthz 200
[exit=0]
```

재시작 전후 비교: 세 flat 모두 `flats list` 가 동일하고 Draft 해시(`6167b7d7…`, `f8fc83ac…`), 미리보기 URL(200), 문서 헬스체크(200), 대기 승인 3건이 그대로다. pid 만 바뀌었다(`17896`).

## 2. `examples/` 원본을 고쳤을 때 배포본이 따라가는가

```text
(참고: 원본 index.html 을 cp 로 백업한 뒤 끝에 주석 한 줄을 붙였고, 실험 후 백업으로 복원했다)
```

```text
$ shasum -a 256 examples/page-k8s-network/index.html; curl -s http://k8s-network-guubxv0d.localhost:7879/ | shasum -a 256
b014ebabd738bae1ca86e328788a25c29fb7505248140386d027fbba693a2f30  examples/page-k8s-network/index.html
b014ebabd738bae1ca86e328788a25c29fb7505248140386d027fbba693a2f30  -
[exit=0]
```

```text
$ shasum -a 256 examples/page-k8s-network/index.html   # 원본을 수정한 뒤
ff711f69c1b7ef232e94be51426437db9343e4d0048024caa5ba0c860cdabf78  examples/page-k8s-network/index.html
[exit=0]
```

```text
$ curl -s http://k8s-network-guubxv0d.localhost:7879/ | shasum -a 256   # 미리보기(배포본)
b014ebabd738bae1ca86e328788a25c29fb7505248140386d027fbba693a2f30  -
[exit=0]
```

```text
$ curl -s http://k8s-network-guubxv0d.localhost:7879/ | grep -c 'test D'
0
[exit=1]
```

```text
$ flats draft k8s-network
k8s-network Private Draft revision 2 (base v0), 2 files, 11.2 kB; hash 6167b7d7ea643288b100471ed7118aa6c6de1bf174542f291ed327e4207dd485; type page
[exit=0]
```

```text
$ shasum -a 256 examples/page-k8s-network/index.html   # 원본 복원 후
b014ebabd738bae1ca86e328788a25c29fb7505248140386d027fbba693a2f30  examples/page-k8s-network/index.html
[exit=0]
```

```text
$ python3 -I /private/tmp/claude-501/-Users-user-study-flats-lab/982efcab-7f67-4652-8ade-b4b9525def1a/scratchpad/mcp.py get_document '{"slug":"k8s-network-doc"}' | grep -o 'hash.....[0-9a-f]\{64\}' | head -1
hash\":\"b97c953de0eae7c7269ddeecd889794ff0860a64195d4e7face858ad5d6bbfad
[exit=0]
```

```text
$ shasum -a 256 examples/doc-k8s-network/index.md
b97c953de0eae7c7269ddeecd889794ff0860a64195d4e7face858ad5d6bbfad  examples/doc-k8s-network/index.md
[exit=0]
```

```text
$ sqlite3 -readonly '/Users/user/Library/Application Support/Flats/flats.db' ".schema flats" ".schema draft_revisions" ".schema revisions" ".schema versions" ".schema approvals" ".schema revision_files"

### 판정

| 대상 | 원본 수정 후 재배포 안 함 | 재배포(`flats deploy`) 후 |
|---|---|---|
| Page Draft/미리보기 | 변하지 않음. 미리보기 응답 SHA-256 이 수정 전 원본 해시(`b014eba…`)와 같고 `test D` 문자열 0건. `flats draft` 해시도 그대로 | (이 항목에서는 Draft 를 만들지 않으려 재배포하지 않음. Draft 재저장은 03 로그의 revision 2 로 확인) |
| Document | 원본 해시 `90aa113a…` 로 바뀌었어도 라이브 문서 해시는 `b97c953d…` 그대로(아래 블록) | 04 로그: 재배포 약 3초 뒤 라이브 문서와 해시가 일치 |

결론: Flats 는 원본 디렉터리를 감시하지 않는다. 배포 시점에 파일을 묶어 복사해 저장하므로 원본은 입력일 뿐 서빙 대상이 아니다.

### 문서 원본 수정 실험 (원문)

```text
$ shasum -a 256 examples/doc-k8s-network/index.md   # 원본 수정 후
90aa113a019826981a45152b1ec262e5676d783a8880dd0ac91650540a710501  examples/doc-k8s-network/index.md
[exit=0]
```

```text
$ python3 -I /private/tmp/claude-501/-Users-user-study-flats-lab/982efcab-7f67-4652-8ade-b4b9525def1a/scratchpad/mcp.py get_document '{"slug":"k8s-network-doc"}' | grep -o 'hash.....[0-9a-f]\{64\}' | head -1   # 라이브 문서
hash\":\"b97c953de0eae7c7269ddeecd889794ff0860a64195d4e7face858ad5d6bbfad
[exit=0]
```

```text
$ shasum -a 256 examples/doc-k8s-network/index.md   # 복원 후
b97c953de0eae7c7269ddeecd889794ff0860a64195d4e7face858ad5d6bbfad  examples/doc-k8s-network/index.md
[exit=0]
```

## 3. 원본 · Flats 내부 저장 파일 · SQLite 행의 관계


```text
$ sqlite3 -readonly -header -column '/Users/user/Library/Application Support/Flats/flats.db' "select slug,name,visibility,live_version from flats"
slug             name                             visibility  live_version
---------------  -------------------------------  ----------  ------------
k8s-network      Kubernetes 네트워크 흐름         private     0           
k8s-network-doc  Kubernetes 네트워크 흐름 (문서)  private     2           
learning-hub     Learning Hub                     private     0           
try-prom-talk    Prometheus 2주차 발표            private     0           
[exit=0]
```

```text
$ sqlite3 -readonly -header -column '/Users/user/Library/Application Support/Flats/flats.db' "select flat,revision,substr(hash,1,12) h,size,files,kind,message from draft_revisions order by flat,revision"
flat             revision  h             size    files  kind    message                              
---------------  --------  ------------  ------  -----  ------  -------------------------------------
k8s-network      1         b7b4e442240c  11558   3      static  test A: draft only                   
k8s-network      2         6167b7d7ea64  11216   2      static  test A: draft rev2 without stray .omc
k8s-network-doc  1         baa5fc91e717  2098    2      server                                       
k8s-network-doc  2         c102675bbb24  2195    2      server                                       
learning-hub     1         f8fc83ac01e9  3498    4      static  test C: v1 draft                     
try-prom-talk    1         b3a2796b7fe1  119689  4      static  try: 발표자료 v1                     
[exit=0]
```

```text
$ sqlite3 -readonly -header -column '/Users/user/Library/Application Support/Flats/flats.db' "select flat,number,kind,files,size,message from revisions order by flat,number"
[exit=0]
```

```text
$ sqlite3 -readonly -header -column '/Users/user/Library/Application Support/Flats/flats.db' "select flat,number,path,substr(hash,1,12) h,size from revision_files order by flat,number,path"
[exit=0]
```

```text
$ sqlite3 -readonly -header -column '/Users/user/Library/Application Support/Flats/flats.db' "select flat,number,published,pruned from versions"
flat             number  published  pruned
---------------  ------  ---------  ------
k8s-network-doc  1       1          0     
k8s-network-doc  2       1          0     
[exit=0]
```

```text
$ sqlite3 -readonly -header -column '/Users/user/Library/Application Support/Flats/flats.db' "select id,flat,action,status,via,datetime(requested_at,'unixepoch') req from approvals"
id                flat           action   status   via  req
----------------  -------------  -------  -------  ---  ---
apr-47ecg86fq2fh  k8s-network    publish  pending  cli     
apr-96p1503rvmiz  k8s-network    publish  pending  cli     
apr-zn27nzgynbpk  learning-hub   publish  pending  cli     
apr-3qgsmkm5upn7  try-prom-talk  publish  pending  cli     
[exit=0]
```

```text
$ sqlite3 -readonly -header -column '/Users/user/Library/Application Support/Flats/flats.db' "select name from sqlite_master where type='table' order by 1"
name                  
----------------------
approvals             
content_groups        
content_head          
content_state         
deployments           
draft_revisions       
drafts                
env_vars              
events                
flat_providers        
flats                 
host_binding          
legacy_private_upgrade
network_settings      
pagepaths             
pageviews             
portal_listing        
previews              
redirects             
revision_files        
revisions             
runtime_generation    
secrets               
settings              
sqlite_sequence       
versions              
[exit=0]
```

```text
$ find '/Users/user/Library/Application Support/Flats/flats' -type f -not -path '*/runtime/*' | sed "s#/Users/user/Library/Application Support/Flats/##" | sort
flats/k8s-network-doc/data/db.sqlite
flats/k8s-network-doc/data/db.sqlite-shm
flats/k8s-network-doc/data/db.sqlite-wal
flats/k8s-network-doc/draft-revs/2/flats.json
flats/k8s-network-doc/draft-revs/2/index.md
flats/k8s-network-doc/versions/1/flats.json
flats/k8s-network-doc/versions/1/index.md
flats/k8s-network-doc/versions/2/flats.json
flats/k8s-network-doc/versions/2/index.md
flats/k8s-network/draft-revs/1/.omc/state/sessions/982efcab-7f67-4652-8ade-b4b9525def1a/pre-tool-advisory-throttle.json
flats/k8s-network/draft-revs/1/flats.json
flats/k8s-network/draft-revs/1/index.html
flats/k8s-network/draft-revs/2/flats.json
flats/k8s-network/draft-revs/2/index.html
flats/learning-hub/draft-revs/1/flats.json
flats/learning-hub/draft-revs/1/index.html
flats/learning-hub/draft-revs/1/networking.html
flats/learning-hub/draft-revs/1/storage.html
flats/try-prom-talk/draft-revs/1/02-01.스크레이프와 저장 흐름.html
flats/try-prom-talk/draft-revs/1/02-02.알림 흐름.html
flats/try-prom-talk/draft-revs/1/flats.json
flats/try-prom-talk/draft-revs/1/index.html
[exit=0]
```

```text
$ du -sh '/Users/user/Library/Application Support/Flats/flats/'*
 36K	/Users/user/Library/Application Support/Flats/flats/k8s-network
124K	/Users/user/Library/Application Support/Flats/flats/k8s-network-doc
 16K	/Users/user/Library/Application Support/Flats/flats/learning-hub
128K	/Users/user/Library/Application Support/Flats/flats/try-prom-talk
[exit=0]
```

### 관계 정리

```text
examples/page-k8s-network/        (원본, 수정해도 영향 없음)
   │  flats deploy 가 읽어서 묶음
   ▼
Flats 데이터/flats/k8s-network/draft-revs/<n>/{index.html,flats.json}   ← 서빙되는 실제 파일
   ▲
flats.db  draft_revisions(flat, revision, hash, size, files, message)   ← 위 파일의 메타데이터·해시
          versions(flat, number, published, pruned)                      ← 승인 후 생기는 번호 있는 버전(콘텐츠는 versions/<n>/)
          flats(slug, visibility, live_version)                          ← 현재 활성 버전 포인터
          approvals(id, flat, action, status, params)                    ← 대기·결정된 승인
```

- 콘텐츠 파일은 DB 가 아니라 디스크의 `flats/<slug>/{draft-revs,versions}/<n>/` 에 평문 파일로 있다. DB 는 해시·크기·메시지·승인·활성 포인터를 가진다.
- Page·Website 는 Draft 가 `draft-revs/<rev>/`, 승인된 버전이 `versions/<n>/` 로 갈라진다(문서는 두 곳 모두 사용: `draft-revs/2`, `versions/1`, `versions/2`).
- Document 는 내부적으로 `versions` 행(`published=1`)을 만들고 `flats.live_version` 이 2 로 올라가지만 CLI 는 `documents have no versions` 로 거부한다. 문서의 실행 중 상태는 `data/db.sqlite`(편집기 상태)에 있다.
- `revisions`·`revision_files` 는 Design 타입의 리비전 이력용으로 보이며 비어 있다(미확인).
- 지금 DB 에서 flat 4개가 보인다. 그중 `try-prom-talk`("Prometheus 2주차 발표", Draft 1개, 파일 4개, publish 승인 대기 `apr-3qgsmkm5upn7`)는 **이 세션이 만들지 않았다.** 같은 호스트를 쓰는 다른 에이전트·세션이 만든 것으로 보인다.

## 결론

- 서비스 재시작은 콘텐츠·Draft·승인 대기·문서 모두 유지한다. 영속 저장은 SQLite(`flats.db`)+디스크 파일이다.
- 원본 수정은 배포본에 자동 반영되지 않는다. 재배포 때만 복사된다.
- 예제 폴더에 섞인 `.omc` 파일은 Draft revision 1 디렉터리(`draft-revs/1/.omc/state/…`)에 남아 있다. Draft 를 다시 저장해도 이전 revision 은 보존된다.

## Needs Verification

- 승인 후 `versions/<n>/` 와 Page·Website 활성 버전의 재시작 후 유지(승인 불가로 미실행).
- `keep_versions=10` 가지치기 동작.
- `try-prom-talk` 를 만든 주체(이 세션 밖).
- `revisions`/`revision_files` 용도.
