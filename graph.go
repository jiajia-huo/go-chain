package main

import (
	"bytes"
	"fmt"
	"go/build"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
	"log"
	"path/filepath"
	"strings"
	"time"
)

func ProduceDotData(a *Analysis) (*dotGraph, map[string]*dotNode, error) {
	var groupType = true
	var groupPkg = true
	cluster := NewDotCluster("focus")
	cluster.Attrs = dotAttrs{
		"bgcolor":   "white",
		"label":     "",
		"labelloc":  "t",
		"labeljust": "c",
		"fontsize":  "18",
	}
	var (
		nodes []*dotNode
		edges []*dotEdge
	)

	nodeMap := make(map[string]*dotNode)
	edgeMap := make(map[string]*dotEdge)
	a.callgraph.DeleteSyntheticNodes()
	//a.DeleteNotInSourcePackage(a.filterPackagePath)
	count := 0
	err := GraphVisitEdges(a.callgraph, a.filterPackagePath, func(edge *callgraph.Edge) error {
		count++

		caller := edge.Caller
		callee := edge.Callee

		posCaller := a.prog.Fset.Position(caller.Func.Pos())
		posCallee := a.prog.Fset.Position(callee.Func.Pos())
		posEdge := a.prog.Fset.Position(edge.Pos())

		// omit synthetic calls
		if isSynthetic(edge) {
			return nil
		}
		if isIgnores(a.ignorePaths, caller) {
			return nil
		}

		var sprintNode = func(node *callgraph.Node, isCaller bool) *dotNode {
			// only once
			key := node.Func.String()
			nodeTooltip := ""

			fileCaller := fmt.Sprintf("%s:%d", filepath.Base(posCaller.Filename), posCaller.Line)
			fileCallee := fmt.Sprintf("%s:%d", filepath.Base(posCallee.Filename), posCallee.Line)

			if isCaller {
				nodeTooltip = fmt.Sprintf("%s | defined in %s", node.Func.String(), fileCaller)
			} else {
				nodeTooltip = fmt.Sprintf("%s | defined in %s", node.Func.String(), fileCallee)
			}

			if n, ok := nodeMap[key]; ok {
				return n
			}

			attrs := make(dotAttrs)

			// node label
			label := node.Func.RelString(node.Func.Pkg.Pkg)

			// func signature
			sign := node.Func.Signature
			if node.Func.Parent() != nil {
				sign = node.Func.Parent().Signature
			}

			// omit type from label
			if groupType && sign.Recv() != nil {
				parts := strings.Split(label, ".")
				label = parts[len(parts)-1]
			}

			pkg, _ := build.Import(node.Func.Pkg.Pkg.Path(), "", 0)
			// set node color
			if pkg.Goroot {
				attrs["fillcolor"] = "#adedad"
			} else {
				attrs["fillcolor"] = "moccasin"
			}

			attrs["label"] = label

			// func styles
			if node.Func.Parent() != nil {
				attrs["style"] = "dotted,filled"
			} else if node.Func.Object() != nil && node.Func.Object().Exported() {
				attrs["penwidth"] = "1.5"
			} else {
				attrs["penwidth"] = "0.5"
			}

			c := cluster

			// group by pkg
			if groupPkg {

				label := node.Func.Pkg.Pkg.Name()
				if pkg.Goroot {
					label = node.Func.Pkg.Pkg.Path()
				} else {
					labelPath := strings.Split(node.Func.Pkg.Pkg.Path(), "/")
					if len(labelPath) > 3 {
						label = strings.Join(labelPath[3:], "/")
					} else {
						label = node.Func.Pkg.Pkg.Path()
					}
				}
				key := node.Func.Pkg.Pkg.Path()
				if _, ok := c.Clusters[key]; !ok {
					c.Clusters[key] = &dotCluster{
						ID:       key,
						Clusters: make(map[string]*dotCluster),
						Attrs: dotAttrs{
							"penwidth":  "0.8",
							"fontsize":  "16",
							"label":     label,
							"style":     "filled",
							"fillcolor": "lightyellow",
							"fontname":  "Tahoma bold",
							"tooltip":   fmt.Sprintf("package: %s", key),
							"rank":      "sink",
						},
					}
					if pkg.Goroot {
						c.Clusters[key].Attrs["fillcolor"] = "#E0FFE1"
					}
				}
				c = c.Clusters[key]
			}

			// group by type
			if groupType && sign.Recv() != nil {
				label := strings.Split(node.Func.RelString(node.Func.Pkg.Pkg), ".")[0]
				key := sign.Recv().Type().String()
				if _, ok := c.Clusters[key]; !ok {
					c.Clusters[key] = &dotCluster{
						ID:       key,
						Clusters: make(map[string]*dotCluster),
						Attrs: dotAttrs{
							"penwidth":  "0.5",
							"fontsize":  "15",
							"fontcolor": "#222222",
							"label":     label,
							"labelloc":  "b",
							"style":     "rounded,filled",
							"fillcolor": "wheat2",
							"tooltip":   fmt.Sprintf("type: %s", key),
						},
					}
					if pkg.Goroot {
						c.Clusters[key].Attrs["fillcolor"] = "#c2e3c2"
					}
				}
				c = c.Clusters[key]
			}

			attrs["tooltip"] = nodeTooltip

			n := &dotNode{
				ID:    node.Func.String(),
				Attrs: attrs,
			}

			if c != nil {
				c.Nodes = append(c.Nodes, n)
			} else {
				nodes = append(nodes, n)
			}

			nodeMap[key] = n
			return n
		}
		callerNode := sprintNode(edge.Caller, true)
		calleeNode := sprintNode(edge.Callee, false)

		// edges
		attrs := make(dotAttrs)

		// dynamic call
		if edge.Site != nil && edge.Site.Common().StaticCallee() == nil {
			attrs["style"] = "dashed"
		}

		// go & defer calls
		switch edge.Site.(type) {
		case *ssa.Go:
			attrs["arrowhead"] = "normalnoneodot"
		case *ssa.Defer:
			attrs["arrowhead"] = "normalnoneodiamond"
		}

		// use position in file where callee is called as tooltip for the edge
		fileEdge := fmt.Sprintf(
			"at %s:%d: calling [%s]",
			filepath.Base(posEdge.Filename),
			posEdge.Line,
			edge.Callee.Func.String(),
		)

		// omit duplicate calls, except for tooltip enhancements
		key := fmt.Sprintf("%s = %s => %s", caller.Func, edge.Description(), callee.Func)
		if _, ok := edgeMap[key]; !ok {
			attrs["tooltip"] = fileEdge
			e := &dotEdge{
				From:  callerNode,
				To:    calleeNode,
				Attrs: attrs,
			}
			edgeMap[key] = e
		} else {
			// make sure, tooltip is created correctly
			if _, okk := edgeMap[key].Attrs["tooltip"]; !okk {
				edgeMap[key].Attrs["tooltip"] = fileEdge
			} else {
				edgeMap[key].Attrs["tooltip"] = fmt.Sprintf(
					"%s\n%s",
					edgeMap[key].Attrs["tooltip"],
					fileEdge,
				)
			}
		}

		return nil
	})

	if err != nil {
		return nil, nil, err
	}
	for _, e := range edgeMap {
		e.From.Attrs["tooltip"] = fmt.Sprintf(
			"%s\n%s",
			e.From.Attrs["tooltip"],
			e.Attrs["tooltip"],
		)
		edges = append(edges, e)
	}

	title := ""
	if a.mainPkg != nil && a.mainPkg.Pkg != nil {
		title = a.mainPkg.Pkg.Path()
	}
	dot := &dotGraph{
		Title:   title,
		Minlen:  2,
		Cluster: cluster,
		Nodes:   nodes,
		Edges:   edges,
		Options: map[string]string{
			"minlen":    fmt.Sprint(2),
			"nodesep":   fmt.Sprint(0.35),
			"nodeshape": fmt.Sprint("box"),
			"nodestyle": fmt.Sprint("filled,rounded"),
			"rankdir":   fmt.Sprint("LR"),
		},
	}
	return dot, nodeMap, nil

}

