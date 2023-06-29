// 原生方法
package config

import "golang.org/x/tools/go/ssa"

type originHttp struct {
}

func (o *originHttp) IsHandler(fun *ssa.Function) bool {

	if fun.Name() != "ServeHTTP" {
		return false
	}

	num := len(fun.Params)
	if num != 2 && num != 3 {
		return false
	}

	p1 := fun.Params[num-1]
	if p1.Type().String() != "*net/http.Request" {
		return false
	}
	p2 := fun.Params[num-2]
	return p2.Type().String() == "net/http.ResponseWriter"
}
