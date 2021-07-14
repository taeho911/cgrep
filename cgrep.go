package main

import (
	"os"
	"flag"
	"fmt"
	"bufio"
	"path/filepath"
	"regexp"
	"sync"
	"bytes"
)

type regexArray []regexp.Regexp
type invertArray []regexp.Regexp
type skipDirArray []string
type grep struct {
	lineNum int
	line string
}
type grepStruct struct {
	file string
	greps []grep
}

func (ra *regexArray) String() string {
	return ""
}

func (ra *regexArray) Set(val string) error {
	*ra = append(*ra, *regexp.MustCompile(val))
	return nil
}

func (ia *invertArray) String() string {
	return ""
}

func (ia *invertArray) Set(val string) error {
	*ia = append(*ia, *regexp.MustCompile(val))
	return nil
}

func (sa *skipDirArray) String() string {
	return ""
}

func (sa *skipDirArray) Set(val string) error {
	*sa = append(*sa, val)
	return nil
}

func main() {
	var ra regexArray
	var ia invertArray
	var sa skipDirArray
	var dirs []string
	flag.Var(&ra, "e", "Regex")
	flag.Var(&ia, "v", "Invert match")
	flag.Var(&sa, "s", "Skip dir")
	concFlag := flag.Bool("c", false, "Concurrent search")
	flag.Parse()
	tail := flag.Args()
	if len(ra) == 0 {
		if len(tail) < 1 {
			fmt.Fprintf(os.Stderr, "Args not enough => len(args) = %d\n", len(tail))
			os.Exit(1)
		}
		ra = append(ra, *regexp.MustCompile(tail[0]))
		dirs = tail[1:]
	} else {
		dirs = tail[:]
	}
	if len(ra) < 1 {
		fmt.Fprintf(os.Stderr, "No regex => len(regex) = %d\n", len(ra))
		os.Exit(1)
	}
	if len(dirs) < 1 {
		dirs = append(dirs, ".")
	}
	grepContents(ra, ia, sa, dirs, *concFlag)
}

func grepContents(ra regexArray, ia invertArray, sa skipDirArray, dirs []string, concFlag bool) {
	var wg sync.WaitGroup
	for i := 0; i < len(dirs); i++ {
		// fmt.Println(dirs[i])
		err := filepath.Walk(dirs[i], func(path string, info os.FileInfo, err error) error {
			// fmt.Println(path)
			skipDir := false
			fileName := info.Name()
			for i := 0; i < len(sa); i++ {
				if fileName == sa[i] {
					skipDir = true
					break
				}
			}
			if skipDir {
				return filepath.SkipDir
			}
			if !info.IsDir() {
				// fmt.Println(path)
				if concFlag {
					wg.Add(1)
					go grepWorkConc(path, ra, ia, &wg)
				} else {
					grepWork(path, ra, ia)
				}
			}
			return nil
		})
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
	wg.Wait()
}

func grepWorkConc(file string, ra regexArray, ia invertArray, wg *sync.WaitGroup) {
	defer wg.Done()
	fp, err := os.Open(file)
	if err != nil {
		return
	}
	defer fp.Close()
	scanner := bufio.NewScanner(fp)
	lineNum := 0
	var greps []grep
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if len(ia) > 0 && matchArray(line, ia) {
			continue
		}
		if matchArray(line, ra) {
			greps = append(greps, grep{lineNum, line})
		}
	}
	if len(greps) > 0 {
		grepResult := grepStruct{file, greps}
		printGrepResult(grepResult)
	}
	if err := scanner.Err(); err != nil {
		return
	}
	return
}

func grepWork(file string, ra regexArray, ia invertArray) error {
	fp, err := os.Open(file)
	if err != nil {
		return err
	}
	defer fp.Close()
	scanner := bufio.NewScanner(fp)
	// If you want to increase buffer size
	// const maxBufSize = 256
	// scanner.Buffer(make([]byte, maxBufSize), maxBufSize)

	// TODO::Transforming char

	lineNum := 0
	var greps []grep
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if len(ia) > 0 && matchArray(line, ia) {
			continue
		}
		if matchArray(line, ra) {
			greps = append(greps, grep{lineNum, line})
		}
	}
	if len(greps) > 0 {
		grepResult := grepStruct{file, greps}
		printGrepResult(grepResult)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func matchArray(str string, ra []regexp.Regexp) bool {
	for i := 0; i < len(ra); i++ {
		if ra[i].MatchString(str) {
			return true
		}
	}
	return false
}

func printGrepResult(gr grepStruct) {
	var b bytes.Buffer
	b.WriteString(fmt.Sprintf("---\n%s\n", gr.file))
	for i := 0; i < len(gr.greps); i++ {
		b.WriteString(fmt.Sprintf("%-5d %s\n", gr.greps[i].lineNum, gr.greps[i].line))
	}
	fmt.Printf(b.String())
	
	// fmt.Println("---")
	// fmt.Println(gr.file)
	// for i := 0; i < len(gr.greps); i++ {
	// 	fmt.Printf("%-5d %s\n", gr.greps[i].lineNum, gr.greps[i].line)
	// }
}