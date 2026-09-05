---
title: review — 진행 중 점검 기록
tags: [review, podwire]
status: final
updated: 2026-09-04
---

# review — 진행 중 점검 기록

Stage 를 채워 나가는 동안 쓰는 글이 여기 쌓인다. `docs/` 와 목적이 다르다.

| | `docs/` | `review/` |
|---|---|---|
| 언제 | Stage 를 끝낸 뒤 | 진행하는 도중, 아무 때나 |
| 누구에게 | 나중에 읽을 사람 | 지금의 나 |
| 무엇을 | 무엇을 했고 무엇이 예측과 어긋났나 | 왜 이렇게 정했나, 어디서 막혔나, 무엇을 되돌렸나 |
| 형태 | 완결된 한 편 | 날짜별 조각 |

Stage 노트를 쓸 때 이 폴더를 되짚으면 "그때 왜 그렇게 했더라"가 남아 있다.
진행 중에 남기지 않으면 그 이유는 코드에도 커밋 메시지에도 안 남는다.

## 파일 배치

```
review/
├─ _template/review-template.md
├─ 2026-09-04/
│  └─ stage0_netlink-cli.review.md
└─ 2026-09-11/
   ├─ stage1_cni-add.review.md
   └─ stage1_ipam-lock.review.md
```

- 폴더는 `YYYY-MM-DD`. `write/_review/` 와 같은 방식이라 눈이 익다
- 파일은 `stage<N>_<주제>.review.md`. 한 날 여러 편이 나올 수 있다
- 폴더는 미리 파 두지 않는다. 쓸 것이 생긴 날 만든다

## 커밋한다

`.sbh/` 는 gitignore 로 두는 로컬 작업 폴더지만 여기는 추적한다.
이 프로젝트에서는 판단의 이유 자체가 산출물이라, 지워지면 아까운 쪽이다.
