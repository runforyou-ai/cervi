/** Vite 客户端类型声明。 */
/// <reference types="vite/client" />

/** 构建配置中的应用版本。 */
declare const __APP_VERSION__: string
/** 构建品牌的产品名称和网站嵌入脚本对象名。 */
declare const __BUILD_BRAND__: { names: Record<string, string>; sdkName: string }
