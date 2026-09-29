/** 连接链接携带的部署地址的交接：连接页登记接收处理，未登记（连接页尚未挂载）时暂存最近一次的地址，登记后立即交给它。 */

type ServerLinkReceiver = (serverURL: string) => void

let activeReceiver: ServerLinkReceiver | null = null
let heldServerURL: string | null = null

/** 把连接链接携带的部署地址交给连接页，连接页未挂载时暂存。 */
export function offerServerLink(serverURL: string) {
  if (activeReceiver) {
    activeReceiver(serverURL)
    return
  }
  heldServerURL = serverURL
}

/** 登记接收处理并交出暂存的地址，返回取消登记函数；只取消仍是当前处理的登记。 */
export function registerServerLinkReceiver(receiver: ServerLinkReceiver) {
  activeReceiver = receiver
  if (heldServerURL !== null) {
    const serverURL = heldServerURL
    heldServerURL = null
    receiver(serverURL)
  }
  return () => {
    if (activeReceiver === receiver) activeReceiver = null
  }
}
