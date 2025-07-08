package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	out := os.Stdout
	if !(len(os.Args) == 2 || len(os.Args) == 3) {
		panic("usage go run main.go . [-f]")
	}
	path := os.Args[1]
	printFiles := len(os.Args) == 3 && os.Args[2] == "-f"
	err := dirTree(out, path, printFiles)
	if err != nil {
		panic(err.Error())
	}
}

func dirTree(out io.Writer, path string, printFiles bool) error {
	var collection []string

	if err := filepath.Walk(path, collectSlice(&collection, printFiles)); err != nil {
		fmt.Printf("Some error: %v\n", err)
	}

	sort.Slice(collection, func(i, j int) bool {
		pi := strings.Split(collection[i], "/")
		pj := strings.Split(collection[j], "/")

		for k := 0; k < min(len(pi), len(pj)); k++ {
			if pi[k] != pj[k] {
				return pi[k] < pj[k]
			}
		}

		return len(pi) > len(pj)
	})

	res := strings.Join(print(collection), "")

	io.WriteString(out, res)

	return nil
}

func collectSlice(res *[]string, printFiles bool) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		fileName := strings.Join(strings.Split(path, "/")[1:], "/")
		if len(fileName) == 0 {
			return nil
		}

		if info.IsDir() {
			*res = append(*res, fileName)
		} else if printFiles {
			var size string
			if info.Size() == 0 {
				size = "empty"
			} else {
				size = fmt.Sprintf("%db", info.Size())
			}
			fileInfo := fmt.Sprintf("%s (%s)", fileName, size)
			*res = append(*res, fileInfo)
		}

		return nil
	}
}

func print(collection []string) []string {

	var res []string
	var nest []string

	for idx, val := range collection {
		sep := strings.Split(val, string(os.PathSeparator))
		if len(sep) > 1 {
			nest = append(nest, strings.Join(sep[1:], string(os.PathSeparator)))
		} else {
			if idx == len(collection)-1 {
				val = fmt.Sprintf("└───%s\n", val)
			} else {
				val = fmt.Sprintf("├───%s\n", val)
			}
			res = append(res, val)
			for _, nval := range print(nest) {
				if idx == len(collection)-1 {
					nval = fmt.Sprintf("\t%s", nval)
				} else {
					nval = fmt.Sprintf("│\t%s", nval)
				}
				res = append(res, nval)
			}
			nest = nil
		}
	}
	return res
}
