package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// Regular expressions to match pattern
type regexArray []string

// Regular expressions not to match pattern
type invertArray []regexp.Regexp

// Directories to skip searching
type skipDirArray []string

// Grepped lines' contents and line numbers
type grep struct {
	lineNum int
	line    string
}

// Grepped results for each file
type grepStruct struct {
	file  string
	greps []grep
}

// Overriding functions to take options' arguments as array
// --------------------------------------------------------
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

// --------------------------------------------------------

// main function mainly does setting of flags and parsing it
func main() {
	var patternList regexArray
	var invertList invertArray
	var skipDirList skipDirArray
	encs := []string{"utf8", "sjis", "encjp", "iso2022jp", "enckr"}
	// In case of encountering .git and .svn directories, cgrep will skip those directories by default
	defaultSkipDirs := []string{".git", ".svn"}

	// Setting available flags
	flag.Var(&patternList, "e", "Regex")
	flag.Var(&invertList, "v", "Invert match")
	flag.Var(&skipDirList, "s", "Skip dir")
	allFlag := flag.Bool("all", false, "Search all directories including .git and .svn")
	concFlag := flag.Bool("c", false, "Concurrent search")
	caseInsensitiveFlag := flag.Bool("i", false, "Case insensitive match")
	enc := flag.String("enc", encs[0], fmt.Sprintf("Encoding %v", encs))
	filenameOnlyFlag := flag.Bool("f", false, "Print filename only")
	flag.Parse()

	patternList, dirs, err1 := processTailArgs(flag.Args(), patternList)
	if err1 != nil {
		fmt.Fprintln(os.Stderr, err1.Error())
		os.Exit(1)
	}
	if err2 := validateRegexArray(patternList); err2 != nil {
		fmt.Fprintln(os.Stderr, err2.Error())
		os.Exit(1)
	}
	dirs = setTargetDirs(dirs)
	skipDirList = setSkipDirs(skipDirList, defaultSkipDirs, *allFlag)
	compiledPatternList := compileRegex(patternList, *caseInsensitiveFlag)
	encoding := setEncoding(*enc, encs)

	walkThroughDirs(compiledPatternList, invertList, skipDirList, dirs, *concFlag, encoding, *filenameOnlyFlag)
}

func processTailArgs(tail []string, patternList regexArray) (regexArray, []string, error) {
	// Process rest of arguments after parsing flags
	var dirs []string
	// When there is no -e option, cgrep guess the tail would be the pattern to match
	if len(patternList) == 0 {
		// If there is no -e options and no tail, cgrep will exit program because there is nothing to search
		if len(tail) < 1 {
			return patternList, dirs, fmt.Errorf("args not enough => len(args) = %d", len(tail))
		}
		patternList = append(patternList, tail[0])
		dirs = tail[1:]
	} else {
		dirs = tail[:]
	}
	return patternList, dirs, nil
}

func validateRegexArray(patternList regexArray) error {
	// Exit program because there is no pattern to match
	if len(patternList) < 1 {
		return fmt.Errorf("noo regex => len(regex) = %d", len(patternList))
	}
	return nil
}

func setTargetDirs(dirs []string) []string {
	// If there is no designated searching target directory, cgrep search from current directory by default
	if len(dirs) < 1 {
		dirs = append(dirs, ".")
	}
	return dirs
}

func setSkipDirs(sa skipDirArray, defaultSkipDirs []string, allFlag bool) skipDirArray {
	// if all flag is false, append default skip directories (.git, .svn)
	if !(allFlag) {
		for i := 0; i < len(defaultSkipDirs); i++ {
			sa = append(sa, defaultSkipDirs[i])
		}
	}
	return sa
}

func setEncoding(enc string, encMaster []string) encoding.Encoding {
	// Set encoding method according to --enc option
	switch enc {
	case encMaster[0]:
		return unicode.UTF8
	case encMaster[1]:
		return japanese.ShiftJIS
	case encMaster[2]:
		return japanese.EUCJP
	case encMaster[3]:
		return japanese.ISO2022JP
	case encMaster[4]:
		return korean.EUCKR
	default:
		return unicode.UTF8
	}
}

