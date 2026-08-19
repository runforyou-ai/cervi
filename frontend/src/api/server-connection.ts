import {
  ConnectServer,
  ServerURL,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import { call, clearSession } from "@/api/client"

export function getServerURL() {
  return call((meta) => ServerURL(meta))
}

export async function connectServer(serverUrl: string) {
  await call((meta) => ConnectServer(meta, serverUrl))
  clearSession()
}
