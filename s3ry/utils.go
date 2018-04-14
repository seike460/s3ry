package s3ry

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/briandowns/spinner"
	"github.com/manifoldco/promptui"
)

var sp = spinner.New(spinner.CharSets[34], 100*time.Millisecond)

func sps(label string) {
	fmt.Println(label)
	sp.Start()
}

func spe() {
	sp.Stop()
}

type PromptItems struct {
	Key          int
	Val          string
	Size         int64
	LastModified time.Time
	Tag          string
}

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

func run(label string, items []PromptItems) string {
	detail := `
{{ "選択値:" | faint }} {{ .Val }}
`
	for _, item := range items {
		if item.Tag == "Object" {
			detail = `
{{ "選択値:" | faint }} {{ .Val }}
{{ "最終更新日:" | faint }} {{ .LastModified }}
`
		}
		break
	}
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "->{{ .Val | red }}",
		Inactive: "{{ .Val | cyan }}",
		Selected: "選択値 {{ .Val | red | cyan }}",
		Details:  detail,
	}

	searcher := func(input string, index int) bool {
		item := items[index]
		name := strings.Replace(strings.ToLower(item.Val), " ", "", -1)
		input = strings.Replace(strings.ToLower(input), " ", "", -1)
		return strings.Contains(name, input)
	}

	prompt := promptui.Select{
		Label:     label,
		Items:     items,
		Templates: templates,
		Size:      20,
		Searcher:  searcher,
	}

	i, _, err := prompt.Run()

	if err != nil {
		fmt.Printf("選択に失敗しました。終了します %v\n", err)
		os.Exit(1)
	}
	return items[i].Val
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