func compileRegex(patternList regexArray, caseInsensitiveFlag bool) []regexp.Regexp {
	var compiledRegexArray []regexp.Regexp
	for i := 0; i < len(patternList); i++ {
		if caseInsensitiveFlag {
			// In case of case insensitive matching, add (?i) regex
			compiledRegexArray = append(compiledRegexArray, *regexp.MustCompile("(?i)" + patternList[i]))
		} else {
			compiledRegexArray = append(compiledRegexArray, *regexp.MustCompile(patternList[i]))
		}
	}
	return compiledRegexArray
}

func walkThroughDirs(
	compiledPatternList []regexp.Regexp,
	invertList invertArray,
	skipDirList skipDirArray,
	dirs []string,
	concFlag bool,
	encoding encoding.Encoding,
	filenameOnlyFlag bool,
) {
	// Make wait group to control goroutine
	var wg sync.WaitGroup
	for i := 0; i < len(dirs); i++ {
		err := filepath.Walk(dirs[i], func(path string, info os.FileInfo, err error) error {
			if isSkipDir(info, skipDirList) {
				return filepath.SkipDir
			}
			// In case of file
			if !info.IsDir() {
				// Concurrent searching via goroutine
				if concFlag {
					wg.Add(1)
					go grepWorkConc(path, compiledPatternList, invertList, encoding, &wg, filenameOnlyFlag)
				} else {
					// Normal searching
					grepWork(path, compiledPatternList, invertList, encoding, filenameOnlyFlag)
				}
			}
			return nil
		})
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
	// Block exit until all goroutine is closed
	wg.Wait()
}

func isSkipDir(info os.FileInfo, skipDirList skipDirArray) bool {
	if info == nil || isInSkipDirList(info.Name(), skipDirList) {
		return true
	}
	return false
}

func isInSkipDirList(thing string, array []string) bool {
	for i := 0; i < len(array); i++ {
		if thing == array[i] {
			return true
		}
	}
	return false
}

// Actually grepping function for concurrent searching
func grepWorkConc(
	file string,
	compiledPatternList []regexp.Regexp,
	invertList invertArray,
	encoding encoding.Encoding,
	wg *sync.WaitGroup,
	filenameOnlyFlag bool,
) {
	defer wg.Done()
	greps, err := doGrep(file, compiledPatternList, invertList, encoding)
	if err != nil {
		return
	}
	// If the target file matched at least one pattern, save the target file information
	if len(greps) > 0 {
		grepResult := grepStruct{file, greps}
		if filenameOnlyFlag {
			fmt.Println(file)
		} else {
			printGrepResult(grepResult)
		}
	}
}

// Actually grepping function for normal searching
func grepWork(
	file string,
	compiledPatternList []regexp.Regexp,
	invertList invertArray,
	encoding encoding.Encoding,
	filenameOnlyFlag bool,
) {
	greps, err := doGrep(file, compiledPatternList, invertList, encoding)
	if err != nil {
		return
	}
	// If the target file matched at least one pattern, save the target file information
	if len(greps) > 0 {
		grepResult := grepStruct{file, greps}
		if filenameOnlyFlag {
			fmt.Println(file)
		} else {
			printGrepResult(grepResult)
		}
	}
}

func doGrep(file string, compiledPatternList []regexp.Regexp, invertList invertArray, encoding encoding.Encoding) ([]grep, error) {
	fp, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fp.Close()
	reader := transform.NewReader(fp, encoding.NewDecoder())
	scanner := bufio.NewScanner(reader)
	// If you want to change buffer size ...
	// const maxBufSize = 256
	// scanner.Buffer(make([]byte, maxBufSize), maxBufSize)
	lineNum := 0
	var greps []grep
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		// If the content contains pattern not to match, skip the line
		if len(invertList) > 0 && matchArray(line, invertList) {
			continue
		}
		// If the content contains pattern to match, save the line information
		if matchArray(line, compiledPatternList) {
			greps = append(greps, grep{lineNum, line})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return greps, nil
}

func matchArray(str string, compiledPatternList []regexp.Regexp) bool {
	for i := 0; i < len(compiledPatternList); i++ {
		if compiledPatternList[i].MatchString(str) {
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
	fmt.Print(b.String())
}
