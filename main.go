package main

import (
	"reaper-build/executor"
	"reaper-build/tool/log"
)

func main() {
	if err := executor.Execute(); err != nil {
		log.Fatalf("Failed to run reaper, the error is: %+v", err)
	}
}
