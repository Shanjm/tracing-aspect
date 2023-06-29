package main

import (
	"fmt"
	"os"
	"path/filepath"

	"githun.com/Shanjm/tracing-aspect/config"
	"githun.com/Shanjm/tracing-aspect/instrument"
)

func main() {
	conf := config.LoadLocal("test.jim")
	ins := instrument.NewInstrument("/data/accurate_test/test", conf)

	// 插桩代码
	ins.Instrument()

	// 导入源码文件, 文件命名无所谓
	filename := filepath.Join(ins.RootDir, instrument.PackageName+"/goreport.go")
	os.MkdirAll(filepath.Dir(filename), 0644)
	os.WriteFile(filename, []byte(instrument.SourceCode), 0644)

	fmt.Println("插桩完成，请运行go get -u github.com/petermattis/goid")

	// for x := range ins.Calling {
	// 	callee := ins.Calling[x]
	// 	for _, ce := range callee {
	// 		fmt.Printf("%s:%d:%d-%d:%d---调用--> %s:%d:%d-%d:%d\n",
	// 			x.Fun.String(),
	// 			ins.Project.SsaProgram.Fset.Position(x.Node.Pos()).Line,
	// 			ins.Project.SsaProgram.Fset.Position(x.Node.Pos()).Column,
	// 			ins.Project.SsaProgram.Fset.Position(x.Node.End()).Line,
	// 			ins.Project.SsaProgram.Fset.Position(x.Node.End()).Column,
	// 			ce.Fun.String(),
	// 			ins.Project.SsaProgram.Fset.Position(ce.Node.Pos()).Line,
	// 			ins.Project.SsaProgram.Fset.Position(ce.Node.Pos()).Column,
	// 			ins.Project.SsaProgram.Fset.Position(ce.Node.End()).Line,
	// 			ins.Project.SsaProgram.Fset.Position(ce.Node.End()).Column)
	// 	}
	// }
}
