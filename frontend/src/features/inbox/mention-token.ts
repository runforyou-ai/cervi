/** 提供消息正文中结构化 @ 标记的统一匹配规则。 */

/** 生成一个或多个成员姓名对应的完整 @ 标记正则片段。 */
export function mentionTokenPattern(displayNames: string[]) {
  const alternatives = displayNames
    .map((name) => name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))
    .join("|")
  return `(?<!\\S)@(?:${alternatives})(?![\\p{L}\\p{N}_])`
}
