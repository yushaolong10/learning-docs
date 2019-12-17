package gray

import (
	"fmt"
	"testing"
)

func TestMakeBallQueue(b *testing.T) {
	for acquire := 0; acquire <= 100; acquire++ {
		queue := makeBallQueue(100, acquire)
		fmt.Println("acquire", acquire, "queue", string(queue))
	}
}

//goos: linux
//goarch: amd64
//BenchmarkMakeBallQueue-4         1000000              1519 ns/op
//PASS
//ok      command-line-arguments  1.586s
func BenchmarkMakeBallQueue(b *testing.B) {
	for i := 0; i < b.N; i++ { //use b.N for looping
		makeBallQueue(100, 80)
	}
}
