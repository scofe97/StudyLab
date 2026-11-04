# ssh-bbs — SSH 로 들어오는 레트로 BBS

`ssh 닉@호스트` 한 줄로 어디서든 들어오는 BBS 를 Go 로 만드는 실습입니다. 클라이언트를 따로 설치하지 않아도 되고, 들어오면 채팅방·게시판·자료실·도어 게임이 있는 예전 PC 통신 화면이 나옵니다. 기능 하나를 만들 때마다 그 밑의 네트워크 주제(SSH 프로토콜, 터미널 제어, 포트 포워딩, 노드 간 복제, 남용 방지)를 같이 배웁니다.

기능 아이디어는 [c4punks/ssh-chatter](https://github.com/c4punks/ssh-chatter)와 그 Go 원본 [gosuda/ssh-chat](https://github.com/gosuda/ssh-chat)에서 얻었습니다. ssh-chatter 는 GPL-2.0 이라 코드는 가져오지 않고 링크로만 참조합니다.

## 관련 문서

| 문서 | 경로 |
|---|---|
| 단계 순서와 범위 | [docs/roadmap.md](docs/roadmap.md) |
| 학습 문서와 진행 상태 | `~/runners-high/write/02_os/project/ssh-bbs/` |
| 단계별 이슈 | 라벨 `ssh-bbs` (#33~#44) |



## 실행

아직 코드가 없습니다. 빌드·실행 명령은 S1(#33)에서 채웁니다. S1 이 끝나면 아래 모양이 됩니다.

```bash
go run ./net/ssh-bbs/cmd/ssh-bbs        # 맥북에서 2222 포트로 띄움 (S1 에서 확정)
ssh -p 2222 닉@localhost                # -p: 포트
```



## 공개 저장소에서 지킬 것

호스트 키, 공개 서버 주소, 토큰은 이 저장소에 넣지 않습니다. 호스트 키는 실행할 때 저장소 밖 경로에 만들고, 공개 서버(S12) 정보는 저장소 밖에 둡니다.
