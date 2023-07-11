package main

import (
	"github.com/Shanjm/tracing-aspect/instrument"
)

func main() {
	// ins := instrument.NewInstrument("/data/accurate_test/test", conf)
	ins := instrument.NewInstrument("/data/main/adq/delivery/delivery/server/dpmain")

	// 插桩代码
	ins.Instrument()

	// 导入源码文件, 文件命名无所谓
	// filename := filepath.Join(ins.RootDir, instrument.PackageName+"/goreport.go")
	// os.MkdirAll(filepath.Dir(filename), 0644)
	// os.WriteFile(filename, []byte(instrument.SourceCode), 0644)

	// fmt.Println("插桩完成，请运行go get -u github.com/petermattis/goid")

	// for x := range ins.Calling {
	// 	callerInfoP := ins.Project.SsaProgram.Fset.Position(x.Node.Pos())
	// 	callerInfoE := ins.Project.SsaProgram.Fset.Position(x.Node.End())

	// 	if callerInfoP.Filename != "/data/main/adq/delivery/delivery/server/dpmain/ad/ad.go" {
	// 		continue
	// 	}

	// 	callee := ins.Calling[x]
	// 	for _, ce := range callee {
	// 		calleeInfoP := ins.Project.SsaProgram.Fset.Position(ce.Node.Pos())
	// 		calleeInfoE := ins.Project.SsaProgram.Fset.Position(ce.Node.End())

	// 		fmt.Printf("文件名:%s, 函数名:%s, 位置信息:%d:%d-%d:%d\n---调用--> \n文件名:%s, 函数名:%s, 位置信息:%d:%d-%d:%d\n\n",
	// 			callerInfoP.Filename,
	// 			x.Fun.String(),
	// 			callerInfoP.Line,
	// 			callerInfoP.Column,
	// 			callerInfoE.Line,
	// 			callerInfoE.Column,

	// 			calleeInfoP.Filename,
	// 			ce.Fun.String(),
	// 			calleeInfoP.Line,
	// 			calleeInfoP.Column,
	// 			calleeInfoE.Line,
	// 			calleeInfoE.Column)
	// 	}
	// }
}
