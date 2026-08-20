/** 企业服务器连接页。 */
import { ServerConnectionForm } from "@/features/server-connection/server-connection-form"

/** 展示企业服务器地址表单。 */
export function ServerConnectionPage() {
  return (
    <main className="flex min-h-svh w-full items-center justify-center p-6 md:p-10">
      <div className="w-full max-w-sm">
        <div className="mb-6 text-center">
          <p className="text-lg font-semibold tracking-tight">Cervi</p>
        </div>
        <ServerConnectionForm />
      </div>
    </main>
  )
}
