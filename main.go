package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

const Usage = `go-chain: 代码链路生成.

Usage:

  go-chain [flags] 
  默认分析当前目录下的所有package: ./..
  不支持更改，支持指定other package
Flags:
`

var (
	otherPackage = flag.String("otherPackage", "", "添加其他package,逗号分割，例如sdk:git.garena.com/shopee/experiment-platform/abtest-core/...")
	outputFile   = flag.String("file", "output", "输出文件名字")
	outputFormat = flag.String("format", "svg", "输出文件格式 [svg | png | jpg | ...]")
	diff         = flag.Bool("diff", false, "和master对比，只输出变动涉及到的func相关链路.")
	focusFunc    = flag.String("f", "", "给定函数名字分析，逗号分割。和diff参数互斥")
	ignoreFlag   = flag.String("ignore", "", "忽略 package paths 前缀或者后缀字符串,逗号分割")
	debugFlag    = flag.Bool("debug", false, "debug日志.")
	versionFlag  = flag.Bool("version", false, "Show version and exit.")
)

func logf(f string, a ...interface{}) {
	if *debugFlag {
		log.Printf(f, a...)
	}
}

func main() {
	flag.Parse()
	if flag.NFlag() == 0 {
		fmt.Fprint(os.Stderr, Usage)
		flag.PrintDefaults()
		os.Exit(2)
	}
	if *versionFlag {
		fmt.Fprintln(os.Stderr, Version())
		os.Exit(0)
	}
	logf("diff:", *diff)
	logf("focusFunc:", *focusFunc)
	if *diff && *focusFunc != "" {
		fmt.Fprintln(os.Stderr, "diff 和指定函数互斥,请指定一个参数")
		os.Exit(0)
	}
	if *debugFlag {
		log.SetFlags(log.Lmicroseconds)
	}
	mainStart := time.Now()
	//os.Chdir("/Users/jiajia.huo/go/abtest-traffic-service")
	analysis := new(Analysis)
	var analysisPackages []string
	analysisPackages = append(analysisPackages, "./...") //分析当前目录的所有package
	if *otherPackage != "" {
		for _, p := range strings.Split(*otherPackage, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				analysisPackages = append(analysisPackages, p)
			}
		}
	}
	analysis.ProcessListArgs()
	if err := analysis.DoAnalysis(analysisPackages); err != nil {
		log.Fatal(err)
	}

	dot, dotNodes, err := ProduceDotData(analysis)
	if err != nil {
		log.Fatal(err)
	}
	OutputFile(dot, dotNodes, analysis.focusFuncs)
	end := time.Since(mainStart)
	fmt.Println("耗时:", end)
}
