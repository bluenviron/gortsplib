package headers

import (
	"fmt"
)

func readKey(origstr string, str string, separator byte) (string, string, error) {
	i := 0
	for {
		if i >= len(str) || str[i] == separator {
			return "", "", fmt.Errorf("unable to read key (%v)", origstr)
		}
		if str[i] == '=' {
			break
		}

		i++
	}
	return str[:i], str[i+1:], nil
}

func readValue(origstr string, str string, separator byte) (string, string, error) {
	if len(str) > 0 && str[0] == '"' {
		i := 1
		for {
			if i >= len(str) {
				return "", "", fmt.Errorf("apexes not closed (%v)", origstr)
			}

			if str[i] == '"' {
				return str[1:i], str[i+1:], nil
			}

			i++
		}
	}

	i := 0
	for {
		if i >= len(str) || str[i] == separator {
			return str[:i], str[i:], nil
		}
		i++
	}
}

func keyValParse(str string, separator byte) (map[string]string, error) {
	ret := make(map[string]string)
	origstr := str

	for len(str) > 0 {
		var k string
		var err error
		k, str, err = readKey(origstr, str, separator)
		if err != nil {
			return nil, err
		}

		var v string
		v, str, err = readValue(origstr, str, separator)
		if err != nil {
			return nil, err
		}

		ret[k] = v

		// skip separator
		if len(str) > 0 && str[0] == separator {
			str = str[1:]
		}

		// skip spaces
		for len(str) > 0 && str[0] == ' ' {
			str = str[1:]
		}
	}

	return ret, nil
}
