package main

import "github.com/deltron-fr/govisor/internal/process"

func main() {
	supervisor := process.NewSupervisor()
	supervisor.ServeAPI()
}
