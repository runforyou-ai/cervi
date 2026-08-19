import { InstallWorkspace } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type { InstallWorkspaceInput } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { acceptSession, call } from "@/api/client"

export async function install(input: InstallWorkspaceInput) {
  return acceptSession(await call((meta) => InstallWorkspace(meta, input)))
}
