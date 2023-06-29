package main // Package main 分析测试代码

import (
	"fmt"
	"net/http"
	"strconv"

	"githun.com/Shanjm/tracing-aspect/test/util"
)

func main() {
	engine := &Engine{}
	http.HandleFunc("/", engine.ServeHTTP)
	fmt.Println("start")
	http.ListenAndServe(":8080", nil)
}

type Engine struct{}

func (e *Engine) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		rw.Write([]byte("only support GET"))
		return
	}
	v := req.URL.Query().Get("value")
	intv, err := strconv.ParseInt(v, 10, 8)
	if err != nil {
		rw.Write([]byte("argument invalid"))
		return
	}
	m := &util.Monster{O: int(intv)}
	ret, _ := m.Add()
	rw.Write([]byte(ret))
}
