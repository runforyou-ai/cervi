import { LoginForm } from "@/components/login-form"

export function LoginPage({ onLogin }: { onLogin: () => void }) {
  return (
    <main className="flex min-h-svh w-full items-center justify-center p-6 md:p-10">
      <div className="w-full max-w-sm">
        <LoginForm onLogin={onLogin} />
      </div>
    </main>
  )
}
