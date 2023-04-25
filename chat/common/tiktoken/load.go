package tiktoken

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

func loadTokenBee(filename string) (map[string]int, error) {
	contents, err := Assets.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	bpeRanks := make(map[string]int)
	for _, line := range strings.Split(string(contents), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, " ")
		token, err := base64.StdEncoding.DecodeString(parts[0])
		if err != nil {
			return nil, err
		}
		rank, err := strconv.Atoi(strings.Trim(parts[1], "\r"))
		if err != nil {
			fmt.Println("strconv.Atoi error", parts[0], parts[1])
			return nil, err
		}
		bpeRanks[string(token)] = rank
	}
	return bpeRanks, nil
}
