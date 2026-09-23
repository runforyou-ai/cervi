package agentruntime

import "slices"

// LocalRuntimeVersion 是本端本机运行时的版本，本机工具的实现或参数语义变化时加一。
const LocalRuntimeVersion = 1

// LocalToolRisk 表示本机工具对主人电脑的风险类别。
type LocalToolRisk string

const (
	// LocalToolRiskReadOnly 表示只读取工作区内容。
	LocalToolRiskReadOnly LocalToolRisk = "read_only"
	// LocalToolRiskModifiesLocal 表示改动工作区内容。
	LocalToolRiskModifiesLocal LocalToolRisk = "modifies_local"
	// LocalToolRiskExecutesCode 表示在主人电脑上执行任意代码。
	LocalToolRiskExecutesCode LocalToolRisk = "executes_code"
)

// localTool 描述一个本机工具的名称、风险类别与所需的最低设备运行时版本。
type localTool struct {
	Name              string
	Risk              LocalToolRisk
	MinRuntimeVersion int
}

// localTools 是本机工具目录，按注册顺序排列。
var localTools = []localTool{
	{Name: "ls", Risk: LocalToolRiskReadOnly, MinRuntimeVersion: 1},
	{Name: "read_file", Risk: LocalToolRiskReadOnly, MinRuntimeVersion: 1},
	{Name: "glob", Risk: LocalToolRiskReadOnly, MinRuntimeVersion: 1},
	{Name: "grep", Risk: LocalToolRiskReadOnly, MinRuntimeVersion: 1},
}

// LocalToolManifest 返回本端运行时实现的本机工具名称，设备注册时上报。
func LocalToolManifest() []string {
	names := make([]string, 0, len(localTools))
	for _, item := range localTools {
		names = append(names, item.Name)
	}
	return names
}

// AvailableLocalTools 按本机工具目录顺序返回设备上报清单与目录的交集，只保留设备运行时版本满足要求且无需审批即可执行的工具。
func AvailableLocalTools(manifest []string, runtimeVersion int) []string {
	names := make([]string, 0, len(localTools))
	for _, item := range localTools {
		if item.MinRuntimeVersion <= runtimeVersion && slices.Contains(manifest, item.Name) && localToolAutoAllowed(item.Name) {
			names = append(names, item.Name)
		}
	}
	return names
}

// IsLocalTool 判断工具名称是否属于本机工具目录。
func IsLocalTool(name string) bool {
	return slices.ContainsFunc(localTools, func(item localTool) bool { return item.Name == name })
}

// localToolAutoAllowed 判断本机工具是否无需审批即可执行；只有声明为只读的工具自动放行，未登记的工具一律需要审批。
func localToolAutoAllowed(name string) bool {
	index := slices.IndexFunc(localTools, func(item localTool) bool { return item.Name == name })
	return index >= 0 && localTools[index].Risk == LocalToolRiskReadOnly
}
