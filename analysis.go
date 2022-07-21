package main

import (
	"fmt"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
	"strings"
	"time"
)

type Analysis struct {
	prog              *ssa.Program
	pkgs              []*ssa.Package
	mainPkg           *ssa.Package
	callgraph         *callgraph.Graph
	filterPackagePath []string
	ignorePaths       []string
	focusFuncs        []string
}

func (a *Analysis) DoAnalysis(analysisPkgs []string) error {
	cfg := &packages.Config{
		Mode: packages.LoadAllSyntax,
	}

	initial, err := packages.Load(cfg, analysisPkgs...)
	if err != nil {
		return err
	}

	if packages.PrintErrors(initial) > 0 {
		return fmt.Errorf("packages contain errors")
	}

	// Create and build SSA-form program representation.
	prog, pkgs := ssautil.AllPackages(initial, 0)
	prog.Build()

	for _, p := range pkgs {
		a.filterPackagePath = append(a.filterPackagePath, p.Pkg.Path())
	}

	var graph *callgraph.Graph
	var mainPkg *ssa.Package

	mains, err := mainPackages(prog.AllPackages())
	if err != nil {
		return err
	}
	mainPkg = mains[0]
	config := &pointer.Config{
		Mains:          mains,
		BuildCallGraph: true,
	}
	ptares, err := pointer.Analyze(config)
	if err != nil {
		return err
	}
	graph = ptares.CallGraph
	//cg.DeleteSyntheticNodes()

	a.prog = prog
	a.pkgs = pkgs
	a.mainPkg = mainPkg
	a.callgraph = graph
	return nil
}

func (a *Analysis) DeleteNotInSourcePackage(sourcePackages []string) {
	for key, n := range a.callgraph.Nodes {
		start := time.Now()
		ok := n.Func.Pkg == nil || n.Func.Pkg.Pkg == nil || !inSourcePkg(n.Func.Pkg.Pkg.Path(), sourcePackages)
		deleteEnd := time.Since(start)
		fmt.Println("删除node耗时：", deleteEnd)
		if ok {
			delete(a.callgraph.Nodes, key)
		}

	}
}

func (a *Analysis) ProcessListArgs() {
	var ignorePaths []string
	var focusFuncs []string
	for _, p := range strings.Split(*ignoreFlag, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			ignorePaths = append(ignorePaths, p)
		}
	}
	for _, f := range strings.Split(*focusFunc, ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			focusFuncs = append(focusFuncs, f)
		}
	}
	a.ignorePaths = ignorePaths
	a.focusFuncs = focusFuncs
}

func mainPackages(pkgs []*ssa.Package) ([]*ssa.Package, error) {
	var mains []*ssa.Package
	for _, p := range pkgs {
		if p != nil && p.Pkg.Name() == "main" && p.Func("main") != nil {
			mains = append(mains, p)
		}
	}
	if len(mains) == 0 {
		return nil, fmt.Errorf("no main packages")
	}
	return mains, nil
}
