package output

import "github.com/moonrhythm/toon"

func encodeTOON(v any) ([]byte, error) {
	b, err := toon.Marshal(v)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
