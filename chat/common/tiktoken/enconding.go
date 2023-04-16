package tiktoken

import (
	"errors"
)

const EndOfText string = "<|endoftext|>"
const FimPrefix string = "<|fim_prefix|>"
const FimMiddle string = "<|fim_middle|>"
const FimSuffix string = "<|fim_suffix|>"
const EndOfPrompt string = "<|endofprompt|>"

var ModelToEncoding = map[string]string{
	// chat
	"gpt-4":              "cl100k_base",
	"gpt-3.5-turbo":      "cl100k_base",
	"gpt-3.5-turbo-0301": "cl100k_base",
	// text
	"text-davinci-003": "p50k_base",
	"text-davinci-002": "p50k_base",
	"text-davinci-001": "r50k_base",
	"text-curie-001":   "r50k_base",
	"text-babbage-001": "r50k_base",
	"text-ada-001":     "r50k_base",
	"davinci":          "r50k_base",
	"curie":            "r50k_base",
	"babbage":          "r50k_base",
	"ada":              "r50k_base",
	// code
	"code-davinci-002": "p50k_base",
	"code-davinci-001": "p50k_base",
	"code-cushman-002": "p50k_base",
	"code-cushman-001": "p50k_base",
	"davinci-codex":    "p50k_base",
	"cushman-codex":    "p50k_base",
	// edit
	"text-davinci-edit-001": "p50k_edit",
	"code-davinci-edit-001": "p50k_edit",
	// embeddings
	"text-embedding-ada-002": "cl100k_base",
	// old embeddings
	"text-similarity-davinci-001":  "r50k_base",
	"text-similarity-curie-001":    "r50k_base",
	"text-similarity-babbage-001":  "r50k_base",
	"text-similarity-ada-001":      "r50k_base",
	"text-search-davinci-doc-001":  "r50k_base",
	"text-search-curie-doc-001":    "r50k_base",
	"text-search-babbage-doc-001":  "r50k_base",
	"text-search-ada-doc-001":      "r50k_base",
	"code-search-babbage-code-001": "r50k_base",
	"code-search-ada-code-001":     "r50k_base",
	// open source
	"gpt2": "gpt2",
}

type Encoding struct {
	Name           string
	PatStr         string
	MergeableRanks map[string]int
	SpecialTokens  map[string]int
	ExplicitNVocab int
}

func getEncoding(encodingName string) (*Encoding, error) {
	encoding, ok := EncodingMap[encodingName]
	if !ok {
		initEncoding, err := initEncoding(encodingName)
		if err != nil {
			return nil, err
		}
		encoding = initEncoding
		EncodingMap[encodingName] = encoding
	}
	return encoding, nil
}

func initEncoding(encodingName string) (*Encoding, error) {
	switch encodingName {
	case "cl100k_base":
		return cl100kBase()
	case "p50k_base":
		return p50kBase()
	case "r50k_base":
		return r50kBase()
	case "p50k_edit":
		return p50kEdit()
	default:
		return nil, errors.New("Unknown encoding: " + encodingName)
	}
}

func cl100kBase() (*Encoding, error) {
	ranks, err := loadTokenBee("cl100k_base.token")
	if err != nil {
		return nil, err
	}
	specialTokens := map[string]int{
		EndOfText:   100257,
		FimPrefix:   100258,
		FimMiddle:   100259,
		FimSuffix:   100260,
		EndOfPrompt: 100276,
	}
	return &Encoding{
		Name:           "cl100k_base",
		PatStr:         `(?i:'s|'t|'re|'ve|'m|'ll|'d)|[^\r\n\p{L}\p{N}]?\p{L}+|\p{N}{1,3}| ?[^\s\p{L}\p{N}]+[\r\n]*|\s*[\r\n]+|\s+(?!\S)|\s+`,
		MergeableRanks: ranks,
		SpecialTokens:  specialTokens,
	}, nil
}

func p50kEdit() (*Encoding, error) {
	ranks, err := loadTokenBee("p50k_base.token")
	if err != nil {
		return nil, err
	}
	specialTokens := map[string]int{EndOfText: 50256, FimPrefix: 50281, FimMiddle: 50282, FimSuffix: 50283}
	return &Encoding{
		Name:           "p50k_edit",
		PatStr:         `'s|'t|'re|'ve|'m|'ll|'d| ?\p{L}+| ?\p{N}+| ?[^\s\p{L}\p{N}]+|\s+(?!\S)|\s+`,
		MergeableRanks: ranks,
		SpecialTokens:  specialTokens,
	}, nil
}

func p50kBase() (*Encoding, error) {
	ranks, err := loadTokenBee("p50k_base.token")
	if err != nil {
		return nil, err
	}
	specialTokens := map[string]int{EndOfText: 50256}

	// ExplicitNVocab := 50281
	// max_tokens := int(math.Max(float64(len(special_tokens)), float64(len(ranks))))

	// if len(special_tokens)+len(ranks) != max_tokens {
	// 	return nil, errors.New("special_tokens and ranks must be disjoint")
	// }

	return &Encoding{
		Name:           "p50k_base",
		PatStr:         `'s|'t|'re|'ve|'m|'ll|'d| ?\p{L}+| ?\p{N}+| ?[^\s\p{L}\p{N}]+|\s+(?!\S)|\s+`,
		MergeableRanks: ranks,
		SpecialTokens:  specialTokens,
		ExplicitNVocab: 50281,
	}, nil
}

func r50kBase() (*Encoding, error) {
	ranks, err := loadTokenBee("r50k_base.token")
	if err != nil {
		return nil, err
	}
	specialTokens := map[string]int{EndOfText: 50256}
	return &Encoding{
		Name:           "r50k_base",
		MergeableRanks: ranks,
		PatStr:         `'s|'t|'re|'ve|'m|'ll|'d| ?\p{L}+| ?\p{N}+| ?[^\s\p{L}\p{N}]+|\s+(?!\S)|\s+`,
		SpecialTokens:  specialTokens,
		ExplicitNVocab: 50257,
	}, nil
}

var EncodingMap = map[string]*Encoding{}
