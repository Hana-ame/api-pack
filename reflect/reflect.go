package reflect

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

func Reflect(w http.ResponseWriter, r *http.Request) {
	printLine := func(x interface{}) {
		val, ok := x.([]uint8)
		if ok {
			w.Write(val)
			return
		}
		str := fmt.Sprintf("%v", x)
		w.Write([]byte(str))
		w.Write([]byte("\n"))
	}
	printLine(`----------head----------`)
	printLine(r.Method)
	printLine(r.Host)
	printLine(r.URL)
	printLine(r.Proto)

	printLine(r.Header)

	printLine(`----------body----------`)
	defer r.Body.Close()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatal(err)
	} else {
		printLine(body)
	}
	printLine(`----------end of body----------`)

}
