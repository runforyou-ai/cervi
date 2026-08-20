/** 对象存储提供商、区域和帮助链接。 */
import { StorageProvider, type StorageProviderId } from "@/api"

type StorageProviderNameKey = `storage.providers.${StorageProviderId}`

type StorageRegionNameKey =
  | "storage.regions.genericUsEast1"
  | "storage.regions.awsUsEast1"
  | "storage.regions.awsUsWest2"
  | "storage.regions.awsEuWest1"
  | "storage.regions.awsApSoutheast1"
  | "storage.regions.r2Auto"
  | "storage.regions.aliyunHangzhou"
  | "storage.regions.aliyunShanghai"
  | "storage.regions.aliyunShenzhen"
  | "storage.regions.aliyunSingapore"
  | "storage.regions.tencentBeijing"
  | "storage.regions.tencentShanghai"
  | "storage.regions.tencentGuangzhou"
  | "storage.regions.tencentSingapore"
  | "storage.regions.baiduBeijing"
  | "storage.regions.baiduGuangzhou"
  | "storage.regions.baiduHongKong"
  | "storage.regions.qiniuEast"
  | "storage.regions.qiniuNorth"
  | "storage.regions.qiniuSouth"
  | "storage.regions.huaweiBeijingFour"
  | "storage.regions.huaweiShanghaiTwo"
  | "storage.regions.ucloudBeijing"
  | "storage.regions.ucloudShanghai"
  | "storage.regions.minioDefault"
  | "storage.regions.rustfsLocal"

type StorageProviderRegion = {
  id: string
  nameKey: StorageRegionNameKey
  endpoint: string
}

type StorageProviderConfig = {
  id: StorageProviderId
  nameKey: StorageProviderNameKey
  helpUrl: string
  forcePathStyle: boolean
  regions: [StorageProviderRegion, ...StorageProviderRegion[]]
}

