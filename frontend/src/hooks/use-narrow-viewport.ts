/** 判断当前是否为窄视口。 */
import * as React from "react"

const NARROW_VIEWPORT_BREAKPOINT = 768
const NARROW_VIEWPORT_QUERY = `(max-width: ${NARROW_VIEWPORT_BREAKPOINT - 1}px)`

/** 监听窄视口断点是否匹配。 */
export function useIsNarrowViewport() {
  const [isNarrowViewport, setIsNarrowViewport] = React.useState(
    () => window.matchMedia(NARROW_VIEWPORT_QUERY).matches
  )

  React.useEffect(() => {
    const mediaQuery = window.matchMedia(NARROW_VIEWPORT_QUERY)
    const updateViewportMatch = (event: MediaQueryListEvent) =>
      setIsNarrowViewport(event.matches)

    mediaQuery.addEventListener("change", updateViewportMatch)
    return () => mediaQuery.removeEventListener("change", updateViewportMatch)
  }, [])

  return isNarrowViewport
}
