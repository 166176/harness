// Package feedback 解析并分类测试运行器（go test / pytest）的输出，
// 将其转换为结构化的 TestFailure 记录，供 agent 反馈闭环使用。
package feedback

import (
	"regexp"
	"strconv"
	"strings"
)

// Kind 表示测试失败的类别。
type Kind string

const (
	KindCompile Kind = "compile"
	KindAssert  Kind = "assert"
	KindTimeout Kind = "timeout"
	KindEnv     Kind = "env"
	KindUnknown Kind = "unknown"
)

// TestFailure 是测试输出中一条结构化的失败记录。
type TestFailure struct {
	File    string
	Line    int
	Message string
	Kind    Kind
}

var (
	goTestLineRe = regexp.MustCompile(`^\s*([\w./-]+_test\.go):(\d+):\s*(.*)$`)
	pyLineRe     = regexp.MustCompile(`([\w./-]+\.py):(\d+):`)
	assertRe     = regexp.MustCompile(`AssertionError|assert|want `)
	compileRe    = regexp.MustCompile(`SyntaxError|NameError|ImportError`)
	timeoutRe    = regexp.MustCompile(`Timeout|timed out`)
	envRe        = regexp.MustCompile(`ModuleNotFoundError|No module named`)
)

// Parse 解析测试输出；format ∈ {"gotest", "pytest"}。
// 未识别的 format 返回 nil（等价于空列表）。
func Parse(format, out string, exitCode int) []TestFailure {
	switch format {
	case "gotest":
		return parseGoTest(out, exitCode)
	case "pytest":
		return parsePytest(out)
	default:
		return nil
	}
}

// parseGoTest 逐行扫描 go test 输出：
// `--- FAIL: <name>` 之后开始收集 `file_test.go:line: message` 行；
// 退出码非 0 且没有解析到任何结构化失败时，产生单个 KindUnknown
// 失败（Message 为输出尾部 500 字符）。
func parseGoTest(out string, exitCode int) []TestFailure {
	var failures []TestFailure
	collecting := false
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "--- FAIL:") {
			collecting = true
			continue
		}
		if !collecting {
			continue
		}
		if m := goTestLineRe.FindStringSubmatch(line); m != nil {
			msg := strings.TrimSpace(m[3])
			failures = append(failures, TestFailure{
				File:    m[1],
				Line:    atoi(m[2]),
				Message: msg,
				Kind:    classify(msg),
			})
		}
	}
	if exitCode != 0 && len(failures) == 0 {
		failures = append(failures, TestFailure{
			Message: tail(out, 500),
			Kind:    KindUnknown,
		})
	}
	return failures
}

// parsePytest 扫描 pytest 输出：`file.py:line:` 行构成失败记录，
// 紧邻其前的 `E   ...` 行并入该记录的 Message；
// 若输出只有 `E   ...` 行而无定位行，则合并为单条失败。
func parsePytest(out string) []TestFailure {
	var failures []TestFailure
	var pending []string
	for _, line := range strings.Split(out, "\n") {
		if m := pyLineRe.FindStringSubmatch(line); m != nil {
			msg := strings.TrimSpace(line[len(m[0]):])
			if len(pending) > 0 {
				msg = strings.TrimSpace(strings.Join(pending, "\n") + "\n" + msg)
				pending = nil
			}
			failures = append(failures, TestFailure{
				File:    m[1],
				Line:    atoi(m[2]),
				Message: msg,
				Kind:    classify(msg),
			})
			continue
		}
		if strings.HasPrefix(line, "E ") {
			pending = append(pending, line)
		}
	}
	if len(failures) == 0 && len(pending) > 0 {
		msg := strings.Join(pending, "\n")
		failures = append(failures, TestFailure{Message: msg, Kind: classify(msg)})
	}
	return failures
}

// classify 按失败信息归类：
// assert（含 go test 的 "want X" 惯用语）、compile、timeout、env，其余 unknown。
func classify(message string) Kind {
	switch {
	case assertRe.MatchString(message):
		return KindAssert
	case compileRe.MatchString(message):
		return KindCompile
	case timeoutRe.MatchString(message):
		return KindTimeout
	case envRe.MatchString(message):
		return KindEnv
	default:
		return KindUnknown
	}
}

func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
