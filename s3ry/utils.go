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

var sp = spinner.New(spinner.CharSets[34], 100*time.Millisecond)

func sps(label string) {
	fmt.Println(label)
	sp.Start()
}

func spe() {
	sp.Stop()
}

/*
PromptItems Create promptuiItems
*/
type PromptItems struct {
	Key          int
	Val          string
	Size         int64
	LastModified time.Time
	Tag          string
}

/*
CheckLocalExists check localFile
*/
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

func awsErrorPrint(err error) {
	if aerr, ok := err.(awserr.Error); ok {
		fmt.Println(aerr.Error())
	} else {
		fmt.Println(err.Error())
	}
	os.Exit(1)
}

// @todo ディレクトリ構造を全て再帰的に持ってきてアップロード出来たほうが使いやすい
func dirwalk() []string {
	dir, _ := os.Getwd()
	files, _ := ioutil.ReadDir(dir)
	var paths []string
	for _, file := range files {
		if !file.IsDir() {
			paths = append(paths, file.Name())
		}
	}
	return paths
}
