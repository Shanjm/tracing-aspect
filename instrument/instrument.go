package instrument

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"os"
	"path/filepath"

	"github.com/Shanjm/tracing-aspect/analysis"
	"github.com/Shanjm/tracing-aspect/callgraph"
	"github.com/Shanjm/tracing-aspect/log"
)

const (
	PackageName = "goreport"
	ParentId    = "_parentId"
)

var (
	ErrNotDir = errors.New("input project path is not dir")
)

// InsPara 插桩结构体
type InsPara struct {
	RootDir string
	Project *analysis.Project
	Calling callgraph.CallingMap

	funcMap       map[string]struct{}           // 需要追踪的函数
	rewriteMap    map[string]*rewrite           // 重写文件map
	nodeInspected map[ast.Node]struct{}         // 已经访问过的节点
	visited       map[*analysis.Member]struct{} // 已经访问过的函数
}

// 重写结构体
type rewrite struct {
	astfile  *ast.File // 重写的文件
	needGoId bool      // 是否需要获取id
}

// NewInstrument 返回一个插桩结构体
func NewInstrument(root string) *InsPara {
	dir, err := os.Stat(root)
	if !(err == nil && dir.IsDir()) {
		log.Fatalln(ErrNotDir)
	}
	root, _ = filepath.Abs(root)
	return &InsPara{
		RootDir: root,

		funcMap:       make(map[string]struct{}),
		rewriteMap:    make(map[string]*rewrite),
		nodeInspected: make(map[ast.Node]struct{}),
		visited:       make(map[*analysis.Member]struct{}),
	}
}

// Instrument 进行插桩
func (i *InsPara) Instrument() {
	i.parseProject()

	for _, pkg := range i.Project.Pm {
		for _, file := range pkg.Fm {
			for _, fu := range file.FunMember {
				log.Println(fmt.Sprintf("find the entry: %s", fu.Name))
				i.instrument(fu, file.ParsedFile, true)
			}
		}
	}

	i.rewrite()
}

func (i *InsPara) parseProject() {
	result, err := analysis.ParseProject(i.RootDir)
	if err != nil {
		log.Fatalln(err)
	}
	cm, err := callgraph.GenerateCallgraph(result)
	if err != nil {
		log.Fatalln(err)
	}

	i.Project = result
	i.Calling = cm

	log.Println(fmt.Sprintf("the root package: %s", i.Project.RootPkg))
}

// 重写文件
func (i *InsPara) rewrite() {
	for filename := range i.rewriteMap {
		file := i.rewriteMap[filename]

		i.modifyImport(file)

		buffer := bytes.NewBufferString("")
		if err := format.Node(buffer, token.NewFileSet(), file.astfile); err != nil {
			log.Fatalln(err)
		}
		os.WriteFile(filename, buffer.Bytes(), 0644)
	}
}

// 修改import
func (i *InsPara) modifyImport(file *rewrite) {
	file.astfile.Decls = append([]ast.Decl{
		&ast.GenDecl{
			Tok: token.IMPORT,
			Specs: []ast.Spec{
				&ast.ImportSpec{
					Name: &ast.Ident{
						Name: PackageName,
					},
					Path: &ast.BasicLit{
						Kind:  token.STRING,
						Value: fmt.Sprintf("\"%s/%s\"", i.Project.RootPkg, PackageName),
					},
				},
			},
		},
	}, file.astfile.Decls...)

	if file.needGoId {
		file.astfile.Decls = append([]ast.Decl{
			&ast.GenDecl{
				Tok: token.IMPORT,
				Specs: []ast.Spec{
					&ast.ImportSpec{
						Path: &ast.BasicLit{
							Kind:  token.STRING,
							Value: "\"github.com/petermattis/goid\"",
						},
					},
				},
			},
		}, file.astfile.Decls...)
	}
}

func (i *InsPara) instrument(funcMember *analysis.Member, file *ast.File, isStart bool) {
	if _, ok := i.visited[funcMember]; ok {
		log.Println(funcMember.Name + " has visited")
		return
	}
	log.Println(fmt.Sprintf("Start to instrument %s\n", funcMember.Name))

	ast.Inspect(file, func(n ast.Node) bool {

		if _, ok := i.nodeInspected[n]; n == nil || ok {
			return false
		}

		if funcMember.ComparePostion(funcMember.Fun.Prog.Fset.Position(n.Pos()),
			funcMember.Fun.Prog.Fset.Position(n.End())) {

			if _, ok := i.rewriteMap[funcMember.File]; !ok {
				i.rewriteMap[funcMember.File] = &rewrite{
					astfile:  file,
					needGoId: false,
				}
			}
			i.reconstrcut(funcMember, n, isStart)
			i.nodeInspected[n] = struct{}{}
			i.visited[funcMember] = struct{}{}
			return false
		}

		return true
	})

	// 遍历下游
	if down, ok := i.Calling[funcMember]; ok {
		for _, d := range down {
			i.instrument(d, i.Project.Pm[d.Pkg.Pkg.Path()].Fm[d.File].ParsedFile, false)
		}
	}
}

