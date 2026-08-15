import * as React from "react"

const MOBILE_BREAKPOINT = 768
const MOBILE_QUERY = `(max-width: ${MOBILE_BREAKPOINT - 1}px)`

export function useIsMobile() {
  const [isMobile, setIsMobile] = React.useState(
    () => window.matchMedia(MOBILE_QUERY).matches
  )

  React.useEffect(() => {
    const mediaQuery = window.matchMedia(MOBILE_QUERY)
    const updateIsMobile = (event: MediaQueryListEvent) =>
      setIsMobile(event.matches)

    mediaQuery.addEventListener("change", updateIsMobile)
    return () => mediaQuery.removeEventListener("change", updateIsMobile)
  }, [])

  return isMobile
}
