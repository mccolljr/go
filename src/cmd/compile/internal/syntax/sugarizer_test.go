package syntax

import (
	"fmt"
	"os"
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
	func(){
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
	}()
}
`

func TestSugarizer(t *testing.T) {
	//errh := func(e error) { t.Fatal(e) }
	fmt.Println(os.Getwd())
	_, err := ParseFile("../gc/typecheck.go", nil, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
}
