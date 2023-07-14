package analysis

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"github.com/Shanjm/tracing-aspect/log"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// ParseProject 解析项目代码，入参为项目根目录，返回项目解析结果
func ParseProject(propath string) (*Project, error) {
	program, ssaPkgs, err := buildSSA(propath)
	if err != nil {
		return nil, err
	}
	mains := ssautil.MainPackages(ssaPkgs)

	rootPkg := ""
	if len(mains) > 0 {
		// 选第一个 main 包作为根包
		rootPkg = mains[0].Pkg.Path()
	}

	p := &Project{
		RootPkg:    rootPkg,
		SsaProgram: program,
		SsaPkgs:    ssaPkgs,
		Pm:         make(map[string]*Package),
		Rely:       make(map[string]string),
		wrappers:   []*ssa.Function{},
	}

	// 通过 ssautil 获取所有函数
	allfunc := ssautil.AllFunctions(program)
	for fun := range allfunc {
		funFile := program.Fset.Position(fun.Pos()).Filename
		if !strings.HasPrefix(funFile, propath) {
			if fun.Pkg != nil && funFile != "" {
				p.Rely[fun.Pkg.Pkg.Path()] = funFile[:strings.LastIndex(funFile, "/")]
			}
			continue
		}
		p.Add(fun)
	}

	p.CheckOtherMember()
	log.Println("finish the parsing project.")
	return p, nil
}

// buildSSA 构建 ssa
func buildSSA(projectPath string) (*ssa.Program, []*ssa.Package, error) {
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

	program, preSsaPkgs := ssautil.AllPackages(pkgs, ssa.GlobalDebug)
	ssaPkgs := []*ssa.Package{}
	for _, p := range preSsaPkgs {
		if p != nil {
			p.Build()
			ssaPkgs = append(ssaPkgs, p)
		}
	}
	log.Println(fmt.Sprintf("共有 %d 个包，正常解析有 %d 个", len(preSsaPkgs), len(ssaPkgs)))
	if len(ssaPkgs) != len(preSsaPkgs) {
		return nil, nil, errors.New("缺少有效的包，源码可能存在错误")
	}
	return program, ssaPkgs, nil
}
