package syntax

import (
	"bytes"
	"testing"
)

const sugarizerTestOK = `
package main

import (
	"encoding/json"
	"errors"
	"fmt"
)

func main() {
	var (
		data = []byte("{\"Test\": 12, \"Test2\": \"test\"}")
		
		ok struct {
			Test int
			Test2 string
		}

		bad struct {
			Test string
			Test2 int
		}
	)

	var err error
	_collect_ err {
		other := func() {
			//var x error
			_collect_ err {
				_! = errors.New("test")
			}

			if x != nil {
				panic(x)
			}
		}

		_! = json.Unmarshal(data, &ok)
		_! = json.Unmarshal(data, &bad)

		var otherErr error
		_collect_ otherErr {
			_! = errors.New("oh no!")			
		}
	}

	if err != nil {
		fmt.Println(err)
	}
}
`

func TestSugarizer(t *testing.T) {
	errh := func(e error) { t.Fatal(e) }

	file, _ := Parse(
		NewFileBase("test_sugar.go"),
		bytes.NewBufferString(sugarizerTestOK),
		errh, nil, 0,
	)

	buf := new(bytes.Buffer)
	if err := Fdump(buf, file); err != nil {
		t.Fatal(err)
	}
	t.Logf("\n%s\n", buf)
}
