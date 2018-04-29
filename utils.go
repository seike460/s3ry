package s3ry

import (
	"fmt"
	"io/ioutil"
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

// CheckLocalExists check localFile
func CheckLocalExists(objectKey string) {
	filename := filepath.Base(objectKey)
	_, err := os.Stat(filename)
	if err == nil {
		var overlide string
		fmt.Printf("そのファイルは存在します。上書きしますか ファイル名:%s, [Yy]/[Nn])\n", filename)
		fmt.Scan(&overlide)
		if overlide != "y" && overlide != "Y" {
			fmt.Println("終了します")
			os.Exit(0)
		}
	}
}

// awsErrorPrint print Error for aws
func awsErrorPrint(err error) {
	if aerr, ok := err.(awserr.Error); ok {
		fmt.Println(aerr.Error())
	} else {
		fmt.Println(err.Error())
	}
	os.Exit(1)
}

// dirwalk get fileList
func dirwalk(dir string) []string {
	if dir == "" {
		dir = "./"
	}
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		panic(err)
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
