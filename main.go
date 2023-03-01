package main

import (
	"encoding/json"
	"log"
	"math"
	"os"
	"path"
	"sort"
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
type DocFreq = map[string]int

func calculateTF(term string, tfTable TermFreq) float32 {
	// TODO: cache this
	var sumOfTerms int = 0
	for _, v := range tfTable {
		sumOfTerms += v
	}
	return float32(tfTable[term]) / float32(sumOfTerms)
}

func calculateIDF(df int, n int) float32 {
	return float32(math.Log(float64(n) / math.Max(float64(df), 1)))
}

type Model struct {
	TF TermFreqTable `json:"tf"`
	DF DocFreq       `json:"df"`
}

func newModel() *Model {
	return &Model{
		TF: make(map[string]map[string]int),
		DF: make(map[string]int),
	}
}

func newModelFromJson(path string) (*Model, error) {
	data, err := readFile(path)
	if err != nil {
		return nil, err
	}

	var model Model
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, err
	}

	return &model, nil
}

func (m *Model) saveAsJson(path string) error {
	json, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	return os.WriteFile(path, json, 0666)
}

func (m *Model) indexFolder(path string) error {
	paths, err := readDir(path)
	if err != nil {
		return err
	}

	for _, filePath := range paths {
		log.Printf("Indexing: %s", filePath)
		content, err := readFile(filePath)
		if err != nil {
			return err
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

		for t := range tf {
			m.DF[t] += 1
		}

		m.TF[filePath] = tf

	}
	return nil
}

func tokenize(term string) []string {
	lexer := NewLexer([]rune(string(term)))
	result := make([]string, 0)

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

		result = append(result, string(token))
	}

	return result
}

func (m *Model) search(query string) SearchResults {
	result := make(SearchResults, 0)
	tokens := tokenize(query)

	for path, tfTable := range m.TF {
		var rank float32 = 0
		for _, token := range tokens {
			rank += calculateTF(token, tfTable) * calculateIDF(m.DF[token], len(m.TF))
		}

		result = append(result, SearchResult{
			Path: path,
			Rank: rank,
		})
	}

	// result = sortMap(result)

	sort.Sort(sort.Reverse(result))

	return result
}

type SearchResult struct {
	Path string
	Rank float32
}
type SearchResults []SearchResult

func (a SearchResults) Len() int           { return len(a) }
func (a SearchResults) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a SearchResults) Less(i, j int) bool { return a[i].Rank < a[j].Rank }

func main() {
	model, err := newModelFromJson("index-new.json")
	if err != nil {
		log.Fatal(err)
	}
	// model := newModel()
	// model.indexFolder("docs.gl/gl4")

	searchResult := model.search(os.Args[1])
	// log.Println(searchResult[:10])
	for _, v := range searchResult[:10] {
		log.Printf("%s => %f", v.Path, v.Rank)
	}

	// model.saveAsJson("index-new.json")

}