func OutputFile(dotGraph *dotGraph, dotNodes map[string]*dotNode, focusFuncs []string) {
	if *diff {
		funcMap, err := CodeDiffWithMaster()

		if err != nil {
			log.Fatalf("%v\n", err)
		}
		logf("diff 函数:", funcMap)
		dealDotDataWithNode(dotGraph, dotNodes, funcMap)
	}
	if !*diff && len(focusFuncs) > 0 {
		var funcMap = make(map[string]bool)
		for _, f := range focusFuncs {
			funcMap[f] = true
		}
		dealDotDataWithNode(dotGraph, dotNodes, funcMap)
	}
	var buf bytes.Buffer
	if err := dotGraph.WriteDot(&buf); err != nil {
		log.Fatalf("%v\n", err)
	}
	_, err := dotToImage(*outputFile, *outputFormat, buf.Bytes())
	if err != nil {
		log.Fatalf("%v\n", err)
	}
	return

}

func dealDotDataWithNode(dotGraph *dotGraph, dotNodes map[string]*dotNode, funcMap map[string]bool) {
	var diffNodes []*dotNode
	for f, _ := range funcMap {
		for _, node := range dotNodes {
			if node.Attrs["label"] == f {
				node.Attrs["fillcolor"] = "red"
				diffNodes = append(diffNodes, node)
			}
		}
	}
	//计算diff node上下游链路上的所有node
	var fromNodes, toNodes = diffNodes, diffNodes
	var mFrom = make(map[*dotNode]bool)
	var mTo = make(map[*dotNode]bool)
	for len(fromNodes) > 0 {
		n := fromNodes[0]
		_, ok := mFrom[n]
		if !ok {
			for _, e := range dotGraph.Edges {
				if e.From == n {
					diffNodes = append(diffNodes, e.To)
					fromNodes = append(fromNodes, e.To)
				}
			}
			mFrom[n] = true
		}
		fromNodes = fromNodes[1:]
	}
	for len(toNodes) > 0 {
		n := toNodes[0]
		_, ok := mTo[n]
		if !ok {
			for _, e := range dotGraph.Edges {
				if e.To == n {
					diffNodes = append(diffNodes, e.From)
					toNodes = append(toNodes, e.From)
				}
			}
			mTo[n] = true
		}
		toNodes = toNodes[1:]
	}

	//删除dotGraph Clusters上不在diffnode里的node
	for _, c := range dotGraph.Cluster.Clusters {
		visitCluster(c, diffNodes)
		for _, c1 := range c.Clusters {
			visitCluster(c1, diffNodes)
		}
	}
	visitEdges(dotGraph, diffNodes)

}

