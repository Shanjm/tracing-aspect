package callgraph

import (
	"githun.com/Shanjm/tracing-aspect/analysis"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa/ssautil"
)

type CallingMap map[*analysis.Member][]*analysis.Member

func GenerateCallgraph(ps *analysis.Project) (CallingMap, error) {
	ssaPkgs := ps.SsaPkgs

	// 寻找主函数
	mains := ssautil.MainPackages(ssaPkgs)

	// 使用pointer生成调用链路
	config := &pointer.Config{
		Mains:          mains,
		BuildCallGraph: true,
	}
	result, err := pointer.Analyze(config)
	if err != nil {
		return nil, err
	}

	callMap := make(CallingMap)

	// 遍历调用链路
	err = callgraph.GraphVisitEdges(result.CallGraph, func(edge *callgraph.Edge) error {
		caller := edge.Caller
		callee := edge.Callee

		callerMember, wrapper1 := ps.FindFuncMember(caller.Func)
		calleeMember, _ := ps.FindFuncMember(callee.Func)
		if callerMember == nil || calleeMember == nil {
			return nil
		}
		if wrapper1 && callerMember == calleeMember {
			// 主调方是 wrapper， 且与被调是一样的
			return nil
		}
		if _, ok := callMap[callerMember]; !ok {
			callMap[callerMember] = []*analysis.Member{}
		}
		callMap[callerMember] = append(callMap[callerMember], calleeMember)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return callMap, nil
}
