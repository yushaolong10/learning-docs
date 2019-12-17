package gray

//the package is for gray scene.
//
// it depends <qconf>
//
//usage:
// (1).Register your scene and set qconf data path
//
// 		func RegisterGrayScenario(scene grayScenarioEnum, qconfPath, idc string) error
//
// (2).use in your code, example like:
//
//		if gray.CheckRateGrayInIfBranch(gray.GraySceneServiceInvoke) {
//					...
//					gray code
//					...
//		} else {
//					...
//					normal code
//					...
//		}
