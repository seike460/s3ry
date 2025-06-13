package s3ry

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/briandowns/spinner"
	"github.com/seike460/s3ry/internal/i18n"
)

// PromptItems struct for promptui
type PromptItems struct {
	Key          int
	Val          string
	Size         int64
	LastModified time.Time
	Tag          string
}

// spinner var
var sp = spinner.New(spinner.CharSets[34], 100*time.Millisecond)

// sps Starts spinner
func sps(label string) {
	fmt.Println(label)
	sp.Start()
}

// spe end spinner
func spe() {
	sp.Stop()
}

// checkLocalExists check localFile
func checkLocalExists(objectKey string) {
	filename := filepath.Base(objectKey)
	_, err := os.Stat(filename)
	if err == nil {
		var overlide string
		fmt.Println(i18n.Sprintf("The file exists. Overwrite? File name:% s, [Yy] / [Nn]", filename))
		fmt.Scan(&overlide)
		if overlide != "y" && overlide != "Y" {
			log.Fatal("End processing")
		}
	}
}

// awsErrorPrint print Error for AWS
func awsErrorPrint(err error) {
	if aerr, ok := err.(awserr.Error); ok {
		log.Fatal(aerr.Error())
	}
	log.Fatal(err.Error())
}

// dirwalk get fileList
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
		// Skip hidden files and directories to prevent infinite loops
		if file.Name()[0] == '.' {
			continue
		}
		
		if file.IsDir() {
			// Add safety check to prevent infinite recursion
			subDir := filepath.Join(dir, file.Name())
			// Skip common directories that cause infinite loops
			if file.Name() == "bin" || file.Name() == "dist" || file.Name() == "vendor" {
				continue
			}
			paths = append(paths, dirwalk(subDir)...)
			continue
		}
		paths = append(paths, filepath.Join(dir, file.Name()))
	}
	return paths
}
