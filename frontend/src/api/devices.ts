/** 本机设备调用。 */
import { Events } from "@wailsio/runtime"

import {
  CurrentDevice,
  ListDevices,
  RevokeDevice,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import {
  DevicePlatform,
  type Device,
  type DeviceList,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import type { NonNullArrays } from "@/api/normalize"

// 与 internal/appservice/types_device.go 中的 LocalDeviceChangedEventName 保持一致。
const localDeviceChangedEventName = "cervi:local-device:changed"

export type DevicePlatformId = Exclude<DevicePlatform, DevicePlatform.$zero>

export type DeviceData = Omit<NonNullArrays<Device>, "platform"> & {
  platform: DevicePlatformId
}

export type DeviceListData = Omit<NonNullArrays<DeviceList>, "devices"> & {
  devices: DeviceData[]
}

const listDevicesBound = bind(ListDevices)

/** 读取当前用户已注册的设备列表。 */
export function listDevices() {
  return listDevicesBound() as Promise<DeviceListData>
}

/** 撤销当前用户的设备。 */
export const revokeDevice = bind(RevokeDevice)

/** 读取本机在当前企业服务器上的设备注册状态与 Agent 运行环境。 */
export const currentDevice = bind(CurrentDevice)

/** 订阅原生端本机设备状态变化，返回取消订阅函数。 */
export function onLocalDeviceChanged(listener: () => void) {
  return Events.On(localDeviceChangedEventName, () => listener())
}
