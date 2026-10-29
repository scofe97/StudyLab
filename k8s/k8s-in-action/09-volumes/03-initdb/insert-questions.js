// (참고) init 컨테이너 이미지 luksa/quiz-initdb-script-installer:0.1 안에 담긴 초기화 스크립트.
// 이 파일을 직접 쓸 필요는 없다 — 이미지가 이미 담고 있고, init 컨테이너가 이 파일을
// initdb 볼륨(/initdb.d)에 복사한다. 그러면 mongo 가 /docker-entrypoint-initdb.d/ 에서
// 발견해 첫 시작 시 실행하고, 아래 질문으로 DB 를 채운다.
//
// MongoDB 이미지는 DB 가 비어 있을 때 /docker-entrypoint-initdb.d/ 의 .js·.sh 를 실행한다.
// 그 관례에 얹어 초기 데이터를 자동 주입하는 것이 이 단계의 핵심이다.
//
// init 컨테이너 이미지의 Dockerfile 개념(문서 09-01 §4):
//   FROM busybox
//   COPY insert-questions.js /
//   CMD cp /insert-questions.js /initdb.d/ && echo "copied" || echo "error"

db = db.getSiblingDB('kiada');
db.questions.insertOne({
  id: 1,
  text: "What does k8s mean?",
  answers: ["Kates", "Kubernetes", "Kooba Dooba Doo!"],
  correctAnswerIndex: 1
});
