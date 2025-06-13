package integration

import (
	"io/ioutil"
	"log"
	"path/filepath"
	"time"

	"github.com/briandowns/spinner"
)

// dirwalk get fileList - copied from main package to avoid import cycles
func dirwalk(dir string) []string {
	if dir == "" {
		dir = "./"
	}
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal(err.Error())
	}
	var paths []string
	for _, file := range files {
		if file.IsDir() {
			paths = append(paths, dirwalk(filepath.Join(dir, file.Name()))...)
			continue
		}
		paths = append(paths, filepath.Join(dir, file.Name()))
	}
	return paths
}

// spinner var
var sp = spinner.New(spinner.CharSets[34], 100*time.Millisecond)

// sps Starts spinner
func sps(label string) {
	sp.Start()
}

// spe end spinner
func spe() {
	sp.Stop()
}