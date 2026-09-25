package localworkspace

import (
	"regexp"
	"strings"
)

// cmdMetaChars 是 cmd.exe 解析命令行时有特殊含义的字符。
var cmdMetaChars = regexp.MustCompile("([()\\][%!^\"`<>&|;, *?])")

// quotedBackslashes 匹配引号前的连续反斜杠。
var quotedBackslashes = regexp.MustCompile(`(\\*)"`)

// trailingBackslashes 匹配结尾的连续反斜杠。
var trailingBackslashes = regexp.MustCompile(`(\\*)$`)

// cmdScriptLine 返回经 cmd.exe /d /s /c 执行脚本的命令行：脚本路径与参数按 C 运行库规则加引号后，再用 ^ 转义 cmd.exe 的特殊字符；
// doubleEscape 为 true 时参数再转义一次，供把参数经 %* 交给 cmd.exe 再解析一次的脚本（如 npm 生成的 .cmd）使用。
func cmdScriptLine(interpreter, script string, args []string, doubleEscape bool) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, cmdMetaChars.ReplaceAllString(script, "^$1"))
	for _, arg := range args {
		arg = quotedBackslashes.ReplaceAllString(arg, `$1$1\"`)
		arg = trailingBackslashes.ReplaceAllString(arg, "$1$1")
		arg = cmdMetaChars.ReplaceAllString(`"`+arg+`"`, "^$1")
		if doubleEscape {
			arg = cmdMetaChars.ReplaceAllString(arg, "^$1")
		}
		parts = append(parts, arg)
	}
	return `"` + interpreter + `" /d /s /c "` + strings.Join(parts, " ") + `"`
}
