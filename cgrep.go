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

type regexArray []string
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
	*ra = append(*ra, val)
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
	caseInsensitiveFlag := flag.Bool("i", false, "Case insensitive match")
	flag.Parse()
	tail := flag.Args()
	if len(ra) == 0 {
		if len(tail) < 1 {
			fmt.Fprintf(os.Stderr, "Args not enough => len(args) = %d\n", len(tail))
			os.Exit(1)
		}
		ra = append(ra, tail[0])
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
	var cra []regexp.Regexp
	for i := 0; i < len(ra); i++ {
		if *caseInsensitiveFlag {
			cra = append(cra, *regexp.MustCompile("(?i)" + ra[i]))
		} else {
			cra = append(cra, *regexp.MustCompile(ra[i]))
		}
	}
	grepContents(cra, ia, sa, dirs, *concFlag)
}

func grepContents(cra []regexp.Regexp, ia invertArray, sa skipDirArray, dirs []string, concFlag bool) {
	var wg sync.WaitGroup
	for i := 0; i < len(dirs); i++ {
		err := filepath.Walk(dirs[i], func(path string, info os.FileInfo, err error) error {
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
				if concFlag {
					wg.Add(1)
					go grepWorkConc(path, cra, ia, &wg)
				} else {
					grepWork(path, cra, ia)
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

func grepWorkConc(file string, cra []regexp.Regexp, ia invertArray, wg *sync.WaitGroup) {
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
		if matchArray(line, cra) {
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

func grepWork(file string, cra []regexp.Regexp, ia invertArray) error {
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
		if matchArray(line, cra) {
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

func matchArray(str string, cra []regexp.Regexp) bool {
	for i := 0; i < len(cra); i++ {
		if cra[i].MatchString(str) {
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
}