const storageProvidersById = {
  [StorageProvider.StorageProviderGeneric]: {
    id: StorageProvider.StorageProviderGeneric,
    nameKey: "storage.providers.generic",
    helpUrl: "https://docs.aws.amazon.com/AmazonS3/latest/API/Welcome.html",
    forcePathStyle: false,
    regions: [
      {
        id: "us-east-1",
        nameKey: "storage.regions.genericUsEast1",
        endpoint: "https://s3.us-east-1.amazonaws.com",
      },
    ],
  },
  [StorageProvider.StorageProviderAWS]: {
    id: StorageProvider.StorageProviderAWS,
    nameKey: "storage.providers.aws",
    helpUrl: "https://docs.aws.amazon.com/general/latest/gr/s3.html",
    forcePathStyle: false,
    regions: [
      {
        id: "us-east-1",
        nameKey: "storage.regions.awsUsEast1",
        endpoint: "https://s3.us-east-1.amazonaws.com",
      },
      {
        id: "us-west-2",
        nameKey: "storage.regions.awsUsWest2",
        endpoint: "https://s3.us-west-2.amazonaws.com",
      },
      {
        id: "eu-west-1",
        nameKey: "storage.regions.awsEuWest1",
        endpoint: "https://s3.eu-west-1.amazonaws.com",
      },
      {
        id: "ap-southeast-1",
        nameKey: "storage.regions.awsApSoutheast1",
        endpoint: "https://s3.ap-southeast-1.amazonaws.com",
      },
    ],
  },
  [StorageProvider.StorageProviderR2]: {
    id: StorageProvider.StorageProviderR2,
    nameKey: "storage.providers.r2",
    helpUrl: "https://developers.cloudflare.com/r2/api/s3/api/",
    forcePathStyle: false,
    regions: [
      {
        id: "auto",
        nameKey: "storage.regions.r2Auto",
        endpoint: "https://ACCOUNT_ID.r2.cloudflarestorage.com",
      },
    ],
  },
  [StorageProvider.StorageProviderAliyun]: {
    id: StorageProvider.StorageProviderAliyun,
    nameKey: "storage.providers.aliyun",
    helpUrl: "https://help.aliyun.com/zh/oss/user-guide/regions-and-endpoints",
    forcePathStyle: false,
    regions: [
      {
        id: "cn-hangzhou",
        nameKey: "storage.regions.aliyunHangzhou",
        endpoint: "https://oss-cn-hangzhou.aliyuncs.com",
      },
      {
        id: "cn-shanghai",
        nameKey: "storage.regions.aliyunShanghai",
        endpoint: "https://oss-cn-shanghai.aliyuncs.com",
      },
      {
        id: "cn-shenzhen",
        nameKey: "storage.regions.aliyunShenzhen",
        endpoint: "https://oss-cn-shenzhen.aliyuncs.com",
      },
      {
        id: "ap-southeast-1",
        nameKey: "storage.regions.aliyunSingapore",
        endpoint: "https://oss-ap-southeast-1.aliyuncs.com",
      },
    ],
  },
  [StorageProvider.StorageProviderTencent]: {
    id: StorageProvider.StorageProviderTencent,
    nameKey: "storage.providers.tencent",
    helpUrl: "https://cloud.tencent.com/document/product/436/6224",
    forcePathStyle: false,
    regions: [
      {
        id: "ap-beijing",
        nameKey: "storage.regions.tencentBeijing",
        endpoint: "https://cos.ap-beijing.myqcloud.com",
      },
      {
        id: "ap-shanghai",
        nameKey: "storage.regions.tencentShanghai",
        endpoint: "https://cos.ap-shanghai.myqcloud.com",
      },
      {
        id: "ap-guangzhou",
        nameKey: "storage.regions.tencentGuangzhou",
        endpoint: "https://cos.ap-guangzhou.myqcloud.com",
      },
      {
        id: "ap-singapore",
        nameKey: "storage.regions.tencentSingapore",
        endpoint: "https://cos.ap-singapore.myqcloud.com",
      },
    ],
  },
  [StorageProvider.StorageProviderBaidu]: {
    id: StorageProvider.StorageProviderBaidu,
    nameKey: "storage.providers.baidu",
    helpUrl: "https://cloud.baidu.com/doc/BOS/s/xjwvyq9l4",
    forcePathStyle: false,
    regions: [
      {
        id: "s3.bj",
        nameKey: "storage.regions.baiduBeijing",
        endpoint: "https://s3.bj.bcebos.com",
      },
      {
        id: "s3.gz",
        nameKey: "storage.regions.baiduGuangzhou",
        endpoint: "https://s3.gz.bcebos.com",
      },
      {
        id: "s3.hkg",
        nameKey: "storage.regions.baiduHongKong",
        endpoint: "https://s3.hkg.bcebos.com",
      },
    ],
  },
  [StorageProvider.StorageProviderQiniu]: {
    id: StorageProvider.StorageProviderQiniu,
    nameKey: "storage.providers.qiniu",
    helpUrl: "https://developer.qiniu.com/kodo/4088/s3-access-domainname",
    forcePathStyle: false,
    regions: [
      {
        id: "cn-east-1",
        nameKey: "storage.regions.qiniuEast",
        endpoint: "https://s3.cn-east-1.qiniucs.com",
      },
      {
        id: "cn-north-1",
        nameKey: "storage.regions.qiniuNorth",
        endpoint: "https://s3.cn-north-1.qiniucs.com",
      },
      {
        id: "cn-south-1",
        nameKey: "storage.regions.qiniuSouth",
        endpoint: "https://s3.cn-south-1.qiniucs.com",
      },
    ],
  },
  [StorageProvider.StorageProviderHuawei]: {
    id: StorageProvider.StorageProviderHuawei,
    nameKey: "storage.providers.huawei",
    helpUrl: "https://console.huaweicloud.com/apiexplorer/#/endpoint/OBS",
    forcePathStyle: false,
    regions: [
      {
        id: "cn-north-4",
        nameKey: "storage.regions.huaweiBeijingFour",
        endpoint: "https://obs.cn-north-4.myhuaweicloud.com",
      },
      {
        id: "cn-east-2",
        nameKey: "storage.regions.huaweiShanghaiTwo",
        endpoint: "https://obs.cn-east-2.myhuaweicloud.com",
      },
    ],
  },
  [StorageProvider.StorageProviderUCloud]: {
    id: StorageProvider.StorageProviderUCloud,
    nameKey: "storage.providers.ucloud",
    helpUrl: "https://docs.ucloud.cn/ufile/s3/s3_introduction",
    forcePathStyle: false,
    regions: [
      {
        id: "cn-bj",
        nameKey: "storage.regions.ucloudBeijing",
        endpoint: "https://s3-cn-bj.ufileos.com",
      },
      {
        id: "cn-sh",
        nameKey: "storage.regions.ucloudShanghai",
        endpoint: "https://s3-cn-sh.ufileos.com",
      },
    ],
  },
  [StorageProvider.StorageProviderMinIO]: {
    id: StorageProvider.StorageProviderMinIO,
    nameKey: "storage.providers.minio",
    helpUrl:
      "https://docs.min.io/aistor/reference/aistor-server/http-endpoints/",
    forcePathStyle: true,
    regions: [
      {
        id: "us-east-1",
        nameKey: "storage.regions.minioDefault",
        endpoint: "http://127.0.0.1:9000",
      },
    ],
  },
  [StorageProvider.StorageProviderRustFS]: {
    id: StorageProvider.StorageProviderRustFS,
    nameKey: "storage.providers.rustfs",
    helpUrl: "https://docs.rustfs.com.cn/developer/sdk/javascript.html",
    forcePathStyle: true,
    regions: [
      {
        id: "us-east-1",
        nameKey: "storage.regions.rustfsLocal",
        endpoint: "http://127.0.0.1:9000",
      },
    ],
  },
} satisfies Record<StorageProviderId, StorageProviderConfig>

export const storageProviders = Object.values(storageProvidersById)

/** 读取指定对象存储提供商配置。 */
export function getStorageProvider(id: StorageProvider) {
  if (!isStorageProviderId(id)) {
    throw new Error(`Unsupported storage provider: ${id}`)
  }
  return storageProvidersById[id]
}

/** 读取提供商下的指定区域。 */
export function getStorageRegion(provider: StorageProviderConfig, id: string) {
  return provider.regions.find((region) => region.id === id)
}

/** 判断取值是否为支持的对象存储提供商。 */
function isStorageProviderId(id: StorageProvider): id is StorageProviderId {
  return id in storageProvidersById
}