// 重造函数
func (i *InsPara) reconstrcut(funcMember *analysis.Member, node ast.Node, isStart bool) {
	var bodyStmt *ast.BlockStmt
	var funcType *ast.FuncType
	switch x := node.(type) {
	case *ast.FuncLit: // 匿名
		funcType = x.Type
		bodyStmt = x.Body
	case *ast.FuncDecl: // 非匿名
		funcType = x.Type
		bodyStmt = x.Body
	default:
		return
	}

	zeroLineStmts := []ast.Stmt{}
	if isStart {
		zeroLineStmts = append(zeroLineStmts, i.getStartStmt()...)
		zeroLineStmts = append(zeroLineStmts, i.getCopyStmt(funcMember)...)
	}

	if _, ok := i.funcMap[funcMember.Name]; ok {
		// 是需要追踪的函数
		zeroLineStmts = append(zeroLineStmts, i.getInputStmt(funcMember)...)
		if funcType.Results != nil {
			i.wrapperReturnStmt(bodyStmt, funcType)
		}
	}

	if varNo := 0; i.detectGoStmt(bodyStmt, &varNo) {
		// 开启了新协程, 需要获取goid
		zeroLineStmts = append(i.getParentIdStmt(), zeroLineStmts...)
		i.rewriteMap[funcMember.File].needGoId = true
	}

	i.insertStmt(bodyStmt, zeroLineStmts, new(int))
}

func (i *InsPara) insertStmt(body ast.Stmt, stmts []ast.Stmt, index *int) {
	switch bo := body.(type) {
	case *ast.BlockStmt:
		list := append(stmts, bo.List[*index:]...)
		bo.List = append(bo.List[:*index], list...)
	case *ast.CommClause:
		list := append(stmts, bo.Body[*index:]...)
		bo.Body = append(bo.Body[:*index], list...)
	case *ast.CaseClause:
		list := append(stmts, bo.Body[*index:]...)
		bo.Body = append(bo.Body[:*index], list...)
	}
	(*index) += len(stmts)
}

// 包装 return 语句
func (i *InsPara) wrapperReturnStmt(bs *ast.BlockStmt, ft *ast.FuncType) {
	if len(ft.Results.List) == 0 {
		// 没有返回参数
		return
	}

	if len(ft.Results.List[0].Names) > 0 {
		args := []ast.Expr{}
		for _, l := range ft.Results.List {
			for _, name := range l.Names {
				args = append(args, &ast.Ident{
					Name: name.Name,
				})
			}
		}
		var deferStmt *ast.DeferStmt = &ast.DeferStmt{
			Call: &ast.CallExpr{
				Fun: &ast.FuncLit{
					Type: &ast.FuncType{
						Params:  &ast.FieldList{},
						Results: &ast.FieldList{},
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ExprStmt{
								X: &ast.CallExpr{
									Fun: &ast.SelectorExpr{
										X: &ast.Ident{
											Name: PackageName,
										},
										Sel: &ast.Ident{
											Name: "ReportM",
										},
									},
									Args: args,
								},
							},
						},
					},
				},
			},
		}

		bs.List = append([]ast.Stmt{deferStmt}, bs.List...)
		return
	}

	ast.Inspect(bs, func(n ast.Node) bool {
		rStmt, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}

		args := []ast.Expr{}
		retArgs := []ast.Expr{}

		for rIndex := 0; rIndex < ft.Results.NumFields(); rIndex++ {
			retArg := &ast.Ident{
				Name: fmt.Sprintf("_ret_arg_%d", rIndex),
			}

			args = append(args, retArg)
			retArgs = append(retArgs, retArg)
		}

		delete := []int{}
		for rIndex, expr := range rStmt.Results {
			if ident, ok := expr.(*ast.Ident); ok && ident.Name == "nil" {
				retArgs[rIndex].(*ast.Ident).Name = "nil"
				delete = append([]int{rIndex}, delete...)
			}
		}

		for _, index := range delete {
			args = append(args[:index], args[index+1:]...)
			rStmt.Results = append(rStmt.Results[:index], rStmt.Results[index+1:]...)
		}

		var insertStmt *ast.ExprStmt = &ast.ExprStmt{
			X: &ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X: &ast.Ident{
						Name: PackageName,
					},
					Sel: &ast.Ident{
						Name: "ReportOutput",
					},
				},
				Args: retArgs,
			},
		}

		rStmt.Results = []ast.Expr{
			&ast.CallExpr{
				Fun: &ast.FuncLit{
					Type: &ast.FuncType{
						Params:  &ast.FieldList{},
						Results: ft.Results,
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.AssignStmt{
								Lhs: args,
								Tok: token.DEFINE,
								Rhs: rStmt.Results,
							},
							insertStmt,
							&ast.ReturnStmt{
								Results: retArgs,
							},
						},
					},
				},
			},
		}
		return false
	})
}

