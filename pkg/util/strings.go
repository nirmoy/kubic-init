package util

import (
	"encoding/base64"
	"strings"
)

// SafeId returns a safe ID (for example, for using in YAML)
// ie, "something:6000/ddd" becommes "something-6000-ddd"
func SafeId(s string) string {
	replacer := strings.NewReplacer(" ", "-", ":", "-", "/", "-", ".", "-")
	return replacer.Replace(s)
}

func URL64encode(v string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(v))
}

func URL64decode(v string) string {
	data, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		return ""
	}
	return string(data)
}

func RemoveDumplicates(in []string) []string {
	processed  := map[string]struct{}{}

	res := []string{}
	for _, s := range in {
		if _, found := processed[s]; !found {
			processed[s] = struct{}{}
			res = append(res, s)
		}
	}

	return res
}
