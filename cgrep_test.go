package main

import (
	"regexp"
	"testing"
)

func TestProcessTailArgs(t *testing.T) {
	ras := []regexArray{
		{},
		{"A"},
	}
	testcases := [][]string{
		{},
		{"1"},
		{"1", "2"},
	}
	for i, testcase := range testcases {
		lenCase := len(testcase)
		for j, ra := range ras {
			ra, dirs, err := processTailArgs(testcase, ra)
			lenRa := len(ra)
			lenDirs := len(dirs)
			t.Logf("len(ra)/len(dirs)/err==nil => %v/%v/%v", lenRa, lenDirs, err == nil)
			switch j {
			case 0:
				switch i {
				case 0:
					if err == nil {
						t.Fatalf("i/j/len(ra)/len(dirs)/err==nil => %v/%v/%v/%v/%v", i, j, lenRa, lenDirs, err == nil)
					}
				case 1:
					if lenRa != 1 || lenDirs != lenCase-1 || err != nil {
						t.Fatalf("i/j/len(ra)/len(dirs)/err==nil => %v/%v/%v/%v/%v", i, j, lenRa, lenDirs, err == nil)
					}
				case 2:
					if lenRa != 1 || lenDirs != lenCase-1 || err != nil {
						t.Fatalf("i/j/len(ra)/len(dirs)/err==nil => %v/%v/%v/%v/%v", i, j, lenRa, lenDirs, err == nil)
					}
				}
			case 1:
				switch i {
				case 0:
					if lenRa != 1 || lenDirs != lenCase || err != nil {
						t.Fatalf("i/j/len(ra)/len(dirs)/err==nil => %v/%v/%v/%v/%v", i, j, lenRa, lenDirs, err == nil)
					}
				case 1:
					if lenRa != 1 || lenDirs != lenCase || err != nil {
						t.Fatalf("i/j/len(ra)/len(dirs)/err==nil => %v/%v/%v/%v/%v", i, j, lenRa, lenDirs, err == nil)
					}
				case 2:
					if lenRa != 1 || lenDirs != lenCase || err != nil {
						t.Fatalf("i/j/len(ra)/len(dirs)/err==nil => %v/%v/%v/%v/%v", i, j, lenRa, lenDirs, err == nil)
					}
				}
			}
		}
	}
}

func TestCompileRegex(t *testing.T) {
	regexs := regexArray{"A", "B"}
	expectTure := regexp.MustCompile(`^\(\?i\).*`)
	testcases := []bool{true, false}
	for _, testcase := range testcases {
		compiledRegexArray := compileRegex(regexs, testcase)
		for _, item := range compiledRegexArray {
			t.Log(item.String())
			switch testcase {
			case true:
				if !expectTure.Match([]byte(item.String())) {
					t.Fatalf("testcase = %v, expression = %v", testcase, item.String())
				}
			case false:
				if expectTure.Match([]byte(item.String())) {
					t.Fatalf("testcase = %v, expression = %v", testcase, item.String())
				}
			}
		}
	}
}
