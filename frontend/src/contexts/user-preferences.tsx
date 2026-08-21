/** 向工作台页面提供当前用户的语言和时区设置。 */
import { createContext, useContext, useEffect, type ReactNode } from "react"

import { Locale, type User } from "@/api"
import { i18n } from "@/i18n"
import { resolveBrowserTimeZone } from "@/lib/time-zones"

type UserPreferences = Pick<User, "locale" | "timeZone">

const UserPreferencesContext = createContext<UserPreferences>({
  locale: Locale.LocaleChineseSimplified,
  timeZone: resolveBrowserTimeZone(),
})

/** 同步用户语言并向子页面提供用户时区。 */
export function UserPreferencesProvider({
  user,
  children,
}: {
  user: User
  children: ReactNode
}) {
  useEffect(() => {
    void i18n.changeLanguage(user.locale)
  }, [user.locale])

  return (
    <UserPreferencesContext.Provider
      value={{ locale: user.locale, timeZone: user.timeZone }}
    >
      {children}
    </UserPreferencesContext.Provider>
  )
}

/** 返回当前用户的语言和时区设置。 */
export function useUserPreferences() {
  return useContext(UserPreferencesContext)
}
