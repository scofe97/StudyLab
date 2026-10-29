// Package uds 는 서비스 밖의 짧은 곁가지 실험이다. 코드는 테스트 파일에만 있다.
//
//	go test ./ch07-10-service/experiments/uds -run TestStreamThreeWrites -v -count=1
//	orb -m ubuntu bash -lc 'cd /Users/$USER/study/runners-lab/net/npg/chapters && go test ./ch07-10-service/experiments/uds -run TestUnixpacket -v -count=1'
package uds
