/** 本机设备调用。 */
import {
  AddLocalWorkspace,
  CurrentDevice,
  ListDeviceWorkspaces,
  ListDevices,
  RevokeDevice,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import {
  DevicePlatform,
  type Device,
  type DeviceList,
  type DeviceWorkspaceList,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import type { NonNullArrays } from "@/api/normalize"

export type DevicePlatformId = Exclude<DevicePlatform, DevicePlatform.$zero>

export type DeviceData = Omit<NonNullArrays<Device>, "platform"> & {
  platform: DevicePlatformId
}

export type DeviceListData = Omit<NonNullArrays<DeviceList>, "devices"> & {
  devices: DeviceData[]
}

export type DeviceWorkspaceListData = NonNullArrays<DeviceWorkspaceList>

const listDevicesBound = bind(ListDevices)
const listDeviceWorkspacesBound = bind(ListDeviceWorkspaces)

/** 读取当前用户已注册的设备列表。 */
export function listDevices() {
  return listDevicesBound() as Promise<DeviceListData>
}

/** 撤销当前用户的设备。 */
export const revokeDevice = bind(RevokeDevice)

/** 读取本机在当前企业服务器上的设备注册状态。 */
export const currentDevice = bind(CurrentDevice)

/** 读取当前用户指定设备上的工作区。 */
export function listDeviceWorkspaces(deviceID: string) {
  return listDeviceWorkspacesBound(deviceID) as Promise<DeviceWorkspaceListData>
}

/** 选择本机目录并注册为本设备的工作区，用户取消选择时返回空工作区编号。 */
export const addLocalWorkspace = bind(AddLocalWorkspace)
