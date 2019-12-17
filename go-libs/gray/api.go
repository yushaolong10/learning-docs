package gray

import (
	"errors"
	"sync"
)

var (
	ErrScenarioExisted  = errors.New("gray scenario already exist")
	ErrInvalidRateRange = errors.New("invalid rate range [0,100]")
)

/**
qconf json format:
{
type:1,
rate:10   // 0-100
}
*/

//accord gray rate exec in  "if condition"
func CheckRateGrayInIfBranch(scene ScenarioEnumType) bool {
	if gs, ok := global[scene]; ok {
		return gs.CheckIfGray()
	}
	return false
}

func RegisterGrayScenario(scene ScenarioEnumType, qconfPath, idc string) error {
	if _, ok := global[scene]; !ok {
		state := &grayState{
			mutex:     new(sync.RWMutex),
			scene:     scene,
			qconfPath: qconfPath,
			idc:       idc,
		}
		if err := state.Update(); err != nil {
			return err
		}
		go state.Monitor()
		global[scene] = state
		return nil
	}
	return ErrScenarioExisted
}
