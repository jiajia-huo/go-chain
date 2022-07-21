package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

func CodeDiffWithMaster() (map[string]bool, error) {
	c := fmt.Sprintf("git   diff   -U0 origin/master...HEAD")
	cmd := exec.Command("bash", "-c", c)
	var stdin, stdout, stderr bytes.Buffer
	cmd.Stdin = &stdin
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	outStr, errStr := string(stdout.Bytes()), string(stderr.Bytes())
	if errStr != "" {
		return nil, fmt.Errorf(errStr)
	}
	if err != nil {
		return nil, err
	}
	//fmt.Println(outStr)
	reg := regexp.MustCompile(`func (?s:(.*?))\(`)
	if reg == nil {
		return nil, fmt.Errorf("MustCompile err")
	}
	//提取关键信息
	result := reg.FindAllStringSubmatch(outStr, -1)
	var funcMap = make(map[string]bool)
	for _, f := range result {
		if f[1] != "" {
			key := strings.Trim(f[1], "")
			funcMap[key] = true
		}
	}
	return funcMap, nil
}
