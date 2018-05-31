package syntax

import (
	"bytes"
	"os"
	"testing"
)

var rwtestcode = `package main

import (
	"json"
	"fmt"
)

func main() {
	x := map[string]interface{}{"a": "b"}
	y := map[string]interface{}{"c": 'd'}
	z := map[string]interface{}{"y": 12}

	var err error

	collect err {
		var (
			data1, data2, data3 []byte
		)

		data1, _! = json.Marshal(x)
		data2, _! = json.Marshal(y)
		data3, _! = json.Marshal(z)

		fmt.Printf("%s.%s.%s", data1, data2, data3)
	}

	if err != nil {
		fmt.Println("ERR:", err)
	}
}
`

func TestRewrite(t *testing.T) {
	file, err := Parse(NewFileBase("<test>"), bytes.NewBuffer([]byte(rwtestcode)), nil, nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	Fdump(os.Stdout, file)

	err = RewriteFile(file)
	if err != nil {
		t.Fatal(err)
	}

	Fprint(os.Stdout, file, true)
}
