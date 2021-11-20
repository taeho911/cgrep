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

func main() {
	var ra regexArray
	var ia invertArray
	var sa skipDirArray
	var dirs []string
	encs := []string{"utf8", "sjis", "encjp", "iso2022jp", "enckr"}
	// In case of encountering .git and .svn directories, cgrep will skip those directories by default
	defaultSkipDirs := []string{".git", ".svn"}

	// Setting available flags
	flag.Var(&ra, "e", "Regex")
	flag.Var(&ia, "v", "Invert match")
	flag.Var(&sa, "s", "Skip dir")
	allFlag := flag.Bool("all", false, "Search all directories including .git and .svn")
	concFlag := flag.Bool("c", false, "Concurrent search")
	caseInsensitiveFlag := flag.Bool("i", false, "Case insensitive match")
	enc := flag.String("enc", encs[0], fmt.Sprintf("Encoding %v", encs))
	filenameOnlyFlag := flag.Bool("f", false, "Print filename only")
	flag.Parse()

	// Rest of arguments after parsing flags
	tail := flag.Args()

	// Validate arguments
	// When there is no -e option, cgrep guess the tail would be the pattern to match
	if len(ra) == 0 {
		// If there is no -e options and no tail, cgrep will exit program because there is nothing to search
		if len(tail) < 1 {
			fmt.Fprintf(os.Stderr, "Args not enough => len(args) = %d\n", len(tail))
			os.Exit(1)
		}
		ra = append(ra, tail[0])
		dirs = tail[1:]
	} else {
		dirs = tail[:]
	}
	// Exit program because there is no pattern to match
	if len(ra) < 1 {
		fmt.Fprintf(os.Stderr, "No regex => len(regex) = %d\n", len(ra))
		os.Exit(1)
	}
	// If there is no designated searching target directory, cgrep search from current directory by default
	if len(dirs) < 1 {
		dirs = append(dirs, ".")
	}
	// if all flag is false, append default skip directories (.git, .svn)
	if !(*allFlag) {
		for i := 0; i < len(defaultSkipDirs); i++ {
			sa = append(sa, defaultSkipDirs[i])
		}
	}
	var cra []regexp.Regexp
	for i := 0; i < len(ra); i++ {
		if *caseInsensitiveFlag {
			// In case of case insensitive matching, add (?i) regex
			cra = append(cra, *regexp.MustCompile("(?i)" + ra[i]))
		} else {
			cra = append(cra, *regexp.MustCompile(ra[i]))
		}
	}
	// Set encoding method according to --enc option
	var encoding encoding.Encoding
	switch *enc {
	case encs[0]:
		encoding = unicode.UTF8
	case encs[1]:
		encoding = japanese.ShiftJIS
	case encs[2]:
		encoding = japanese.EUCJP
	case encs[3]:
		encoding = japanese.ISO2022JP
	case encs[4]:
		encoding = korean.EUCKR
	default:
		encoding = unicode.UTF8
	}
	grepContents(cra, ia, sa, dirs, *concFlag, encoding, *filenameOnlyFlag)
}

func grepContents(
	cra []regexp.Regexp,
	ia invertArray,
	sa skipDirArray,
	dirs []string,
	concFlag bool,
	encoding encoding.Encoding,
	filenameOnlyFlag bool,
) {
	// Make wait group to control goroutine
	var wg sync.WaitGroup
	// Walk through searching target dirs
	for i := 0; i < len(dirs); i++ {
		err := filepath.Walk(dirs[i], func(path string, info os.FileInfo, err error) error {
			skipDir := false
			if info == nil {
				return filepath.SkipDir
			}
			fileName := info.Name()
			// If the name of dir is in skip dirs list, skip searching
			for i := 0; i < len(sa); i++ {
				if fileName == sa[i] {
					skipDir = true
					break
				}
			}
			if skipDir {
				return filepath.SkipDir
			}
			// In case of file
			if !info.IsDir() {
				// Concurrent searching via goroutine
				if concFlag {
					wg.Add(1)
					go grepWorkConc(path, cra, ia, encoding, &wg, filenameOnlyFlag)
				} else {
					// Normal searching
					grepWork(path, cra, ia, encoding, filenameOnlyFlag)
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

// Actually grepping function for concurrent searching
func grepWorkConc(
	file string,
	cra []regexp.Regexp,
	ia invertArray,
	encoding encoding.Encoding,
	wg *sync.WaitGroup,
	filenameOnlyFlag bool,
) {
	defer wg.Done()
	// Open and read the target file
	fp, err := os.Open(file)
	if err != nil {
		return
	}
	defer fp.Close()
	reader := transform.NewReader(fp, encoding.NewDecoder())
	scanner := bufio.NewScanner(reader)
	lineNum := 0
	var greps []grep
	// Scan the file line by line
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		// If the content contains pattern not to match, skip the line
		if len(ia) > 0 && matchArray(line, ia) {
			continue
		}
		// If the content contains pattern to match, save the line information
		if matchArray(line, cra) {
			greps = append(greps, grep{lineNum, line})
		}
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
	if err := scanner.Err(); err != nil {
		return
	}
}

// Actually grepping function for normal searching
func grepWork(
	file string,
	cra []regexp.Regexp,
	ia invertArray,
	encoding encoding.Encoding,
	filenameOnlyFlag bool,
) {
	// Open and read the target file
	fp, err := os.Open(file)
	if err != nil {
		return
	}
	defer fp.Close()
	reader := transform.NewReader(fp, encoding.NewDecoder())
	scanner := bufio.NewScanner(reader)
	// If you want to change buffer size ...
	// const maxBufSize = 256
	// scanner.Buffer(make([]byte, maxBufSize), maxBufSize)
	lineNum := 0
	var greps []grep
	// Scan the file line by line
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		// If the content contains pattern not to match, skip the line
		if len(ia) > 0 && matchArray(line, ia) {
			continue
		}
		// If the content contains pattern to match, save the line information
		if matchArray(line, cra) {
			greps = append(greps, grep{lineNum, line})
		}
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
	if err := scanner.Err(); err != nil {
		return
	}
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
	fmt.Print(b.String())
}
