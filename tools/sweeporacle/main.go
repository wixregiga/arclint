// sweeporacle prints the line sources go/scanner implies for each Go file
// named on standard input, one "path:line<TAB>source" per line. The
// domain-sweep skill's inventory script classifies Go lines with its own
// lexer; its tests compare that lexer with this oracle on real files. It
// is not linked into arclint.
package main

import (
	"bufio"
	"fmt"
	"go/scanner"
	"go/token"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// words matches a literal holding two words, as the inventory's WORDS does.
var words = regexp.MustCompile(`[A-Za-z]{2,}[ \t]+[A-Za-z]{2,}`)

// precedence is the order in which one line's sources win, as the
// inventory's pick does.
var precedence = []string{"test", "comment", "text"}

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "sweeporacle:", err)
		os.Exit(1)
	}
}

func run(in io.Reader, out io.Writer) error {
	paths := bufio.NewScanner(in)
	w := bufio.NewWriter(out)
	for paths.Scan() {
		if err := classify(paths.Text(), w); err != nil {
			return err
		}
	}
	if err := paths.Err(); err != nil {
		return fmt.Errorf("read paths: %w", err)
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("write sources: %w", err)
	}
	return nil
}

// classify writes the sources of one file's non-blank lines: a comment,
// a string literal that spans lines or holds two words, or a func Test...(
// declaration.
func classify(path string, w *bufio.Writer) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	fset := token.NewFileSet()
	file := fset.AddFile(path, -1, len(src))
	var s scanner.Scanner
	var scanErr error
	s.Init(file, src, func(pos token.Position, msg string) {
		if scanErr == nil {
			scanErr = fmt.Errorf("%s: %s", pos, msg)
		}
	}, scanner.ScanComments)
	kinds := map[int]map[string]bool{}
	add := func(line int, kind string) {
		if kinds[line] == nil {
			kinds[line] = map[string]bool{}
		}
		kinds[line][kind] = true
	}
	afterFunc := false
	testName := 0 // the line of a func Test... name, until its parenthesis
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		start := fset.Position(pos).Line
		end := start + strings.Count(lit, "\n")
		switch {
		case tok == token.COMMENT:
			for line := start; line <= end; line++ {
				add(line, "comment")
			}
		case tok == token.STRING && end > start:
			for line := start; line <= end; line++ {
				add(line, "text")
			}
		case tok == token.STRING && lit[0] != '\'' && words.MatchString(lit[1:len(lit)-1]):
			add(start, "text")
		case tok == token.LPAREN && testName > 0:
			// A test takes its parameters at once; func TestX[T any]( is
			// a generic function, not a test.
			add(testName, "test")
		}
		testName = 0
		if tok == token.IDENT && afterFunc && strings.HasPrefix(lit, "Test") {
			testName = start
		}
		afterFunc = tok == token.FUNC
	}
	if scanErr != nil {
		return scanErr
	}
	lines := strings.Split(string(src), "\n")
	for line := 1; line <= len(lines); line++ {
		if strings.TrimSpace(strings.TrimSuffix(lines[line-1], "\r")) == "" {
			continue
		}
		for _, kind := range precedence {
			if kinds[line][kind] {
				if _, err := fmt.Fprintf(w, "%s:%s\t%s\n", path, strconv.Itoa(line), kind); err != nil {
					return fmt.Errorf("write sources: %w", err)
				}
				break
			}
		}
	}
	return nil
}
