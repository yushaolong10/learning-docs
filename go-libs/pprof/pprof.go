package attacker

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
)

//cpu
func StartCpuProf() {
	f, err := os.Create("cpu.prof")
	if err != nil {
		fmt.Println("create cpu profile file error: ", err.Error())
		return
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		fmt.Println("can not start cpu profile,  error: ", err.Error())
		f.Close()
	}
}

func StopCpuProf() {
	pprof.StopCPUProfile()
}

//mem
func SaveMemProf() {
	f, err := os.Create("mem.prof")
	if err != nil {
		fmt.Println("create mem profile file error: ", err.Error())
		return
	}
	runtime.GC() // get up-to-date statistics
	if err := pprof.WriteHeapProfile(f); err != nil {
		fmt.Println("could not write memory profile: ", err.Error())
	}
	f.Close()
}

// goroutine
func SaveGoroutineProfile() {
	f, err := os.Create("goroutine.prof")
	if err != nil {
		fmt.Println("create mem profile file error: ", err.Error())
		return
	}

	if err := pprof.Lookup("goroutine").WriteTo(f, 1); err != nil {
		fmt.Println("could not write block profile: ", err.Error())
	}
	f.Close()
}
