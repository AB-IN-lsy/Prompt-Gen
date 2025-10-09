/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:43:52
 * @FilePath: \electron-go-app\frontend\src\lib\utils.ts
 * @LastEditTime: 2025-10-09 22:43:56
 */
import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
