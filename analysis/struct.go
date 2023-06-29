package analysis

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// 自动生成函数前缀
const (
	Wrapper     = "wrapper for func"
	BoundMethod = "bound method wrapper for func"
	Initializer = "package initializer"
)

type (
	// 代码成员结构体
	Member struct {
		Name      string                     // 函数名，同 Fun.String()
		Pkg       *ssa.Package               // 包
		File      string                     // 文件名
		Node      ast.Node                   // 包含了整体起始结束位置
		Token     token.Token                // 函数则 String() 为 func
		Type      types.Type                 // 函数则实际类型为 *types.Signature
		Fun       *ssa.Function              // 函数
		Wrapper   map[*ssa.Function]struct{} // 包装器
		AnonFuncs []string                   // 成员内匿名函数名
		// Const Var属性
		Vname     string
		NameNode  ast.Node
		ValueNode ast.Node
	}

	// 源码文件结构体
	File struct {
		FunMember   map[string]*Member
		VarMember   []*Member
		ConstMember []*Member
		TypeMember  []*Member
		File        string
		ParsedFile  *ast.File
		Imports     map[string]string
	}

	// 源码包结构体
	Package struct {
		Fm  map[string]*File
		Pkg *ssa.Package
	}

	// 项目结构体
	Project struct {
		Pm         map[string]*Package
		SsaProgram *ssa.Program
		SsaPkgs    []*ssa.Package
		RootPkg    string

		wrappers []*ssa.Function
	}
)

// 处理非函数成员，import, var, const, type
func (p *Project) CheckOtherMember() {
	p.linkToWrapper()

	for _, pkg := range p.Pm {
		for _, fs := range pkg.Fm {

			for _, decl := range fs.ParsedFile.Decls {
				if gd, ok := decl.(*ast.GenDecl); ok {

					switch gd.Tok {
					case token.TYPE:
						for _, sp := range gd.Specs {
							fs.addType(sp, pkg.Pkg)
						}
					case token.CONST:
						fs.addConst(gd.Specs, pkg.Pkg)
					case token.VAR:
						fs.addVar(gd.Specs, pkg.Pkg)
					case token.IMPORT:
						fs.addImport(p.RootPkg, gd.Specs)
					}
				}
			}
		}
	}
}

// 将 wrapper 链接到真实代码中的函数
func (p *Project) linkToWrapper() {
	for _, pkg := range p.Pm {
		for _, f := range pkg.Fm {

			for _, m := range f.FunMember {

				for _, w := range p.wrappers {
					if strings.Contains(w.Synthetic, m.Fun.String()) {
						m.Wrapper[w] = struct{}{}
					}
				}

			}
		}
	}
}

// 添加 var 变量
func (f *File) addVar(specs []ast.Spec, pkg *ssa.Package) {
	for _, spec := range specs {
		if t, ok := spec.(*ast.ValueSpec); ok {
			for i := 0; i < len(t.Names); i++ {
				tName := t.Names[i]
				var tValue ast.Node
				if t.Values != nil {
					tValue = t.Values[i]
				} else {
					tValue = nil
				}

				var typ types.Type
				if mem, ok := pkg.Members[tName.Name]; ok && mem.Token() == token.VAR {
					typ = mem.Type()
					if strings.HasPrefix(typ.Underlying().String(), "*func(") {
						funcMember := f.FindFuncMember(tValue, pkg)
						if funcMember != nil {
							funcMember.Vname = tName.Name
						}
						continue
					}
				}

				f.VarMember = append(f.VarMember, &Member{
					Name:      pkg.Pkg.Path() + "." + tName.Name,
					Pkg:       pkg,
					File:      f.File,
					Node:      t,
					Token:     token.TYPE,
					Type:      typ,
					Fun:       nil,
					Wrapper:   nil,
					AnonFuncs: nil,
					NameNode:  tName,
					ValueNode: tValue,
					Vname:     tName.Name,
				})
			}
		}
	}
}

// 添加常量
func (f *File) addConst(specs []ast.Spec, pkg *ssa.Package) {
	for _, spec := range specs {
		if t, ok := spec.(*ast.ValueSpec); ok {
			for i := 0; i < len(t.Names); i++ {
				tName := t.Names[i]
				var tValue ast.Node
				if t.Values != nil {
					tValue = t.Values[i]
				} else {
					tValue = nil
				}

				var typ types.Type
				if mem, ok := pkg.Members[tName.Name]; ok && mem.Token() == token.CONST {
					typ = mem.Type()
				}

				f.ConstMember = append(f.ConstMember, &Member{
					Name:      pkg.Pkg.Path() + "." + tName.Name,
					Pkg:       pkg,
					File:      f.File,
					Node:      t,
					Token:     token.TYPE,
					Type:      typ,
					Fun:       nil,
					Wrapper:   nil,
					AnonFuncs: nil,
					NameNode:  tName,
					ValueNode: tValue,
					Vname:     tName.Name,
				})
			}
		}
	}
}

