package localworkspace

import "testing"

// TestCmdScriptLine 验证脚本参数加引号并转义 cmd.exe 特殊字符，npm 生成的 .cmd 再转义一次。
func TestCmdScriptLine(t *testing.T) {
	line := cmdScriptLine(`C:\Windows\system32\cmd.exe`, `C:\Program Files\npx.cmd`, []string{"-y", `C:\R&D`, `say "hi"`, `dir\`}, false)
	expected := `"C:\Windows\system32\cmd.exe" /d /s /c "C:\Program^ Files\npx.cmd ^"-y^" ^"C:\R^&D^" ^"say^ \^"hi\^"^" ^"dir\\^""`
	if line != expected {
		t.Fatalf("命令行不符合预期:\n%s\n%s", line, expected)
	}
	double := cmdScriptLine("cmd.exe", "npx.cmd", []string{`C:\R&D`}, true)
	if expected := `"cmd.exe" /d /s /c "npx.cmd ^^^"C:\R^^^&D^^^""`; double != expected {
		t.Fatalf("二次转义不符合预期:\n%s\n%s", double, expected)
	}
}
