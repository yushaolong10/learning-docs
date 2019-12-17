package gray

import (
	"fmt"
	"testing"
	"time"
)

const (
	GrayScenePropertyPost ScenarioEnumType = iota
	GraySceneServiceInvoke
)

func TestCheckRateGrayInIfBranch(t *testing.T) {
	RegisterGrayScenario(GrayScenePropertyPost, "/HyperX/ServiceDiscovery/Gray/migrateRuleengine", "")

	ticker := time.NewTicker(time.Second)
	for range ticker.C {
		if CheckRateGrayInIfBranch(GrayScenePropertyPost) {
			fmt.Println("in gray state")
		} else {
			fmt.Println("in normal state")
		}
	}
}

func BenchmarkCheckRateGrayInIfBranch(b *testing.B) {
	RegisterGrayScenario(GraySceneServiceInvoke, "/HyperX/ServiceDiscovery/Gray/migrateRuleengine", "bjcc")
	for i := 0; i < b.N; i++ {
		CheckRateGrayInIfBranch(GrayScenePropertyPost)
	}
}
