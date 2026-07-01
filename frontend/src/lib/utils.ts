import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

// cn merges conditional class lists (clsx) and de-dupes conflicting Tailwind
// utilities (tailwind-merge) — the standard shadcn-vue class helper.
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