// 增加类型
func (f *File) addType(spec ast.Spec, pkg *ssa.Package) {
	if t, ok := spec.(*ast.TypeSpec); ok {

		var typ types.Type
		if mem, ok := pkg.Members[t.Name.Name]; ok && mem.Token() == token.TYPE {
			typ = mem.Type()
		}

		f.TypeMember = append(f.TypeMember, &Member{
			Name:      pkg.Pkg.Path() + "." + t.Name.Name,
			Pkg:       pkg,
			File:      f.File,
			Node:      spec,
			Token:     token.TYPE,
			Type:      typ,
			Fun:       nil,
			Wrapper:   nil,
			AnonFuncs: nil,
			NameNode:  nil,
			ValueNode: nil,
		})
	}
}

// 增加导入
func (f *File) addImport(root string, specs []ast.Spec) {
	for _, spec := range specs {
		if importSpec, ok := spec.(*ast.ImportSpec); ok {
			path := strings.Trim(importSpec.Path.Value, "\"")
			name := path
			if importSpec.Name != nil {
				name = importSpec.Name.Name
			} else if strings.LastIndex(path, "/") >= 0 {
				name = path[strings.LastIndex(path, "/")+1:]
			}
			if strings.HasPrefix(path, root) {
				f.Imports[name] = path
			}
		}
	}
}

// 解析项目中函数
func (p *Project) Add(fun *ssa.Function) {
	if strings.HasPrefix(fun.Synthetic, Wrapper) || strings.HasPrefix(fun.Synthetic, BoundMethod) {
		p.wrappers = append(p.wrappers, fun)
		return
	}
	if fun.Synthetic == Initializer {
		// 初始化
		return
	}
	pkg := p.initPackage(fun.Pkg)
	pkg.initFuncMember(fun)
}

// 初始化包
func (p *Project) initPackage(pkg *ssa.Package) *Package {
	if _, ok := p.Pm[pkg.Pkg.Path()]; !ok {
		p.Pm[pkg.Pkg.Path()] = &Package{
			Fm:  map[string]*File{},
			Pkg: pkg,
		}
	}

	return p.Pm[pkg.Pkg.Path()]
}

// 初始化函数成员
func (p *Package) initFuncMember(fun *ssa.Function) {

	file := fun.Prog.Fset.Position(fun.Pos()).Filename
	f := p.initFile(file)
	if _, ok := f.FunMember[fun.String()]; !ok {
		anon := make([]string, len(fun.AnonFuncs))
		for _, f := range fun.AnonFuncs {
			anon = append(anon, f.String())
		}

		f.FunMember[fun.String()] = &Member{
			Name:      fun.String(),
			Pkg:       fun.Pkg,
			File:      f.File,
			Node:      fun.Syntax(),
			Token:     fun.Token(),
			Type:      fun.Type(),
			Fun:       fun,
			Wrapper:   make(map[*ssa.Function]struct{}),
			AnonFuncs: anon,
			NameNode:  nil,
			ValueNode: nil,
		}
	}
}

// 初始化文件结构体
func (p *Package) initFile(file string) *File {

	if _, ok := p.Fm[file]; !ok {
		f, _ := parser.ParseFile(p.Pkg.Prog.Fset, file, nil, parser.ParseComments)

		p.Fm[file] = &File{
			File:        file,
			FunMember:   make(map[string]*Member),
			VarMember:   []*Member{},
			ConstMember: []*Member{},
			TypeMember:  []*Member{},
			ParsedFile:  f,
			Imports:     make(map[string]string),
		}
	}

	return p.Fm[file]
}

// 工具类函数
// 寻找函数成员
func (p *Project) FindFuncMember(fun *ssa.Function) (*Member, bool) {
	if fun.Pkg == nil {
		return nil, false
	}
	if pkg, ok := p.Pm[fun.Pkg.Pkg.Path()]; ok {
		f := fun.Prog.Fset.Position(fun.Pos()).Filename
		if file, ok := pkg.Fm[f]; ok {
			for _, mem := range file.FunMember {
				if mem.Fun == fun {
					return mem, false
				}
				if _, ok := mem.Wrapper[fun]; ok {
					return mem, true
				}
			}
		}
	}

	return nil, false
}

// 从文件中寻找成员函数
func (f *File) FindFuncMember(n ast.Node, pkg *ssa.Package) *Member {
	pos := pkg.Prog.Fset.Position(n.Pos())
	end := pkg.Prog.Fset.Position(n.End())
	for _, m := range f.FunMember {
		pos1 := pkg.Prog.Fset.Position(m.Node.Pos())
		end1 := pkg.Prog.Fset.Position(m.Node.End())
		if pos.Line == pos1.Line && pos.Column == pos1.Column &&
			end.Line == end1.Line && end.Column == end1.Column {
			return m
		}
	}
	return nil
}

func (m *Member) ComparePostion(pos token.Position, end token.Position) bool {
	fset := m.Pkg.Prog.Fset
	return fset.Position(m.Node.Pos()).Line == pos.Line &&
		fset.Position(m.Node.Pos()).Column == pos.Column &&
		fset.Position(m.Node.End()).Line == end.Line &&
		fset.Position(m.Node.End()).Column == end.Column
}