// 检查 go 语句
func (i *InsPara) detectGoStmt(body ast.Stmt, varNo *int) (containGo bool) {
	var stmts *[]ast.Stmt
	switch typ := body.(type) {
	case *ast.BlockStmt:
		stmts = &typ.List
	case *ast.CommClause:
		stmts = &typ.Body
	case *ast.CaseClause:
		stmts = &typ.Body
	default:
		return false
	}

	for index := 0; index < len(*stmts); index++ {
		switch s := (*stmts)[index].(type) {
		case *ast.GoStmt:
			containGo = true
			i.handleGoStmt(s, body, &index, varNo)
		case *ast.IfStmt:
			containGo = i.detectGoStmt(s.Body, varNo) || containGo
		case *ast.ForStmt:
			containGo = i.detectGoStmt(s.Body, varNo) || containGo
		case *ast.RangeStmt:
			containGo = i.detectGoStmt(s.Body, varNo) || containGo
		case *ast.SelectStmt:
			containGo = i.detectGoStmt(s.Body, varNo) || containGo
		case *ast.SwitchStmt:
			containGo = i.detectGoStmt(s.Body, varNo) || containGo
		case *ast.TypeSwitchStmt:
			containGo = i.detectGoStmt(s.Body, varNo) || containGo
		case *ast.CommClause:
			if s.Body == nil {
				continue
			}
			containGo = i.detectGoStmt(s, varNo) || containGo
		case *ast.CaseClause:
			if s.Body == nil {
				continue
			}
			containGo = i.detectGoStmt(s, varNo) || containGo
		}
	}
	return
}

func (i *InsPara) handleGoStmt(s *ast.GoStmt, bodyStmt ast.Stmt, index, varNo *int) {
	i.insertIncreaseStmt(bodyStmt, index)

	if funclit, ok := s.Call.Fun.(*ast.FuncLit); ok {
		i.insertRegistStmt(funclit.Body)
	} else {
		args := []ast.Expr{}
		if s.Call.Args != nil && len(s.Call.Args) > 0 {
			for range s.Call.Args {
				args = append(args, &ast.Ident{
					Name: fmt.Sprintf("_%d", *varNo),
				})
				(*varNo)++
			}
			assignStmt := &ast.AssignStmt{
				Lhs: args,
				Rhs: s.Call.Args,
				Tok: token.DEFINE,
			}
			i.nodeInspected[assignStmt] = struct{}{}
			i.insertStmt(bodyStmt, []ast.Stmt{assignStmt}, index)
		}
		i.insertWraperStmt(s, args)
	}
}

// waitgroup 自增
func (i *InsPara) insertIncreaseStmt(bodyStmt ast.Stmt, index *int) {
	var insertStmt *ast.ExprStmt = &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X: &ast.Ident{
					Name: PackageName,
				},
				Sel: &ast.Ident{
					Name: "IncreaseWG",
				},
			},
		},
	}

	i.insertStmt(bodyStmt, []ast.Stmt{insertStmt}, index)
}

// 注册子 goroutine 语句
func (i *InsPara) insertRegistStmt(body ast.Stmt) {
	var insertStmt *ast.ExprStmt = &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X: &ast.Ident{
					Name: PackageName,
				},
				Sel: &ast.Ident{
					Name: "RegisterChildrenId",
				},
			},
			Args: []ast.Expr{
				&ast.Ident{
					Name: ParentId,
				},
			},
		},
	}

	var deferStmt *ast.DeferStmt = &ast.DeferStmt{
		Call: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X: &ast.Ident{
					Name: PackageName,
				},
				Sel: &ast.Ident{
					Name: "CloseGoRoutine",
				},
			},
		},
	}

	i.insertStmt(body, []ast.Stmt{deferStmt, insertStmt}, new(int))
}

// 为 go 语句增加 wrapper 函数
func (i *InsPara) insertWraperStmt(stmt *ast.GoStmt, args []ast.Expr) {
	stmt.Call.Args = args // 改变原语句参数
	stmt.Call = &ast.CallExpr{
		Fun: &ast.FuncLit{
			Type: &ast.FuncType{
				Params:  &ast.FieldList{},
				Results: &ast.FieldList{},
			},
			Body: &ast.BlockStmt{
				List: []ast.Stmt{
					&ast.ExprStmt{
						X: stmt.Call,
					},
				},
			},
		},
	}
	i.insertRegistStmt(stmt.Call.Fun.(*ast.FuncLit).Body)
}
