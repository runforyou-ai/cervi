/** 在 YYYY-MM-DD 日期键上回退一个日历日，避免按 24 小时计算跨越夏令时。 */
export function previousDayKey(day: string) {
  const date = new Date(`${day}T00:00:00Z`)
  date.setUTCDate(date.getUTCDate() - 1)
  return date.toISOString().slice(0, 10)
}
