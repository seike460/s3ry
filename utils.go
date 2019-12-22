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
		fmt.Println(i18nPrinter.Sprintf("The file exists. Overwrite? File name:% s, [Yy] / [Nn]", filename))
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
		if file.IsDir() {
			paths = append(paths, dirwalk(filepath.Join(dir, file.Name()))...)
			continue
		}
		paths = append(paths, filepath.Join(dir, file.Name()))
	}
	return paths
}
