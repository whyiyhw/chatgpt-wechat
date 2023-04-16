package tiktoken

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dlclark/regexp2"
)

func GetEncoding(encodingName string) (*Tiktoken, error) {
	enc, err := getEncoding(encodingName)
	if err != nil {
		return nil, err
	}
	pbe, err := NewCoreBPE(enc.MergeableRanks, enc.SpecialTokens, enc.PatStr)
	if err != nil {
		return nil, err
	}
	specialTokensSet := map[string]bool{}
	for k := range enc.SpecialTokens {
		specialTokensSet[k] = true
	}
	return &Tiktoken{
		bpe:         pbe,
		pbeEncoding: enc,
	}, nil
}

func EncodingForModel(modelName string) (*Tiktoken, error) {
	if encodingName, ok := ModelToEncoding[modelName]; !ok {
		return nil, fmt.Errorf("no encoding for model %s", modelName)
	} else {
		return GetEncoding(encodingName)
	}
}

type Tiktoken struct {
	bpe              *CoreBPE
	pbeEncoding      *Encoding
	specialTokensSet map[string]any
}

func (t *Tiktoken) Encode(text string, allowedSpecial []string, disallowedSpecial []string) []int {
	var allowedSpecialSet map[string]any
	if len(allowedSpecial) == 0 {
		allowedSpecialSet = map[string]any{}
	} else if len(disallowedSpecial) == 1 && disallowedSpecial[0] == "all" {
		allowedSpecialSet = t.specialTokensSet
	} else {
		allowedSpecialSet = map[string]any{}
		for _, v := range allowedSpecial {
			allowedSpecialSet[v] = nil
		}
	}

	var disallowedSpecialSet map[string]any
	if len(disallowedSpecial) == 0 || (len(disallowedSpecial) == 1 && disallowedSpecial[0] == "all") {
		disallowedSpecialSet = map[string]any{}
		for k1 := range t.specialTokensSet {
			if _, ok := allowedSpecialSet[k1]; !ok {
				disallowedSpecialSet[k1] = nil
			}
		}
	} else {
		disallowedSpecialSet = map[string]any{}
		for _, v := range disallowedSpecial {
			disallowedSpecialSet[v] = nil
		}
	}

	if len(disallowedSpecialSet) > 0 {
		specialRegex := t.SpecialTokenRegex(disallowedSpecialSet)
		m := findRegex2StringMatch(text, specialRegex)
		if m != "" {
			panic(fmt.Sprintf("text contains disallowed special token %s", m))
		}
	}

	tokens, _ := t.bpe.encodeNative(text, allowedSpecialSet)
	return tokens
}

func (t *Tiktoken) Decode(tokens []int) string {
	return string(t.bpe.decodeNative(tokens))
}

func (t *Tiktoken) SpecialTokenRegex(disallowedSpecialSet map[string]any) *regexp2.Regexp {
	specialRegexStrs := make([]string, 0, len(disallowedSpecialSet))
	for k := range disallowedSpecialSet {
		specialRegexStrs = append(specialRegexStrs, regexp.QuoteMeta(k))
	}
	specialRegex, _ := regexp2.Compile(strings.Join(specialRegexStrs, "|"), regexp2.None)
	return specialRegex
}

func findRegex2StringMatch(text string, reg *regexp2.Regexp) string {
	m, _ := reg.FindStringMatch(text)
	if m == nil {
		return ""
	}

	return m.String()
}
