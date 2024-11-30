package main

import (
	"time"

	"github.com/AlCutter/octonaut/cmd/octonaut/cmd"
	"github.com/charmbracelet/log"
)

func main() {
	log.SetTimeFormat(time.Kitchen)
	cmd.Execute()
}
