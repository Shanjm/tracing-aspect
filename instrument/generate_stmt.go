package instrument

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/Shanjm/tracing-aspect/analysis"
)

func (i *InsPara) getStartStmt() []ast.Stmt {
	var callStmt *ast.ExprStmt = &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X: &ast.Ident{
					Name: PackageName,
				},
				Sel: &ast.Ident{
					Name: "StartMultiMode",
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
					Name: "StopMultiMode",
				},
			},
		},
	}

	return []ast.Stmt{deferStmt, callStmt}
}

// TODO
func (i *InsPara) getCopyStmt(funcMember *analysis.Member) []ast.Stmt {
	p := funcMember.Fun.Params
	var reassignStmt *ast.AssignStmt = &ast.AssignStmt{
		Tok: token.ASSIGN,
		Lhs: []ast.Expr{
			&ast.Ident{
				Name: p[len(p)-2].Name(),
			},
		},
		Rhs: []ast.Expr{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X: &ast.Ident{
						Name: PackageName,
					},
					Sel: &ast.Ident{
						Name: "NewRecorder",
					},
				},
				Args: []ast.Expr{
					&ast.Ident{
						Name: p[len(p)-2].Name(),
					},
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
					Name: "DumpOriHttp",
				},
			},
			Args: []ast.Expr{
				&ast.Ident{
					Name: p[len(p)-1].Name(),
				},
				&ast.Ident{
					Name: p[len(p)-2].Name(),
				},
			},
		},
	}
	return []ast.Stmt{reassignStmt, deferStmt}
}

func (i *InsPara) getParentIdStmt() []ast.Stmt {
	var getIdStmt *ast.AssignStmt = &ast.AssignStmt{
		Tok: token.DEFINE,
		Lhs: []ast.Expr{
			&ast.Ident{
				Name: ParentId,
			},
		},
		Rhs: []ast.Expr{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X: &ast.Ident{
						Name: "goid",
					},
					Sel: &ast.Ident{
						Name: "Get",
					},
				},
			},
		},
	}

	return []ast.Stmt{getIdStmt}
}

func (i *InsPara) getInputStmt(funcMember *analysis.Member) []ast.Stmt {
	// 构造参数
	args := []ast.Expr{
		&ast.BasicLit{
			Value: fmt.Sprintf("\"函数名：%s\"", funcMember.Name),
		},
	}
	for _, para := range funcMember.Fun.Params {
		args = append(args, &ast.Ident{
			Name: para.Name(),
		})
	}

	var insertStmt *ast.ExprStmt = &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X: &ast.Ident{
					Name: PackageName,
				},
				Sel: &ast.Ident{
					Name: "ReportInput",
				},
			},
			Args: args,
		},
	}

	return []ast.Stmt{insertStmt}
}
