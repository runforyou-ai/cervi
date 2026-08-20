/** 企业初始化页。 */
import { SetupForm } from "@/features/installation/setup-form"

/** 展示企业初始化表单。 */
export function SetupPage() {
  return (
    <main className="flex min-h-svh w-full items-center justify-center p-6 md:p-10">
      <div className="w-full max-w-sm">
        <div className="mb-6 text-center">
          <p className="text-lg font-semibold tracking-tight">Cervi</p>
        </div>
        <SetupForm />
      </div>
    </main>
  )
}