func visitCluster(c *dotCluster, diffNodes []*dotNode) {
	var resultNodes []*dotNode
	for _, n := range c.Nodes {
		if isInDiffNodes(n, diffNodes) {
			resultNodes = append(resultNodes, n)
		}
	}
	c.Nodes = resultNodes
}

func visitEdges(dot *dotGraph, diffNodes []*dotNode) {
	var edgeResult []*dotEdge
	for _, e := range dot.Edges {
		if isInDiffNodes(e.From, diffNodes) && isInDiffNodes(e.To, diffNodes) {
			edgeResult = append(edgeResult, e)
		}
	}
	dot.Edges = edgeResult
}

func isInDiffNodes(node *dotNode, dNodes []*dotNode) bool {
	for _, n := range dNodes {
		if n == node {
			return true
		}
	}
	return false
}

func isSynthetic(edge *callgraph.Edge) bool {
	return edge.Caller.Func.Pkg == nil || edge.Callee.Func.Synthetic != ""
}
func isIgnores(ignorePaths []string, node *callgraph.Node) bool {
	pkgPath := node.Func.Pkg.Pkg.Path()
	for _, p := range ignorePaths {
		if strings.HasSuffix(pkgPath, p) || strings.HasPrefix(pkgPath, p) {
			return true
		}
	}
	return false
}
func inSourcePkg(path string, paths []string) bool {
	for _, p := range paths {
		if p == path {
			return true
		}
	}
	return false
}
func inStd(node *callgraph.Node) bool {
	std := time.Now()
	pkg, _ := build.Import(node.Func.Pkg.Pkg.Path(), "", 0)
	end := time.Since(std)
	fmt.Println("std 耗时：", end)
	return pkg.Goroot
}

func GraphVisitEdges(g *callgraph.Graph, filterPackagePath []string, edge func(*callgraph.Edge) error) error {
	seen := make(map[*callgraph.Node]bool)
	var visit func(n *callgraph.Node) error
	visit = func(n *callgraph.Node) error {
		if n.Func.Pkg == nil || n.Func.Pkg.Pkg == nil || !inSourcePkg(n.Func.Pkg.Pkg.Path(), filterPackagePath) {
			return nil
		}
		if !seen[n] {
			seen[n] = true
			for _, e := range n.Out {
				if e.Callee.Func.Pkg == nil || e.Callee.Func.Pkg.Pkg == nil || !inSourcePkg(e.Callee.Func.Pkg.Pkg.Path(), filterPackagePath) {
					continue
				}
				if err := visit(e.Callee); err != nil {
					return err
				}
				if err := edge(e); err != nil {
					return err
				}
			}
		}

		return nil
	}
	for _, n := range g.Nodes {
		if err := visit(n); err != nil {
			return err
		}
	}
	return nil
}
