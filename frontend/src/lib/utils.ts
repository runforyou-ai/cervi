/** 合并 className。 */
import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

/** 合并 Tailwind class。 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
