package core

import (
	"os"
)

var panels string = "[]"

func loadPanels() {
	bin, err := os.ReadFile("panels.json")
	if err == nil {
		panels = string(bin)
	}
}
