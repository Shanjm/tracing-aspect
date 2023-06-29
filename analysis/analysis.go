package analysis

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"githun.com/Shanjm/tracing-aspect/log"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

var (
	ErrMissingMain = errors.New("could not find the main function")
)

// ParseProject 解析项目代码，入参为项目根目录，返回项目解析结果
func ParseProject(propath string) *Project {
	program, ssaPkgs := buildSSA(propath)
	mains := ssautil.MainPackages(ssaPkgs)
	// 选第一个 main 包作为根包
	if len(mains) == 0 {
		log.Panicln(ErrMissingMain)
	}
	rootPkg := mains[0].Pkg.Path()

	p := &Project{
		RootPkg:    rootPkg,
		SsaProgram: program,
		SsaPkgs:    ssaPkgs,
		Pm:         make(map[string]*Package),
	}

	// 通过 ssautil 获取所有函数
	allfunc := ssautil.AllFunctions(program)
	for fun := range allfunc {
		if !strings.Contains(fun.String(), rootPkg) {
			continue
		}
		p.Add(fun)
	}

	p.CheckOtherMember()
	return p
}

// buildSSA 构建 ssa
func buildSSA(projectPath string) (program *ssa.Program, ssaPkgs []*ssa.Package) {
	pkgs, _ := packages.Load(&packages.Config{
		Mode: packages.NeedCompiledGoFiles |
			packages.NeedDeps |
			packages.NeedEmbedFiles |
			packages.NeedEmbedPatterns |
			packages.NeedExportFile |
			packages.NeedFiles |
			packages.NeedImports |
			packages.NeedModule |
			packages.NeedName |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedTypesSizes,
		Tests: false,
		Dir:   projectPath,
		ParseFile: func(fset *token.FileSet, filename string, src []byte) (*ast.File, error) {
			if strings.HasSuffix(filename, "_test.go") {
				return nil, nil
			}
			return parser.ParseFile(fset, filename, src, parser.ParseComments)
		},
	}, projectPath+"/...")

	program, ssaPkgs = ssautil.AllPackages(pkgs, 0)
	for _, p := range ssaPkgs {
		if p != nil {
			p.Build()
		}
	}
	return
}
