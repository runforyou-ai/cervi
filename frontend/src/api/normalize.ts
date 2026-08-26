/** 生成类型的边界归一化公共工具。 */

/** 把可空切片转换为空数组。 */
export function asList<T>(value: T[] | null | undefined): T[] {
  return value ?? []
}
