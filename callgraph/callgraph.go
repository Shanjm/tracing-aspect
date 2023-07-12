package callgraph

import (
	"regexp"
	"strings"

	"github.com/Shanjm/tracing-aspect/analysis"
	"github.com/Shanjm/tracing-aspect/log"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa/ssautil"
)

type CallingMap map[*analysis.Member][]*analysis.Member

func (c CallingMap) merge(nl map[string]*analysis.Member) {
	for n, mem := range nl {
		father := n[:strings.LastIndex(n, "$")]
		if strings.HasSuffix(father, "init") {
			continue
		}
		faMem := c.findByName(father)
		if faMem != nil {
			c.attach(faMem, mem)
		}
	}
}

func (c CallingMap) findByName(n string) *analysis.Member {
	for mem := range c {
		if mem.Name == n {
			return mem
		}
	}
	return nil
}

func (c CallingMap) attach(father, son *analysis.Member) {
	callees := c[father]
	for _, callee := range callees {
		if callee == son {
			return
		}
	}
	c[father] = append(c[father], son)
}

// GenerateCallgraph TODO
// 源码 -> 第三方 -> 第三方 -> 源码 可能存在问题
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
		log.Panicln(err)
		return nil, err
	}

	nameless := regexp.MustCompile(`\(?\*?[a-zA-Z.]+\)?\.[a-zA-Z]+\$[0-9]+`)
	namelessCallMap := make(map[string]*analysis.Member)
	for caller := range callMap {
		if nameless.MatchString(caller.Name) {
			namelessCallMap[caller.Name] = caller
		}
	}
	callMap.merge(namelessCallMap)
	log.Debugln("get the callgraph done")

	return callMap, nil
}
