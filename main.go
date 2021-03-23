package main

import (
	"os"

	"github.com/elastic/beats/libbeat/beat"

	"github.com/narmi/cloudwatchmetricbeat/beater"
)

func main() {
	err := beat.Run("cloudwatchmetricbeat", "", beater.New)
	if err != nil {
		os.Exit(1)
	}
}
