/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package metadata

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

func keyValueParser(r io.Reader) (map[string]string, error) {
	b := bufio.NewReader(r)

	keyvalue := make(map[string]string)

	l := 0

	for {
		line, _, err := b.ReadLine()
		if err == io.EOF {
			break
		}

		l++

		parts := strings.SplitN(string(line), "=", 2)
		if len(parts) < 2 {
			return nil, fmt.Errorf("'=' expected on line %d", l)
		}

		keyvalue[parts[0]] = parts[1]
	}

	return keyvalue, nil
}
