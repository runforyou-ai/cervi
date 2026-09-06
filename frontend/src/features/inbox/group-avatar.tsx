/** 群聊图片展示和跨平台图片选择交互。 */
import {
  useRef,
  useState,
  type ChangeEvent,
} from "react"
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { selectImage } from "@/api"
import { ProfileAvatar } from "@/components/profile-avatar"
import { cn } from "@/lib/utils"
import { resolveAppPlatform } from "@/platform/app-platform"

const groupImageContentTypes = new Set([
  "image/jpeg",
  "image/png",
  "image/webp",
])
const maxGroupImageByteSize = 5 * 1024 * 1024
const groupImageFileAccept =
  ".jpg,.jpeg,.png,.webp,image/jpeg,image/png,image/webp"

/** 展示自定义群图片，并在缺失或加载失败时回退到默认群组图标。 */
export function GroupAvatar({
  imageURL,
  className,
}: {
  imageURL?: string
  className?: string
}) {
  return (
    <ProfileAvatar
      imageURL={imageURL}
      fallback="group"
      className={cn("size-full", className)}
    />
  )
}

/** 将原生文件选择器返回的 Base64 内容转换为浏览器文件。 */
function nativeImageToFile(selected: {
  name: string
  contentType: string
  dataBase64: string
}) {
  const binary = window.atob(selected.dataBase64)
  const content = new Uint8Array(binary.length)
  for (let index = 0; index < binary.length; index += 1) {
    content[index] = binary.charCodeAt(index)
  }
  return new File([content], selected.name, { type: selected.contentType })
}

/** 提供群图片预览按钮和 Web、桌面端文件选择入口。 */
export function GroupImagePicker({
  imageURL,
  disabled = false,
  loading = false,
  className,
  onSelect,
}: {
  imageURL?: string
  disabled?: boolean
  loading?: boolean
  className?: string
  onSelect: (file: File) => void
}) {
  const { t } = useTranslation("inbox")
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [selecting, setSelecting] = useState(false)

  /** 校验图片并把有效文件交给业务表单。 */
  function acceptImage(file: File) {
    if (!groupImageContentTypes.has(file.type)) {
      toast.error(t("groupImageTypeError"))
      return
    }
    if (file.size <= 0 || file.size > maxGroupImageByteSize) {
      toast.error(t("groupImageSizeError"))
      return
    }
    onSelect(file)
  }

  /** 处理 Web 文件选择器返回的图片。 */
  function selectBrowserImage(event: ChangeEvent<HTMLInputElement>) {
    const selected = event.target.files?.[0]
    event.target.value = ""
    if (selected) acceptImage(selected)
  }

  /** 按当前平台打开群图片选择器。 */
  async function chooseImage() {
    if (resolveAppPlatform() !== "desktop") {
      fileInputRef.current?.click()
      return
    }
    setSelecting(true)
    try {
      const selected = await selectImage()
      if (selected.name) acceptImage(nativeImageToFile(selected))
    } catch (error) {
      console.warn("选择群聊图片失败", error)
      toast.error(t("groupImageChooseError"))
    } finally {
      setSelecting(false)
    }
  }

  const busy = selecting || loading

  return (
    <div>
      <input
        ref={fileInputRef}
        className="sr-only"
        type="file"
        accept={groupImageFileAccept}
        aria-label={t("groupImageChoose")}
        onChange={selectBrowserImage}
      />
      <button
        type="button"
        className={cn(
          "group relative size-20 overflow-hidden rounded-2xl outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60",
          className,
        )}
        aria-label={t("groupImageChoose")}
        disabled={disabled || busy}
        onClick={() => void chooseImage()}
      >
        <GroupAvatar imageURL={imageURL} />
        {!disabled || busy ? (
          <span
            className={cn(
              "absolute inset-0 flex items-center justify-center bg-black/55 px-2 text-center text-xs font-medium text-white opacity-0 transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100",
              busy && "opacity-100",
            )}
          >
            {busy ? (
              <LoaderCircleIcon className="animate-spin" />
            ) : imageURL ? (
              t("groupImageChange")
            ) : (
              t("groupImageChoose")
            )}
          </span>
        ) : null}
      </button>
    </div>
  )
}
