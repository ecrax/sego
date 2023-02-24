package main

import (
	"encoding/json"
	"log"
	"math"
	"os"
	"path"
	"unicode"
)

type lexer struct {
	content []rune
}

func NewLexer(content []rune) *lexer {
	return &lexer{content: content}
}

func (l *lexer) trimLeft() {
	for len(l.content) > 0 && unicode.IsSpace(l.content[0]) {
		l.content = l.content[1:]
	}
}

func (l *lexer) chop(n int) []rune {
	token := l.content[0:n]
	l.content = l.content[n:]
	return token
}

func (l *lexer) chopWhile(predicate func(rune) bool) []rune {
	n := 0
	for n < len(l.content) && predicate(l.content[n]) {
		n++
	}
	return l.chop(n)
}

func (l *lexer) Next() (value []rune, hasNext bool) {
	l.trimLeft()
	if len(l.content) == 0 {
		return nil, false
	}

	// HTML Tags, tokenize but don't return them as tokens
	if l.content[0] == '<' {
		n := 0
		for n < len(l.content) && l.content[n] != '>' {
			n++
		}
		l.content = l.content[n+1:]
		return nil, true
	}

	if unicode.IsNumber(l.content[0]) {
		return l.chopWhile(func(r rune) bool {
			return unicode.IsNumber(r)
		}), true
	}

	if unicode.IsLetter(l.content[0]) {
		return l.chopWhile(func(r rune) bool {
			return (unicode.IsLetter(r) || unicode.IsNumber(r))
		}), true
	}

	return l.chop(1), true
}

func readFile(filePath string) ([]byte, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatal(err)
	}
	return content, err
}

func readDir(dirPath string) ([]string, error) {
	dirContent, err := os.ReadDir(dirPath)
	paths := make([]string, 0)
	for _, file := range dirContent {
		if !file.IsDir() {
			paths = append(paths, path.Join(dirPath, file.Name()))
		}
	}

	return paths, err
}

type TermFreq = map[string]int
type TermFreqTable = map[string]TermFreq

type TfIdf = map[string]float32
type PathTfIdf = map[string]TfIdf

func calculateTF(count int, document TermFreq) float32 {
	var sumOfTerms int = 0
	for _, v := range document {
		sumOfTerms += v
	}
	return float32(count) / float32(sumOfTerms)
}

func calculateIDF(numberOfDocs int, numberOfDocsWithTerm int) float32 {
	return float32(math.Log(float64(numberOfDocs) / math.Max(float64(numberOfDocsWithTerm), 1)))
}

func numOfDocsWithTerm(term string, table TermFreqTable) int {
	var numOfDocsWithTerm int = 0
	for _, document := range table {
		if _, ok := document[term]; ok {
			numOfDocsWithTerm++
		}
	}
	return numOfDocsWithTerm
}

func calculateTFIDF(table TermFreqTable) PathTfIdf {
	t := make(PathTfIdf)
	numOfDocs := len(table)

	for path, document := range table {
		log.Printf("Calculating: %s", path)
		t[path] = make(TfIdf)
		for term, count := range document {
			tf := calculateTF(count, document)
			idf := calculateIDF(numOfDocs, numOfDocsWithTerm(term, table))
			tfidf := tf * idf
			if tfidf > 0 {
				t[path][term] = tfidf
			}
		}
	}

	return t
}

func generateTft(dirPath string) (TermFreqTable, error) {
	paths, err := readDir(dirPath)
	if err != nil {
		return nil, err
	}

	tft := make(TermFreqTable)

	for _, filePath := range paths {
		log.Printf("Indexing: %s", filePath)
		content, err := readFile(filePath)
		if err != nil {
			return nil, err
		}

		tf := make(TermFreq)

		lexer := NewLexer([]rune(string(content)))

		for {
			token, hasNext := lexer.Next()
			if !hasNext {
				break
			}

			if token == nil {
				continue
			}

			for i := range token {
				token[i] = unicode.ToUpper(token[i])
			}

			// omit everything less or equal than 2 chars to make table smaller
			// if len(token) <= 2 {
			// 	continue
			// }

			tf[string(token)]++
		}

		tft[filePath] = tf
	}

	return tft, nil
}

func main() {
	tft, err := generateTft("docs.gl/gl4")
	if err != nil {
		log.Fatalln(err)
	}

	t := calculateTFIDF(tft)

	json, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	os.WriteFile("index.json", json, 0666)
}
