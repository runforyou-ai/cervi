/** 工作区地址：工作区页面位于 /w/<工作区标识> 之下，账号级页面位于根路径，路由器按当前工作区设置 basename。 */

const workspacePrefixPattern = /^\/w\/([a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)(?=\/|\?|$)/
const lastWorkspaceStorageKey = "cervi.lastWorkspace"

/** 从 hash 地址中读取工作区标识，不在工作区地址下时返回 null。 */
export function workspaceSlugFromHash(hash: string) {
  const path = hash.replace(/^#/, "") || "/"
  return workspacePrefixPattern.exec(path)?.[1] ?? null
}

/** 返回工作区页面的地址前缀。 */
export function workspaceBasename(slug: string) {
  return `/w/${slug}`
}

/** 返回工作区内指定页面的完整地址，path 为工作区内的相对根路径。 */
export function workspaceHref(slug: string, path = "/inbox") {
  return `${workspaceBasename(slug)}${path === "/" ? "" : path}`
}

/** 通过改写 hash 跳转到根路径下的完整地址，用于离开当前工作区或进入其他工作区。 */
export function navigateToHashPath(path: string, options: { replace?: boolean } = {}) {
  if (options.replace) {
    window.location.replace(`#${path}`)
    return
  }
  window.location.hash = path
}

/** 进入指定工作区的页面。 */
export function enterWorkspace(slug: string, path = "/inbox", options: { replace?: boolean } = {}) {
  navigateToHashPath(workspaceHref(slug, path), options)
}

/** 返回以当前工作区页面为返回地址的账号级页面地址；当前不在工作区内时不带返回地址。 */
export function withReturnTo(path: string) {
  const current = window.location.hash.replace(/^#/, "")
  if (!workspaceSlugFromHash(current)) return path
  const separator = path.includes("?") ? "&" : "?"
  return `${path}${separator}returnTo=${encodeURIComponent(current)}`
}

/** 读取账号级页面的返回地址，只接受工作区内的地址。 */
export function returnToPath(searchParams: URLSearchParams) {
  const value = searchParams.get("returnTo")
  return value && workspaceSlugFromHash(value) ? value : null
}

/** 读取本机最近进入的工作区标识。 */
export function lastWorkspaceSlug() {
  try {
    return window.localStorage.getItem(lastWorkspaceStorageKey)
  } catch {
    return null
  }
}

/** 记住本机最近进入的工作区标识，下次登录后优先进入。 */
export function rememberWorkspaceSlug(slug: string) {
  try {
    window.localStorage.setItem(lastWorkspaceStorageKey, slug)
  } catch {
    // 本地存储不可用时只影响下次默认进入的工作区。
  }
}
