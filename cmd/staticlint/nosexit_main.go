package main

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

// NoOsExitInMainAnalyzer запрещает прямой вызов os.Exit в функции main пакета main.
var NoOsExitInMainAnalyzer = &analysis.Analyzer{
	Name:     "noosexitinmain",
	Doc:      "reports direct calls to os.Exit in main.main of package main",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      runNoOsExitInMain,
}

func runNoOsExitInMain(pass *analysis.Pass) (any, error) {
	if pass.Pkg == nil || pass.Pkg.Name() != "main" {
		return nil, nil
	}

	for _, f := range pass.Files {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name == nil || fn.Name.Name != "main" || fn.Body == nil {
				continue
			}

			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel == nil {
					return true
				}

				obj := pass.TypesInfo.Uses[sel.Sel]
				fun, ok := obj.(*types.Func)
				if !ok || fun.Pkg() == nil {
					return true
				}

				if fun.Pkg().Path() == "os" && fun.Name() == "Exit" {
					pass.Reportf(call.Pos(), "запрещён прямой вызов os.Exit в main.main; вынесите завершение в отдельную функцию")
				}
				return true
			})
		}
	}

	return nil, nil
}
